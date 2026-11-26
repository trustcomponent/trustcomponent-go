package trustcomponent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const testID = "cc2e2d5e-d1ef-4a7f-a7bd-dec5b37df47a"
const ts = "2020-01-01T13:30:05.941Z"

var result = `{"captchaId":"` + testID + `","verificationId":"` + testID + `","verificationPassed":true,"score":0.3,"decisionType":"STANDARD","decisionAction":"ALLOW",` +
	`"gatewayFailoverActive":false,"riskScoringEnabled":true,"minimalDataModeEnabled":false,"origin":"https://example.com","ipAddress":"1.2.3.4","countryCode":"DE",` +
	`"deviceFamily":"Desktop","operatingSystem":"Linux","browser":"Firefox","verificationStartedAt":"` + ts + `","verificationFinishedAt":"` + ts + `",` +
	`"resultExpiresAt":"` + ts + `","resultFirstFetchedAt":"` + ts + `","resultLastFetchedAt":"` + ts + `"}`

type answer struct {
	status  int
	headers map[string]string
	body    string
}

func ok(headers map[string]string) answer { return answer{200, headers, result} }

type fake struct {
	mu       sync.Mutex
	requests []*http.Request
	warnings []string
}

func start(t *testing.T, answers ...answer) (*Client, *fake) {
	f := &fake{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r)
		a := answers[min(len(f.requests)-1, len(answers)-1)]
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		for k, v := range a.headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(a.status)
		_, _ = w.Write([]byte(a.body))
	}))
	t.Cleanup(server.Close)
	return New("ak_test", WithHost(server.URL), WithIntegration("test-suite/1.0"), WithLogger(func(m string) { f.warnings = append(f.warnings, m) })), f
}

func token(clientFailover bool) string {
	data, _ := json.Marshal(map[string]any{"verificationId": testID, "clientFailover": clientFailover})
	return base64.StdEncoding.EncodeToString(data)
}

func apiError(t *testing.T, err error) *Error {
	t.Helper()
	var tcErr *Error
	if !errors.As(err, &tcErr) {
		t.Fatalf("expected *Error, got %T %v", err, err)
	}
	return tcErr
}

func TestSendsKeyUserAgentAndMetadataAndDecodesTheWidgetToken(t *testing.T) {
	tc, f := start(t, ok(nil))
	res, err := tc.Captcha.GetVerificationResult(context.Background(), token(true))
	if err != nil || !res.GetVerificationPassed() {
		t.Fatalf("unexpected %v %v", res, err)
	}
	r := f.requests[0]
	if r.URL.Path != "/captcha/v2/verifications/"+testID+"/results" || r.URL.RawQuery != "clientFailover=true" {
		t.Fatalf("path %s?%s", r.URL.Path, r.URL.RawQuery)
	}
	if r.Header.Get("Authorization") != "Bearer ak_test" || !strings.HasPrefix(r.Header.Get("User-Agent"), "trustcomponent-go/") {
		t.Fatalf("headers %v", r.Header)
	}
	data, _ := base64.StdEncoding.DecodeString(r.Header.Get("TrustComponent-Sdk"))
	var metadata map[string]string
	_ = json.Unmarshal(data, &metadata)
	if metadata["sdk"] != "go" || metadata["integration"] != "test-suite/1.0" {
		t.Fatalf("metadata %v", metadata)
	}
}

func TestAnInvalidWidgetTokenFailsBeforeAnyRequest(t *testing.T) {
	tc, f := start(t, ok(nil))
	if _, err := tc.Captcha.GetVerificationResult(context.Background(), "not-a-token"); !errors.Is(err, ErrVerificationTokenInvalid) {
		t.Fatalf("got %v", err)
	}
	if len(f.requests) != 0 {
		t.Fatal("request sent")
	}
}

func TestRetries503And429WithRetryAfterThenSucceeds(t *testing.T) {
	tc, f := start(t, answer{503, nil, ""}, answer{429, map[string]string{"Retry-After": "0"}, ""}, ok(nil))
	if _, _, err := tc.Captcha.Verifications.GetResult(context.Background(), testID).Execute(); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 3 {
		t.Fatalf("%d requests", len(f.requests))
	}
}

func TestA429WithoutRetryAfterIsFinalAndNotRetried(t *testing.T) {
	tc, f := start(t, answer{429, nil, ""}, ok(nil))
	_, _, err := tc.Captcha.Verifications.GetResult(context.Background(), testID).Execute()
	if e := apiError(t, err); !e.IsRateLimit() || e.RetryAfterSeconds != nil {
		t.Fatalf("got %v", e)
	}
	if len(f.requests) != 1 {
		t.Fatalf("%d requests", len(f.requests))
	}
}

func TestProblemDetailsBecomeTypedErrorsWithTheCodeOfTheApi(t *testing.T) {
	tc, _ := start(t, answer{404, nil, `{"status":404,"code":"validation_result_not_found","detail":"detail"}`})
	_, _, err := tc.TaxValidation.ValidationResults.GetValidationResult(context.Background(), testID).Execute()
	if e := apiError(t, err); !e.IsNotFound() || e.Code != "validation_result_not_found" {
		t.Fatalf("got %v", e)
	}
}

func TestAClientFailoverRejectedByTheGatewayIsAConflict(t *testing.T) {
	tc, _ := start(t, answer{412, nil, `{"reason":"CLIENT_FAILOVER_REJECTED"}`})
	_, err := tc.Captcha.GetVerificationResult(context.Background(), token(true))
	if e := apiError(t, err); !e.IsConflict() {
		t.Fatalf("got %v", e)
	}
}

func TestADeprecatedOperationWarnsOnceARetiredOneAsksForAnUpdate(t *testing.T) {
	deprecated := map[string]string{"Deprecation": "@1799971200", "Sunset": "Thu, 15 Jul 2027 00:00:00 GMT"}
	tc, f := start(t, ok(deprecated), ok(deprecated), answer{410, nil, `{"status":410,"code":"operation_retired","detail":"retired"}`})
	for i := 0; i < 2; i++ {
		if _, _, err := tc.Captcha.Verifications.GetResult(context.Background(), testID).Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.warnings) != 1 || !strings.Contains(f.warnings[0], "15 Jul 2027") {
		t.Fatalf("warnings %v", f.warnings)
	}
	_, _, err := tc.Platform.Events.GetEvents(context.Background()).Execute()
	if e := apiError(t, err); !e.IsOperationRetired() {
		t.Fatalf("got %v", e)
	}
}
