// This file binds recovery challenges and OTPs to one current customer contact,
// browser and account revision. Contact aliases are never returned to guests.
package notification

import (
	"context"
	"encoding/base64"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-shared-services/backend/cap/challengecap"
	"strconv"
)

const (
	recoveryPurpose = "account.recovery"
	recoveryEvent   = "account.password_recovery_code"
)

// RecoveryInput identifies the verified contact and random browser binding.
type RecoveryInput struct {
	Kind                          customerlogincap.Kind
	Account, BrowserNonce, Locale string
}

// RecoveryChallengeInput chooses a supported human-verification presentation.
type RecoveryChallengeInput struct {
	RecoveryInput
	PresentationKind string
}

// RecoveryCompletionInput answers the purpose-bound human challenge.
type RecoveryCompletionInput struct {
	RecoveryInput
	ChallengeNo, Answer, ProviderToken string
}

// RecoveryCodeInput consumes a human proof before sending a rate-limited OTP.
type RecoveryCodeInput struct {
	RecoveryInput
	ProofToken, IdempotencyKey, ClientIP string
}

// RecoveryResetInput consumes the OTP while changing only this account password.
type RecoveryResetInput struct {
	RecoveryInput
	VerificationNo, Code, Password string
}

// RecoveryResult distinguishes saved passwords from outstanding session cleanup.
type RecoveryResult struct {
	Reset          bool
	CleanupToken   string
	CleanupPending bool
}

func (s *serviceImpl) recoveryBinding(ctx context.Context, input RecoveryInput, revision int64) (*loginContactBinding, error) {
	if !browserNoncePattern.MatchString(input.BrowserNonce) || !validLocale(input.Locale) || (input.Kind != customerlogincap.Email && input.Kind != customerlogincap.Phone) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if err := s.loginAuth.RecoveryEnabled(ctx); err != nil {
		return nil, err
	}
	identity, err := s.loginDirectory.Resolve(ctx, input.Kind, input.Account)
	if err != nil {
		return nil, err
	}
	channel, destination := "email", identity.Email
	if input.Kind == customerlogincap.Phone {
		channel, destination = "sms", identity.Phone
	}
	normalized, err := normalizeLoginContact(channel, destination)
	if err != nil {
		return nil, err
	}
	digests, err := s.hmacDigests(ctx, map[string][]byte{"contact_owner": []byte(strconv.Itoa(identity.UserID)), "contact_session": []byte(input.BrowserNonce), "contact_snapshot": []byte(digestText(identity.Email, identity.Phone, strconv.FormatInt(revision, 10))), "contact_destination": normalized})
	if err != nil {
		clear(normalized)
		return nil, err
	}
	binding := &loginContactBinding{userID: identity.UserID, kind: ContactKind(input.Kind), destination: normalized, channel: channel, locale: input.Locale, purpose: recoveryPurpose, event: recoveryEvent, ownerDigest: digests["contact_owner"], sessionDigest: digests["contact_session"], snapshotDigest: digests["contact_snapshot"], destinationDigest: digests["contact_destination"]}
	binding.shape = digestText(recoveryPurpose, string(input.Kind), binding.ownerDigest, binding.sessionDigest, binding.snapshotDigest, binding.destinationDigest)
	return binding, nil
}

