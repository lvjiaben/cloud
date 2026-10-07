// Package onboardingcap defines the source-only boundary used by notify after
// it has atomically verified a public customer contact code.
package onboardingcap

import (
	"context"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	OwnerPluginID         = "idc-core"
	AllowedCallerPluginID = "idc-notify"
	ContractVersion       = "v1"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type CreateVerifiedIntentRequest struct {
	OperationDigest string
	RequestDigest   string
	UsernameDigest  string
	ContactDigest   string
	BrowserDigest   string
	ContactChannel  string
	CustomerType    string
	ReturnPath      string
	VerifiedAt      time.Time
}

type VerifiedIntent struct {
	IntentNo  string
	Token     string
	ExpiresAt time.Time
	Reused    bool
}

type Service interface {
	CreateVerifiedIntent(context.Context, CreateVerifiedIntentRequest) (*VerifiedIntent, error)
}

func ValidateCreateRequest(request CreateVerifiedIntentRequest) error {
	if !digestPattern.MatchString(request.OperationDigest) ||
		!digestPattern.MatchString(request.RequestDigest) ||
		!digestPattern.MatchString(request.UsernameDigest) ||
		!digestPattern.MatchString(request.ContactDigest) ||
		!digestPattern.MatchString(request.BrowserDigest) ||
		(request.ContactChannel != "email" && request.ContactChannel != "phone") ||
		request.CustomerType != "individual" ||
		len(request.ReturnPath) < 1 || len(request.ReturnPath) > 512 || request.ReturnPath[0] != '/' ||
		(len(request.ReturnPath) > 1 && request.ReturnPath[1] == '/') || request.VerifiedAt.IsZero() {
		return gerror.New("onboardingcap: verified intent request is invalid")
	}
	return nil
}

func ValidateResult(result VerifiedIntent) error {
	if len(result.IntentNo) != 36 || len(result.Token) != 43 || result.ExpiresAt.IsZero() {
		return gerror.New("onboardingcap: verified intent result is invalid")
	}
	return nil
}
