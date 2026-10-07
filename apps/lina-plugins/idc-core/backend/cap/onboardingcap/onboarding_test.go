package onboardingcap

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCreateRequestRequiresPersonalFirstAccount(t *testing.T) {
	request := CreateVerifiedIntentRequest{
		OperationDigest: strings.Repeat("1", 64),
		RequestDigest:   strings.Repeat("2", 64),
		UsernameDigest:  strings.Repeat("3", 64),
		ContactDigest:   strings.Repeat("4", 64),
		BrowserDigest:   strings.Repeat("5", 64),
		ContactChannel:  "email",
		CustomerType:    "individual",
		ReturnPath:      "/account",
		VerifiedAt:      time.Now().UTC(),
	}
	if err := ValidateCreateRequest(request); err != nil {
		t.Fatalf("individual onboarding rejected: %v", err)
	}
	request.CustomerType = "enterprise"
	if err := ValidateCreateRequest(request); err == nil {
		t.Fatal("enterprise-first onboarding unexpectedly accepted")
	}
}
