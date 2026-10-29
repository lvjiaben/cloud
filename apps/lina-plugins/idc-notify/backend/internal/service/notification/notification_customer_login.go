// This file resolves explicit contact login methods, then delegates password
// checking to the host. Stale mappings never expose a mistakenly minted session.
package notification

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/internal/service/loginprofile"
	"strings"
)

// CustomerLoginInput keeps independent usernames distinct from contact aliases.
type CustomerLoginInput struct {
	Kind              customerlogincap.Kind
	Account, Password string
}

// CustomerPasswordLogin validates the live signed profile after password login.
func (s *serviceImpl) CustomerPasswordLogin(ctx context.Context, input CustomerLoginInput) (*loginprofile.Session, error) {
	if input.Kind != customerlogincap.Username && input.Kind != customerlogincap.Email && input.Kind != customerlogincap.Phone || len(input.Account) > 320 || input.Account == "" || len(input.Password) > 128 || input.Password == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	username := input.Account
	var expected *customerlogincap.Identity
	if input.Kind != customerlogincap.Username {
		identity, err := s.loginDirectory.Resolve(ctx, input.Kind, input.Account)
		if err != nil {
			return nil, err
		}
		expected = identity
		username = identity.Username
	}
	session, err := s.loginAuth.PasswordLogin(ctx, username, input.Password)
	if err != nil {
		return nil, err
	}
	// Username login may precede the one-time customer onboarding redemption.
	// Contact login always requires an active, unambiguous customer binding.
	actual, observeErr := s.loginDirectory.Observe(ctx, session.AccessToken)
	if expected != nil && (observeErr != nil || actual.UserID != expected.UserID || !sameLoginContact(input.Kind, input.Account, actual)) {
		if logoutErr := s.loginAuth.Logout(ctx, session.AccessToken); logoutErr != nil {
			logger.Error(ctx, "revoke rejected customer login session failed", "error", logoutErr)
		}
		return nil, bizerr.NewCode(CodeVerificationInvalid)
	}
	if observeErr != nil && expected == nil {
		logger.Debug(ctx, "customer login index not ready until customer onboarding completes")
	}
	if actual != nil {
		if pending, cleanupErr := s.loginAuth.CleanupReset(ctx, session.AccessToken, ""); cleanupErr != nil || pending {
			logger.Warning(ctx, "customer password reset session cleanup remains pending", "error", cleanupErr)
		}
	}
	return session, nil
}
func sameLoginContact(kind customerlogincap.Kind, value string, identity *customerlogincap.Identity) bool {
	if identity == nil {
		return false
	}
	channel, current := "email", identity.Email
	if kind == customerlogincap.Phone {
		channel, current = "sms", identity.Phone
	}
	left, err := normalizeLoginContact(channel, value)
	if err != nil {
		return false
	}
	defer clear(left)
	right, err := normalizeLoginContact(channel, current)
	if err != nil {
		return false
	}
	defer clear(right)
	return strings.EqualFold(string(left), string(right))
}
