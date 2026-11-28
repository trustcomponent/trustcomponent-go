package trustcomponent

import "fmt"

// Error is every error the SDK returns for an answer of the API. Status is the HTTP status, Code the stable reason
// of the API. Use errors.As(err, &tcErr) and the Is… helpers.
type Error struct {
	Status            int
	Code              string
	Detail            string
	RetryAfterSeconds *int
}

func (e *Error) Error() string {
	message := fmt.Sprintf("TrustComponent API answered %d", e.Status)
	if e.Code != "" {
		message += " (" + e.Code + ")"
	}
	if e.Detail != "" {
		message += ": " + e.Detail
	}
	if e.IsOperationRetired() {
		message += ". Update the TrustComponent SDK."
	}
	return message
}

func (e *Error) IsInvalidRequest() bool { return e.Status == 400 || e.Status == 422 }
func (e *Error) IsAuthentication() bool { return e.Status == 401 }
func (e *Error) IsPlanLimit() bool      { return e.Status == 402 }
func (e *Error) IsPermission() bool     { return e.Status == 403 }
func (e *Error) IsNotFound() bool       { return e.Status == 404 }
func (e *Error) IsConflict() bool       { return e.Status == 409 || e.Status == 412 }
func (e *Error) IsLocked() bool         { return e.Status == 423 }
func (e *Error) IsGone() bool           { return e.Status == 410 && !e.IsOperationRetired() }
func (e *Error) IsRateLimit() bool      { return e.Status == 429 }
func (e *Error) IsServer() bool         { return e.Status >= 500 }

// IsOperationRetired means the operation was switched off after its sunset; a newer SDK calls its successor.
func (e *Error) IsOperationRetired() bool { return e.Status == 410 && e.Code == "operation_retired" }

// ConnectionError means no response arrived: timeout, DNS, connection refused.
type ConnectionError struct{ Cause error }

func (e *ConnectionError) Error() string {
	return "TrustComponent API not reachable: " + e.Cause.Error()
}
func (e *ConnectionError) Unwrap() error { return e.Cause }

// ErrVerificationTokenInvalid is returned when a string is not a token of the TrustCaptcha widget.
var ErrVerificationTokenInvalid = fmt.Errorf("the verification token is not a TrustCaptcha token")
