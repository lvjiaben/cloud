// This file keeps temporary verified-identity sessions on the server and commits
// password writes through the original authenticated host profile transaction.
package notification

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/authcap/extlogin"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-idc-notify/backend/internal/model/entity"
	"time"
)

// recoveryWriteKey is private to the verified server-side transaction.
type recoveryWriteKey struct{}

type recoveryProof struct {
	RecoveryInput
	VerificationNo string
	CleanupToken   string
}

// ResetCustomerPassword verifies the one-time code before requesting a host
// session. A failed write preserves the proof; a saved password remains success
// even when revoking prior sessions needs a later retry.
func (s *serviceImpl) ResetCustomerPassword(ctx context.Context, input RecoveryResetInput) (*RecoveryResult, error) {
	if len(input.Password) < 8 || len(input.Password) > 32 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	identity, err := s.loginDirectory.Resolve(ctx, input.Kind, input.Account)
	if err != nil {
		return nil, err
	}
	err = s.verifyBoundContactCode(ctx, identity.UserID, input.VerificationNo, input.Code, func(tx context.Context, revision int64) (*loginContactBinding, error) {
		return s.recoveryBinding(tx, input.RecoveryInput, revision)
	})
	if err != nil {
		return nil, err
	}
	session, err := s.externalLogin.LoginByVerifiedIdentity(ctx, extlogin.LoginInput{Provider: customerlogincap.IdentityProviderID, Subject: identity.CustomerNo, AllowAutoProvision: false})
	if err != nil {
		return nil, err
	}
	if session == nil || session.AccessToken == "" {
		return nil, bizerr.NewCode(CodeVerificationInvalid)
	}
	revoked := false
	defer func() {
		if !revoked {
			if logoutErr := s.loginAuth.Logout(ctx, session.AccessToken); logoutErr != nil {
				logger.Error(ctx, "revoke temporary password recovery session failed", "error", logoutErr)
			}
		}
	}()
	current, err := s.loginDirectory.Observe(ctx, session.AccessToken)
	if err != nil || current.UserID != identity.UserID || !sameLoginContact(input.Kind, input.Account, current) {
		return nil, errorOrCode(err, CodeVerificationInvalid)
	}
	tokens, err := s.hmacDigests(ctx, map[string][]byte{"recovery_cleanup": []byte(digestText(input.VerificationNo, input.BrowserNonce, identity.CustomerNo))})
	if err != nil {
		return nil, err
	}
	cleanupToken := tokens["recovery_cleanup"]
	raw, err := json.Marshal(recoveryProof{input.RecoveryInput, input.VerificationNo, cleanupToken})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	defer clear(raw)
	if err = s.loginAuth.SetPassword(ctx, session.AccessToken, input.Password, base64.RawURLEncoding.EncodeToString(raw)); err != nil {
		return nil, err
	}
	pending, cleanupErr := s.loginAuth.CleanupReset(ctx, session.AccessToken, input.VerificationNo)
	if cleanupErr != nil {
		logger.Warning(ctx, "password saved; session cleanup pending", "error", cleanupErr)
		pending = true
	}
	revoked = cleanupErr == nil && !pending
	return &RecoveryResult{Reset: true, CleanupPending: pending, CleanupToken: cleanupToken}, nil
}

