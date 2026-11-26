package trustcomponent

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

var (
	retryableStatus     = map[int]bool{502: true, 503: true, 504: true}
	uuidPattern         = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f-]{27}`)
	warnedOperations    sync.Map
	maxRetryAfterSecond = 60
)

// transport is the single HTTP path of the SDK: it retries what the API reserves for retries (429 with Retry-After,
// 502/503/504, no response at all), warns once per deprecated operation and turns every other non-2xx answer into an
// *Error before the generated code sees it.
type transport struct {
	next                http.RoundTripper
	apiKey, userAgent   string
	sdkHeader           string
	maxRetries          int
	deprecationWarnings bool
	logger              func(string)
}

func (t *transport) RoundTrip(original *http.Request) (*http.Response, error) {
	var body []byte
	if original.Body != nil {
		var err error
		if body, err = io.ReadAll(original.Body); err != nil {
			return nil, err
		}
		original.Body.Close()
	}
	for attempt := 0; ; attempt++ {
		request := original.Clone(original.Context())
		if body != nil {
			request.Body = io.NopCloser(bytes.NewReader(body))
		}
		request.Header.Set("Authorization", "Bearer "+t.apiKey)
		request.Header.Set("User-Agent", t.userAgent)
		request.Header.Set("TrustComponent-Sdk", t.sdkHeader)

		response, err := t.next.RoundTrip(request)
		if err != nil {
			if attempt < t.maxRetries && original.Context().Err() == nil {
				if sleep(original, backoff(attempt)) {
					continue
				}
			}
			return nil, &ConnectionError{Cause: err}
		}
		t.warnIfDeprecated(request, response)
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, nil
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		retryAfter := retryAfterSeconds(response.Header.Get("Retry-After"))
		retryable := retryableStatus[response.StatusCode] || (response.StatusCode == 429 && retryAfter != nil)
		if retryable && attempt < t.maxRetries && (retryAfter == nil || *retryAfter <= maxRetryAfterSecond) {
			wait := backoff(attempt)
			if retryAfter != nil {
				wait = time.Duration(*retryAfter) * time.Second
			}
			if sleep(original, wait) {
				continue
			}
		}
		return nil, errorOf(response.StatusCode, data, retryAfter)
	}
}

func sleep(request *http.Request, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-request.Context().Done():
		return false
	}
}

func backoff(attempt int) time.Duration {
	ms := 500 << attempt
	if ms > 8000 {
		ms = 8000
	}
	return time.Duration(ms+rand.Intn(250)) * time.Millisecond
}

func retryAfterSeconds(value string) *int {
	if value == "" {
		return nil
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			seconds = 0
		}
		return &seconds
	}
	if date, err := http.ParseTime(value); err == nil {
		seconds := int(time.Until(date).Seconds())
		if seconds < 0 {
			seconds = 0
		}
		return &seconds
	}
	return nil
}

func errorOf(status int, data []byte, retryAfter *int) *Error {
	var problem struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(data, &problem)
	return &Error{Status: status, Code: problem.Code, Detail: problem.Detail, RetryAfterSeconds: retryAfter}
}

func (t *transport) warnIfDeprecated(request *http.Request, response *http.Response) {
	if !t.deprecationWarnings || response.Header.Get("Deprecation") == "" {
		return
	}
	operation := request.Method + " " + uuidPattern.ReplaceAllString(request.URL.Path, "{id}")
	if _, seen := warnedOperations.LoadOrStore(operation, true); seen {
		return
	}
	message := "TrustComponent: " + operation + " is deprecated"
	if sunset := response.Header.Get("Sunset"); sunset != "" {
		message += " and will be switched off on " + sunset
	}
	t.logger(message + ". Update the TrustComponent SDK.")
}
