// Package trustcomponent is the official TrustComponent SDK for Go: TrustCaptcha, TrustAnalytics, TrustMonitoring,
// Trust VAT Checker and the platform API.
//
//	tc := trustcomponent.New("ak_…")
//	result, err := tc.Captcha.GetVerificationResult(ctx, token)
package trustcomponent

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/trustcomponent/trustcomponent-go/api"
)

// Client is the entry point of the SDK. One client per application: it holds the connection pool.
type Client struct {
	Captcha       *CaptchaModule
	Analytics     *AnalyticsModule
	Monitoring    *MonitoringModule
	TaxValidation *TaxValidationModule
	Platform      *PlatformModule
}

type settings struct {
	host                string
	timeout             time.Duration
	proxy               *url.URL
	maxRetries          int
	deprecationWarnings bool
	integration         string
	logger              func(string)
	transport           http.RoundTripper
}

// Option changes one setting of New.
type Option func(*settings)

// WithHost points the client at another API host, e.g. a local stack for tests.
func WithHost(host string) Option { return func(s *settings) { s.host = host } }

// WithTimeout limits one attempt: connecting, and waiting for the answer to start.
func WithTimeout(timeout time.Duration) Option { return func(s *settings) { s.timeout = timeout } }

// WithProxy sends every request through an HTTP proxy.
func WithProxy(proxy *url.URL) Option { return func(s *settings) { s.proxy = proxy } }

// WithMaxRetries sets the retries after 429 with Retry-After, 502/503/504 and lost connections; 0 switches them off.
func WithMaxRetries(maxRetries int) Option {
	return func(s *settings) {
		if maxRetries < 0 {
			maxRetries = 0
		}
		s.maxRetries = maxRetries
	}
}

// WithDeprecationWarnings switches the one-time warning per deprecated operation on or off.
func WithDeprecationWarnings(enabled bool) Option {
	return func(s *settings) { s.deprecationWarnings = enabled }
}

// WithIntegration names the plugin or product built on the SDK, e.g. "hugo-module/1.2.0".
func WithIntegration(integration string) Option {
	return func(s *settings) { s.integration = integration }
}

// WithLogger receives the deprecation warnings.
func WithLogger(logger func(string)) Option { return func(s *settings) { s.logger = logger } }

// WithTransport replaces the underlying http.RoundTripper, e.g. for tests or an own connection pool.
func WithTransport(transport http.RoundTripper) Option {
	return func(s *settings) { s.transport = transport }
}

// New creates a client for an API key (ak_…).
func New(apiKey string, options ...Option) *Client {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		panic("trustcomponent: apiKey must not be empty")
	}
	s := settings{host: "https://api.trustcomponent.com", timeout: 10 * time.Second, maxRetries: 2, deprecationWarnings: true, logger: func(m string) { log.Print(m) }}
	for _, option := range options {
		option(&s)
	}
	next := s.transport
	if next == nil {
		base := http.DefaultTransport.(*http.Transport).Clone()
		base.ResponseHeaderTimeout = s.timeout
		base.DialContext = (&net.Dialer{Timeout: s.timeout, KeepAlive: 30 * time.Second}).DialContext
		if s.proxy != nil {
			base.Proxy = http.ProxyURL(s.proxy)
		}
		next = base
	}
	userAgent := "trustcomponent-go/" + Version
	configuration := api.NewConfiguration()
	configuration.Servers = api.ServerConfigurations{{URL: strings.TrimRight(s.host, "/")}}
	configuration.UserAgent = userAgent
	configuration.HTTPClient = &http.Client{
		Transport: &transport{next: next, apiKey: apiKey, userAgent: userAgent, sdkHeader: sdkHeader(s.integration),
			maxRetries: s.maxRetries, deprecationWarnings: s.deprecationWarnings, logger: s.logger},
	}
	client := api.NewAPIClient(configuration)
	return &Client{
		Captcha:       newCaptchaModule(client),
		Analytics:     newAnalyticsModule(client),
		Monitoring:    newMonitoringModule(client),
		TaxValidation: newTaxValidationModule(client),
		Platform:      newPlatformModule(client),
	}
}

func sdkHeader(integration string) string {
	metadata := map[string]string{"sdk": "go", "version": Version, "api": apiVersion, "runtime": runtime.Version(), "os": runtime.GOOS}
	if integration != "" {
		metadata["integration"] = integration
	}
	data, _ := json.Marshal(metadata)
	return base64.StdEncoding.EncodeToString(data)
}