// ApplyRecovery authorizes only one original password-only write for the real
// bearer-authenticated account. Both contacts and guard revision must still match.
func (s *serviceImpl) ApplyRecovery(ctx context.Context, proof string, write profilechangecap.Write) error {
	if len(proof) > 2048 || write == nil {
		return bizerr.NewCode(CodeVerificationInvalid)
	}
	raw, err := base64.RawURLEncoding.DecodeString(proof)
	if err != nil {
		return bizerr.NewCode(CodeVerificationInvalid)
	}
	defer clear(raw)
	var input recoveryProof
	if err = json.Unmarshal(raw, &input); err != nil {
		return bizerr.NewCode(CodeVerificationInvalid)
	}
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != notificationTenantID || current.TokenID == "" || current.IsImpersonation || current.ActingUserID != 0 {
		return bizerr.NewCode(CodeNotAccessible)
	}
	return publicError(s.lockContact(ctx, current.UserID, func(tx context.Context, revision int64) error {
		binding, err := s.recoveryBinding(tx, input.RecoveryInput, revision)
		if err != nil {
			return err
		}
		defer clear(binding.destination)
		if binding.userID != current.UserID {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		profile, err := s.loginProfiles.Current(tx)
		if err != nil {
			return err
		}
		identity, err := s.loginDirectory.Resolve(tx, input.Kind, input.Account)
		if err != nil {
			return err
		}
		if profile.ID != current.UserID || profile.Email != identity.Email || profile.Phone != identity.Phone {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		row, err := s.loadVerificationByNo(tx, input.VerificationNo)
		if err != nil {
			return err
		}
		if !validContactProof(row, binding, recoveryPurpose, binding.shape, s.now().UTC()) {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		result, err := s.database.Model(contactVerificationTable).Ctx(tx).Where(do.ContactVerification{TenantId: notificationTenantID, VerificationNo: input.VerificationNo, StatusCode: contactVerified}).Data(do.ContactVerification{StatusCode: contactSuperseded}).Update()
		if err != nil {
			return err
		}
		if err = requireOne(result); err != nil {
			return err
		}
		if len(input.CleanupToken) != 64 {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		if err = s.captureRecoverySessions(tx, input.VerificationNo); err != nil {
			return err
		}
		if err = write(context.WithValue(tx, recoveryWriteKey{}, current.TokenID)); err != nil {
			return err
		}
		if err = s.captureRecoverySessions(tx, input.VerificationNo); err != nil {
			return err
		}
		completedAt := s.now().UTC()
		result, err = s.database.Model(contactGuardTable).Ctx(tx).Where(do.LoginContactGuard{TenantId: notificationTenantID, UserId: current.UserID, Revision: revision}).Data(do.LoginContactGuard{Revision: revision + 1, RecoveryNo: input.VerificationNo, CleanupPending: true, RecoveryAt: &completedAt, RecoveryRevision: revision + 1, CleanupTokenDigest: shaText(input.CleanupToken)}).Update()
		if err != nil {
			return err
		}
		return requireOne(result)
	}))
}

// RetryRecoveryCleanup uses a narrow post-reset grant and never changes a
// password or exposes a login session. Newer sessions are outside its cutoff.
func (s *serviceImpl) RetryRecoveryCleanup(ctx context.Context, input RecoveryCleanupInput) (bool, error) {
	if len(input.CleanupToken) != 64 || input.VerificationNo == "" || len(input.VerificationNo) > 48 {
		return true, bizerr.NewCode(CodeVerificationInvalid)
	}
	identity, err := s.loginDirectory.Resolve(ctx, input.Kind, input.Account)
	if err != nil {
		return true, err
	}
	var guard entity.LoginContactGuard
	if err = s.database.Model(contactGuardTable).Ctx(ctx).Where(do.LoginContactGuard{TenantId: notificationTenantID, UserId: identity.UserID}).Scan(&guard); err != nil {
		return true, publicError(err)
	}
	if guard.RecoveryNo != input.VerificationNo || guard.RecoveryRevision != guard.Revision || guard.RecoveryAt.IsZero() || guard.RecoveryAt.Before(s.now().UTC().Add(-10*time.Minute)) || !hmac.Equal([]byte(guard.CleanupTokenDigest), []byte(shaText(input.CleanupToken))) {
		return true, bizerr.NewCode(CodeVerificationInvalid)
	}
	tokens, tokenErr := s.hmacDigests(ctx, map[string][]byte{"recovery_cleanup": []byte(digestText(input.VerificationNo, input.BrowserNonce, identity.CustomerNo))})
	if tokenErr != nil {
		return true, tokenErr
	}
	if !hmac.Equal([]byte(tokens["recovery_cleanup"]), []byte(input.CleanupToken)) {
		return true, bizerr.NewCode(CodeVerificationInvalid)
	}
	if !guard.CleanupPending {
		return false, nil
	}
	session, err := s.externalLogin.LoginByVerifiedIdentity(ctx, extlogin.LoginInput{Provider: customerlogincap.IdentityProviderID, Subject: identity.CustomerNo})
	if err != nil {
		return true, err
	}
	if session == nil || session.AccessToken == "" {
		return true, bizerr.NewCode(CodeVerificationInvalid)
	}
	defer func() {
		if logoutErr := s.loginAuth.Logout(ctx, session.AccessToken); logoutErr != nil {
			logger.Error(ctx, "revoke cleanup-only temporary session failed", "error", logoutErr)
		}
	}()
	return s.loginAuth.CleanupReset(ctx, session.AccessToken, input.VerificationNo)
}
