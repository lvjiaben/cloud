// This file sends and verifies session-bound login-contact codes using the
// existing channel adapters, HMAC secret, durable proofs and delivery evidence.
package notification

import (
	"context"
	"crypto/hmac"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-idc-notify/backend/internal/model/entity"
	"strconv"
	"time"
)

// SendLoginContactCode derives the old address server-side and requires its
// successful proof before sending to a new address. Replays never resend mail.
func (s *serviceImpl) SendLoginContactCode(ctx context.Context, input LoginContactInput) (*PresalesVerification, error) {
	return s.sendBoundContactCode(ctx, s.bizctx.Current(ctx).UserID, input.IdempotencyKey, func(tx context.Context, revision int64) (*loginContactBinding, error) {
		return s.currentContact(tx, input, revision)
	}, nil)
}

// sendBoundContactCode reuses delivery, account serialization and durable rate
// limits for authenticated changes and human-verified password recovery.
func (s *serviceImpl) sendBoundContactCode(ctx context.Context, userID int, idempotencyKey string, resolve func(context.Context, int64) (*loginContactBinding, error), authorize func(context.Context, *loginContactBinding) error) (*PresalesVerification, error) {
	if !validateIdempotencyKey(idempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var (
		prepared *entity.ContactVerification
		binding  *loginContactBinding
		plan     *presalesDeliveryPlan
		code     []byte
		created  bool
	)
	defer func() {
		clear(code)
		if binding != nil {
			clear(binding.destination)
		}
	}()
	err := s.lockContact(ctx, userID, func(txCtx context.Context, revision int64) error {
		var err error
		binding, err = resolve(txCtx, revision)
		if err != nil {
			return err
		}
		hashes, err := s.hmacDigests(txCtx, map[string][]byte{"contact_idempotency": []byte(binding.ownerDigest + ":" + idempotencyKey)})
		if err != nil {
			return err
		}
		prior, err := s.loadVerificationByIdempotency(txCtx, hashes["contact_idempotency"])
		if err != nil {
			return err
		}
		if prior != nil {
			if prior.RequestShapeDigest != binding.shape {
				return bizerr.NewCode(CodeConflict)
			}
			prepared = prior
			return nil
		}
		if authorize != nil {
			if err = authorize(txCtx, binding); err != nil {
				return err
			}
		}
		plan, err = s.loadPresalesDeliveryPlan(txCtx, binding.channel, binding.event, binding.locale, map[string]string{"code": "000000", "expires_minutes": "10"})
		if err != nil {
			return err
		}
		if binding.channel == "sms" && plan.template.ProviderCode != *plan.channel.ProviderCode {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		tooSoon, err := s.database.Model(contactVerificationTable).Ctx(txCtx).Where(do.ContactVerification{TenantId: notificationTenantID, TicketDigest: binding.ownerDigest}).WhereIn("purpose_code", []string{contactOldPurpose, contactNewPurpose, recoveryPurpose}).WhereGT("retry_after_at", s.now().UTC()).Where(do.ContactVerification{PurposeCode: binding.purpose}).Count()
		if err != nil {
			return err
		}
		if tooSoon > 0 {
			return bizerr.NewCode(CodeVerificationRateLimited)
		}
		limits := []rateLimit{{"account", binding.ownerDigest, 3600, 10}, {"account", binding.ownerDigest, 86400, 20}, {"destination", binding.destinationDigest, 3600, 5}, {"destination", binding.destinationDigest, 86400, 10}}
		if binding.channel == "sms" {
			hashes, err = s.hmacDigests(txCtx, map[string][]byte{"rate_sms_global": []byte("installation")})
			if err != nil {
				return err
			}
			limits = append(limits, rateLimit{"sms_global", hashes["rate_sms_global"], 86400, s.runtimeConfig.SMSVerificationDailyLimit})
		}
		for _, limit := range limits {
			if err = s.consumeRateLimit(txCtx, limit); err != nil {
				return err
			}
		}
		code, err = randomSixDigitCode()
		if err != nil {
			return err
		}
		idempotency, err := s.hmacDigests(txCtx, map[string][]byte{"contact_idempotency": []byte(binding.ownerDigest + ":" + idempotencyKey)})
		if err != nil {
			return err
		}
		number := stableNumber("nver_", idempotency["contact_idempotency"], binding.shape)
		digest, err := s.hmacDigests(txCtx, map[string][]byte{"contact_code": []byte(number + ":" + string(code))})
		if err != nil {
			return err
		}
		// Resending invalidates previous codes for the same actor/session/step.
		if _, err = s.database.Model(contactVerificationTable).Ctx(txCtx).Where(do.ContactVerification{TenantId: notificationTenantID, PurposeCode: binding.purpose, TicketDigest: binding.ownerDigest, BrowserDigest: binding.sessionDigest}).WhereIn("status_code", []string{contactPrepared, contactSent, contactUnknown, contactVerified}).Data(do.ContactVerification{StatusCode: contactSuperseded}).Update(); err != nil {
			return err
		}
		now := s.now().UTC()
		expires, retry := now.Add(10*time.Minute), now.Add(time.Minute)
		if _, err = s.database.Model(contactVerificationTable).Ctx(txCtx).Data(do.ContactVerification{
			TenantId: notificationTenantID, VerificationNo: number, PurposeCode: binding.purpose, OperationCode: contactCreate, DestinationKind: binding.channel,
			DestinationDigest: binding.destinationDigest, BrowserDigest: binding.sessionDigest, TicketDigest: binding.ownerDigest, LocaleCode: binding.locale,
			TemplateNo: plan.template.TemplateNo, TemplateVersion: plan.template.Version, RequestShapeDigest: binding.shape, IdempotencyDigest: idempotency["contact_idempotency"], CodeDigest: digest["contact_code"],
			MaskedDestination: maskTestTarget(binding.channel, string(binding.destination)), StatusCode: contactPrepared, DeliveryState: contactPrepared, FailedAttempts: 0, RetryAfterAt: &retry, ExpiresAt: &expires, ProviderBoundaryCrossed: false,
		}).Insert(); err != nil {
			return err
		}
		prepared, err = s.loadVerificationByNo(txCtx, number)
		created = err == nil
		return err
	})
	if err != nil {
		return nil, publicError(err)
	}
	if !created {
		return replayVerification(prepared, binding.shape)
	}
	result := s.deliverPresalesOnce(ctx, plan, prepared.VerificationNo, binding.event, binding.destination, map[string]string{"code": string(code), "expires_minutes": "10"})
	state, status := providerDeliveryState(result)
	if _, err = s.database.Model(contactVerificationTable).Ctx(ctx).Where(do.ContactVerification{TenantId: notificationTenantID, VerificationNo: prepared.VerificationNo, StatusCode: contactPrepared}).Data(do.ContactVerification{StatusCode: status, DeliveryState: state, ProviderBoundaryCrossed: result.CrossedExternalBorder}).Update(); err != nil {
		return nil, publicError(err)
	}
	prepared.StatusCode, prepared.DeliveryState, prepared.ProviderBoundaryCrossed = status, state, result.CrossedExternalBorder
	return projectVerification(prepared), nil
}

// VerifyLoginContactCode persists failed attempts even when the response rejects
// the code. No profile write occurs at this step.
func (s *serviceImpl) VerifyLoginContactCode(ctx context.Context, input LoginContactVerifyInput) error {
	return s.verifyBoundContactCode(ctx, s.bizctx.Current(ctx).UserID, input.VerificationNo, input.Code, func(tx context.Context, revision int64) (*loginContactBinding, error) {
		return s.currentContact(tx, input.LoginContactInput, revision)
	})
}

func (s *serviceImpl) verifyBoundContactCode(ctx context.Context, userID int, verificationNo, code string, resolve func(context.Context, int64) (*loginContactBinding, error)) error {
	if len(code) != 6 || len(verificationNo) > 48 || verificationNo == "" {
		return bizerr.NewCode(CodeVerificationInvalid)
	}
	if _, err := strconv.Atoi(code); err != nil {
		return bizerr.NewCode(CodeVerificationInvalid)
	}
	var rejection error
	err := s.lockContact(ctx, userID, func(txCtx context.Context, revision int64) error {
		binding, err := resolve(txCtx, revision)
		if err != nil {
			return err
		}
		defer clear(binding.destination)
		row, err := s.loadVerificationByNo(txCtx, verificationNo)
		if err != nil {
			return err
		}
		if !contactProofMatches(row, binding, binding.purpose, binding.shape) || row.ExpiresAt == nil || !row.ExpiresAt.After(s.now().UTC()) || row.FailedAttempts >= 5 || (row.StatusCode != contactSent && row.StatusCode != contactUnknown && row.StatusCode != contactVerified) {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		digest, err := s.hmacDigests(txCtx, map[string][]byte{"contact_code": []byte(verificationNo + ":" + code)})
		if err != nil {
			return err
		}
		if !hmac.Equal([]byte(row.CodeDigest), []byte(digest["contact_code"])) {
			attempts, status := row.FailedAttempts+1, row.StatusCode
			if attempts >= 5 {
				status = contactLocked
			}
			if _, err = s.database.Model(contactVerificationTable).Ctx(txCtx).Where(do.ContactVerification{TenantId: notificationTenantID, VerificationNo: row.VerificationNo}).Data(do.ContactVerification{FailedAttempts: attempts, StatusCode: status}).Update(); err != nil {
				return err
			}
			rejection = bizerr.NewCode(CodeVerificationInvalid)
			return nil
		}
		if row.StatusCode == contactVerified {
			return nil
		}
		now := s.now().UTC()
		_, err = s.database.Model(contactVerificationTable).Ctx(txCtx).Where(do.ContactVerification{TenantId: notificationTenantID, VerificationNo: row.VerificationNo}).Data(do.ContactVerification{StatusCode: contactVerified, VerifiedAt: &now}).Update()
		return err
	})
	if err != nil {
		return publicError(err)
	}
	return rejection
}

func contactProofMatches(row *entity.ContactVerification, binding *loginContactBinding, purpose, shape string) bool {
	return row != nil && row.PurposeCode == purpose && row.OperationCode == contactCreate && row.TicketDigest == binding.ownerDigest && row.BrowserDigest == binding.sessionDigest && row.RequestShapeDigest == shape
}

func validContactProof(row *entity.ContactVerification, binding *loginContactBinding, purpose, shape string, now time.Time) bool {
	return contactProofMatches(row, binding, purpose, shape) && row.StatusCode == contactVerified && row.VerifiedAt != nil && row.ExpiresAt != nil && row.ExpiresAt.After(now) && row.FailedAttempts < 5
}