// CreateRecoveryHumanChallenge creates an accessible or image challenge.
func (s *serviceImpl) CreateRecoveryHumanChallenge(ctx context.Context, input RecoveryChallengeInput) (*PresalesHumanChallenge, error) {
	binding, err := s.recoveryBinding(ctx, input.RecoveryInput, 0)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	kind := challengecap.PresentationKind(input.PresentationKind)
	if kind != challengecap.PresentationAccessible && kind != challengecap.PresentationImage {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	// The human proof excludes mutable revision; OTP binding re-reads it under lock.
	presentation, err := s.challenges.Create(ctx, challengecap.CreateRequest{Purpose: recoveryPurpose, Binding: digestText(recoveryPurpose, binding.ownerDigest, binding.sessionDigest, binding.destinationDigest), Kind: kind})
	if err != nil || presentation == nil {
		return nil, errorOrCode(err, CodeHumanVerificationUnavailable)
	}
	item := &PresalesHumanChallenge{ChallengeNo: presentation.ChallengeNo, ProviderCode: presentation.ProviderCode, ProviderAction: presentation.ProviderAction, Kind: string(presentation.Kind), AccessiblePrompt: presentation.AccessiblePrompt, SiteKey: presentation.SiteKey, ExpiresAt: presentation.ExpiresAt.UTC()}
	if len(presentation.ImageBytes) > 0 {
		if presentation.ImageMediaType != "image/png" {
			return nil, bizerr.NewCode(CodeHumanVerificationUnavailable)
		}
		item.ImageDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(presentation.ImageBytes)
	}
	return item, nil
}

// CompleteRecoveryHumanChallenge returns a single-use send authorization.
func (s *serviceImpl) CompleteRecoveryHumanChallenge(ctx context.Context, input RecoveryCompletionInput) (*PresalesHumanProof, error) {
	if len(input.ChallengeNo) > 96 || len(input.Answer) > 256 || len(input.ProviderToken) > 4096 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.recoveryBinding(ctx, input.RecoveryInput, 0)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	proof, err := s.challenges.Complete(ctx, challengecap.CompleteRequest{ChallengeNo: input.ChallengeNo, Purpose: recoveryPurpose, Binding: digestText(recoveryPurpose, binding.ownerDigest, binding.sessionDigest, binding.destinationDigest), Answer: input.Answer, ProviderToken: input.ProviderToken})
	if err != nil || proof == nil {
		return nil, errorOrCode(err, CodeVerificationInvalid)
	}
	return &PresalesHumanProof{ProofToken: proof.Token, ExpiresAt: proof.ExpiresAt.UTC()}, nil
}

// SendRecoveryCode requires the matching human proof and shared account/IP limits.
func (s *serviceImpl) SendRecoveryCode(ctx context.Context, input RecoveryCodeInput) (*PresalesVerification, error) {
	identity, err := s.loginDirectory.Resolve(ctx, input.Kind, input.Account)
	if err != nil {
		return nil, err
	}
	if input.ProofToken == "" || len(input.ProofToken) > 4096 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	ip, err := normalizeClientIP([]byte(input.ClientIP))
	if err != nil {
		return nil, err
	}
	defer clear(ip)
	return s.sendBoundContactCode(ctx, identity.UserID, input.IdempotencyKey, func(tx context.Context, revision int64) (*loginContactBinding, error) {
		return s.recoveryBinding(tx, input.RecoveryInput, revision)
	}, func(tx context.Context, binding *loginContactBinding) error {
		if _, err := s.challenges.Consume(tx, challengecap.ConsumeRequest{Token: input.ProofToken, Purpose: recoveryPurpose, Binding: digestText(recoveryPurpose, binding.ownerDigest, binding.sessionDigest, binding.destinationDigest)}); err != nil {
			return bizerr.WrapCode(err, CodeVerificationInvalid)
		}
		values, err := s.hmacDigests(tx, map[string][]byte{"recovery_ip": ip})
		if err != nil {
			return err
		}
		for _, limit := range []rateLimit{{"ip", values["recovery_ip"], 3600, 20}, {"ip", values["recovery_ip"], 86400, 100}} {
			if err = s.consumeRateLimit(tx, limit); err != nil {
				return err
			}
		}
		return nil
	})
}

// RecoveryCleanupInput authorizes only session cleanup for a saved reset. It
// carries a separate opaque grant; the consumed OTP is never required again.
type RecoveryCleanupInput struct {
	RecoveryInput
	VerificationNo, CleanupToken string
}
