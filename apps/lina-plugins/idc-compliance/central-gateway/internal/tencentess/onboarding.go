// This file owns Tencent's official one-use enterprise console onboarding link.
package tencentess

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"
)

const maximumLegalNameCharacters = 200

var onboardingInstallationPattern = regexp.MustCompile(`^linst_[0-9a-f]{64}$`)

// OnboardingResult contains only the official short-lived URL and safe status flags.
// The provider subject remains private and can only be consumed by the local gateway.
type OnboardingResult struct {
	ConsoleURL       string
	Activated        bool
	OperatorVerified bool
	RequestID        string
	authorization    []byte
}

// ConsumeAuthorization transfers the one-time provider subject to the local
// supplier-identity service and clears this result's reference.
func (r *OnboardingResult) ConsumeAuthorization() []byte {
	if r == nil {
		return nil
	}
	value := r.authorization
	r.authorization = nil
	return value
}

// CreateOrganizationConsoleURL generates Tencent's official one-use PC link.
// LinaPro submits only its existing legal name; Tencent collects all sensitive
// certification data on its own page.
func (c *Client) CreateOrganizationConsoleURL(
	ctx context.Context,
	installationID string,
	legalName string,
) (OnboardingResult, error) {
	legalName = strings.TrimSpace(legalName)
	if !c.Configured() || c.onboarding == nil || ctx == nil || ctx.Err() != nil ||
		!onboardingInstallationPattern.MatchString(installationID) || legalName == "" ||
		!utf8.ValidString(legalName) || utf8.RuneCountInString(legalName) > maximumLegalNameCharacters ||
		strings.ContainsAny(legalName, "\r\n\t\x00") {
		return OnboardingResult{}, &Error{Code: ErrorInvalidInput}
	}
	organizationOpenID := c.derivedOnboardingOpenID("organization", installationID)
	operatorOpenID := c.derivedOnboardingOpenID("operator", installationID)
	endpoint := "PC"
	request := essbasic.NewCreateConsoleLoginUrlRequest()
	request.Agent = &essbasic.Agent{
		AppId:                   &c.appID,
		ProxyOrganizationOpenId: &organizationOpenID,
		ProxyOperator:           &essbasic.UserInfo{OpenId: &operatorOpenID},
	}
	request.ProxyOrganizationName = &legalName
	request.Endpoint = &endpoint
	response, err := c.onboarding.CreateConsoleLoginUrlWithContext(ctx, request)
	if err != nil {
		return OnboardingResult{}, safeError(err, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		response.Response.ConsoleUrl == nil || response.Response.IsActivated == nil ||
		response.Response.ProxyOperatorIsVerified == nil ||
		!validHTTPSURL(*response.Response.ConsoleUrl) ||
		!validOpaque(*response.Response.RequestId, 1, 128) {
		return OnboardingResult{}, &Error{Code: ErrorMalformedResponse}
	}
	result := OnboardingResult{
		ConsoleURL:       *response.Response.ConsoleUrl,
		Activated:        *response.Response.IsActivated,
		OperatorVerified: *response.Response.ProxyOperatorIsVerified,
		RequestID:        *response.Response.RequestId,
	}
	if result.Activated && result.OperatorVerified {
		subject, marshalErr := json.Marshal(canonicalSubject{
			ProxyOrganizationOpenID: organizationOpenID,
			ProxyOperatorOpenID:     operatorOpenID,
		})
		if marshalErr != nil || len(subject) == 0 || len(subject) > maximumSubjectBytes {
			clearBytes(subject)
			return OnboardingResult{}, &Error{Code: ErrorMalformedResponse, cause: marshalErr}
		}
		result.authorization = subject
	}
	return result, nil
}

// SubjectAvailable checks only the local encrypted binding and never calls Tencent.
func (c *Client) SubjectAvailable(ctx context.Context) bool {
	return c != nil && c.withAgent(ctx, func(*essbasic.Agent) error { return nil }) == nil
}

func (c *Client) derivedOnboardingOpenID(kind string, installationID string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"idc/tencent-essbasic/onboarding/v1", kind, c.appID, installationID,
	}, "\x00")))
	return "lpo_" + base64.RawURLEncoding.EncodeToString(digest[:])
}
