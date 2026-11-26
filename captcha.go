package trustcomponent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/trustcomponent/trustcomponent-go/api"
)

// VerificationToken is what the TrustCaptcha widget writes into the form.
type VerificationToken struct {
	VerificationID string
	ClientFailover bool
}

// GetVerificationResult returns the result of the verification behind a widget token, the one call a form handler
// needs. When the widget could not reach TrustCaptcha (clientFailover in the token), the public gateway decides whether
// the visitor may pass.
func (m *CaptchaModule) GetVerificationResult(ctx context.Context, verificationToken string) (*api.CaptchaVerificationResult, error) {
	token, err := ParseVerificationToken(verificationToken)
	if err != nil {
		return nil, err
	}
	request := m.Verifications.GetResult(ctx, token.VerificationID)
	if token.ClientFailover {
		request = request.ClientFailover(true)
	}
	result, _, err := request.Execute()
	return result, err
}

// ParseVerificationToken decodes the widget token.
func ParseVerificationToken(verificationToken string) (VerificationToken, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(verificationToken))
	if err != nil {
		return VerificationToken{}, ErrVerificationTokenInvalid
	}
	var token struct {
		VerificationID string `json:"verificationId"`
		ClientFailover bool   `json:"clientFailover"`
	}
	if json.Unmarshal(data, &token) != nil || token.VerificationID == "" {
		return VerificationToken{}, ErrVerificationTokenInvalid
	}
	return VerificationToken{VerificationID: token.VerificationID, ClientFailover: token.ClientFailover}, nil
}
