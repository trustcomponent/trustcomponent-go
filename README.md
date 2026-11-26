# TrustComponent SDK for Go

The official SDK for the TrustComponent API: TrustCaptcha, TrustAnalytics, TrustMonitoring, Trust VAT Checker and the
platform API, in one package. Developed and hosted in the EU.

Requires Go 1.22 or newer, no dependencies outside the standard library.

## Install

```sh
go get github.com/trustcomponent/trustcomponent-go
```

## Check a TrustCaptcha result

The widget writes a verification token into your form. One call turns it into the result:

```go
import trustcomponent "github.com/trustcomponent/trustcomponent-go"

tc := trustcomponent.New("ak_…")
result, err := tc.Captcha.GetVerificationResult(ctx, r.FormValue("tc-verification-token"))
if err == nil && result.GetVerificationPassed() { /* … */ }
```

Create the API key (`ak_…`) in the dashboard under the project's API keys. The key decides which modules the client can use.

## Every module

The client is `module → resource → operation`, with the same names in every language:

```go
tc.Analytics.Websites.GetWebsites(ctx).Execute()
tc.TaxValidation.Validations.CreateValidation(ctx).TaxValidationValidationCreateParams(*api.NewTaxValidationValidationCreateParams("DE", "DE811569869")).Execute()
tc.Monitoring.Heartbeats.PingHeartbeat(ctx, serviceID, monitorID).Execute()
tc.Platform.Events.GetEvents(ctx).Execute()
```

The full reference of every operation is at https://docs.trustcomponent.com.

## Configuration

```go
tc := trustcomponent.New("ak_…", trustcomponent.WithTimeout(5*time.Second), trustcomponent.WithMaxRetries(1), trustcomponent.WithIntegration("my-plugin/1.0"))
```

Options: `WithHost`, `WithTimeout`, `WithProxy`, `WithMaxRetries`, `WithDeprecationWarnings`, `WithIntegration`, `WithLogger`, `WithTransport`.

The SDK retries only what the API reserves for retries: `429` with `Retry-After`, `502`, `503`, `504` and a lost
connection (2 retries by default). When the API marks an operation as deprecated, it logs one warning per operation.
Every request names the SDK in `User-Agent` and `TrustComponent-Sdk`, together with the integration you set.

## Errors

Every error is a `*trustcomponent.Error` carrying the HTTP status and the stable `code` of the API:
`IsInvalidRequest`, `IsAuthentication`, `IsPermission`, `IsPlanLimit`, `IsNotFound`, `IsConflict`, `IsGone`, `IsRateLimit`, `IsServer`, `IsOperationRetired`; no response at all is a `*trustcomponent.ConnectionError`, an invalid widget token `trustcomponent.ErrVerificationTokenInvalid`.

```go
var tcErr *trustcomponent.Error
if errors.As(err, &tcErr) && tcErr.IsNotFound() {
    // tcErr.Status, tcErr.Code
}
```

`OperationRetired` means the operation was switched off after its sunset: update the SDK.

## Support

The GitHub repository of this SDK is a read-only mirror, generated from the TrustComponent API, so pull requests and
issues are not processed there. For questions and bug reports, write to mail@trustcomponent.com or contact support
through https://www.trustcomponent.com.

## License

MIT
