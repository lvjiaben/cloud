// This file verifies onboarding validation convergence and opaque number generation.

package principal

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// TestValidateOnboardingIntentCollapsesUnsafeStates verifies all unusable
// states return the same non-enumerating business error.
func TestValidateOnboardingIntentCollapsesUnsafeStates(t *testing.T) {
	var (
		now        = time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
		verifiedAt = now.Add(-time.Minute)
		expiresAt  = now.Add(time.Minute)
		input      = redeemInput{
			UserID:                 21,
			HostRegistrationUserID: 21,
			UsernameDigest:         digestValue("example"),
			TokenDigest:            digestValue("token"),
			BrowserDigest:          strings.Repeat("a", 64),
		}
		valid = entity.CustomerOnboardingIntent{
			TenantId:               0,
			StatusCode:             string(onboardingStatusVerified),
			BrowserNonceDigest:     strings.Repeat("a", 64),
			ContactHash:            strings.Repeat("b", 64),
			CustomerTypeCode:       string(CustomerTypeIndividual),
			HostRegistrationUserId: input.UserID,
			NormalizedUsernameHash: input.UsernameDigest,
			VerifiedContactAt:      &verifiedAt,
			ExpiresAt:              &expiresAt,
		}
	)
	if replay, err := validateOnboardingIntent(&valid, input, now); err != nil || replay {
		t.Fatalf("expected valid first redemption, replay=%v err=%v", replay, err)
	}

	tests := []struct {
		name   string
		mutate func(*entity.CustomerOnboardingIntent)
	}{
		{name: "pending", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.StatusCode = "pending" }},
		{name: "expired", mutate: func(intent *entity.CustomerOnboardingIntent) { expired := now; intent.ExpiresAt = &expired }},
		{name: "unverified contact", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.VerifiedContactAt = nil }},
		{name: "different host identity", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.HostRegistrationUserId++ }},
		{name: "different username", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.NormalizedUsernameHash = digestValue("other") }},
		{name: "missing browser binding", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.BrowserNonceDigest = "" }},
		{name: "malformed browser binding", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.BrowserNonceDigest = strings.Repeat("z", 64) }},
		{name: "missing contact digest", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.ContactHash = "" }},
		{name: "unsupported customer type", mutate: func(intent *entity.CustomerOnboardingIntent) { intent.CustomerTypeCode = "team" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := valid
			test.mutate(&intent)
			if replay, err := validateOnboardingIntent(&intent, input, now); replay || !bizerr.Is(err, CodeOnboardingUnavailable) {
				t.Fatalf("expected uniform unavailable error, replay=%v err=%v", replay, err)
			}
		})
	}
}

// TestValidateOnboardingIntentAllowsOnlySameUserReplay verifies a consumed
// intent converges to its original customer only for the same identity.
func TestValidateOnboardingIntentAllowsOnlySameUserReplay(t *testing.T) {
	input := redeemInput{UserID: 21}
	intent := &entity.CustomerOnboardingIntent{
		StatusCode:       string(onboardingStatusConsumed),
		ConsumedByUserId: input.UserID,
		ResultCustomerNo: "CUS_existing",
	}
	if replay, err := validateOnboardingIntent(intent, input, time.Now()); err != nil || !replay {
		t.Fatalf("expected same-user replay, replay=%v err=%v", replay, err)
	}
	intent.ConsumedByUserId++
	if replay, err := validateOnboardingIntent(intent, input, time.Now()); replay || !bizerr.Is(err, CodeOnboardingUnavailable) {
		t.Fatalf("expected cross-user replay rejection, replay=%v err=%v", replay, err)
	}
}

// TestNewOpaqueBusinessNumberUsesFullEntropy verifies the stable prefix and
// complete 128-bit hexadecimal payload without relying on randomness in tests.
func TestNewOpaqueBusinessNumberUsesFullEntropy(t *testing.T) {
	content := bytes.Repeat([]byte{0xab}, 16)
	number, err := newOpaqueBusinessNumber(bytes.NewReader(content), "CUS_")
	if err != nil {
		t.Fatalf("newOpaqueBusinessNumber returned error: %v", err)
	}
	if number != "CUS_"+strings.Repeat("ab", 16) {
		t.Fatalf("unexpected opaque business number %q", number)
	}
}
