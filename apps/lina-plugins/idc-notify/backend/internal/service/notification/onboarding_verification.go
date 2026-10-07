package notification

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/onboardingcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-idc-notify/backend/internal/model/entity"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

const onboardingVerificationEvent = "account.customer_onboarding_verification_code"

func (s *serviceImpl) SendOnboardingVerificationCode(ctx context.Context, input OnboardingVerificationInput) (*PresalesVerification, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.ClientIP)
	if !validateIdempotencyKey(input.IdempotencyKey) || input.ProofToken == "" || len(input.ProofToken) > 4096 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.prepareOnboardingBinding(ctx, input.OnboardingBindingInput)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	ip, err := normalizeClientIP(input.ClientIP)
	if err != nil {
		return nil, err
	}
	defer clear(ip)
	digests, err := s.hmacDigests(ctx, map[string][]byte{"idempotency": []byte("onboarding:" + input.IdempotencyKey), "ip": ip})
	if err != nil {
		return nil, err
	}
	requestShape := digestText(onboardingChallengePurpose, binding.usernameSHA, binding.contactSHA, binding.browserSHA, binding.customerType, binding.returnPath, binding.locale)
	if existing, loadErr := s.loadVerificationByIdempotency(ctx, digests["idempotency"]); loadErr != nil {
		return nil, publicError(loadErr)
	} else if existing != nil {
		return replayVerification(existing, requestShape)
	}
	plan, err := s.loadPresalesDeliveryPlan(ctx, binding.destinationKind, onboardingVerificationEvent, binding.locale, map[string]string{"code": "000000", "expires_minutes": "10"})
	if err != nil {
		return nil, err
	}
	code, err := randomSixDigitCode()
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer clear(code)
	verificationNo := stableNumber("nver_", digests["idempotency"], requestShape)
	codeValues := append([]byte(verificationNo+":"), code...)
	codeDigest, err := s.hmacDigests(ctx, map[string][]byte{"code": codeValues})
	clear(codeValues)
	if err != nil {
		return nil, err
	}
	rateSubjects, err := s.hmacDigests(ctx, map[string][]byte{"rate_destination": binding.destination, "rate_browser": input.BrowserNonce, "rate_ip": ip, "rate_template": []byte(plan.template.TemplateNo + ":" + strconv.FormatInt(plan.template.Version, 10)), "rate_sms_global": []byte("installation")})
	if err != nil {
		return nil, err
	}
	var prepared *entity.ContactVerification
	createdHere := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.GetValue(txCtx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", onboardingChallengePurpose+":"+binding.destinationDigest+":"+binding.browserDigest); lockErr != nil {
			return lockErr
		}
		if existing, loadErr := s.loadVerificationByIdempotency(txCtx, digests["idempotency"]); loadErr != nil {
			return loadErr
		} else if existing != nil {
			prepared = existing
			return nil
		}
		tooSoon, limitErr := s.database.GetValue(txCtx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_notify_contact_verification WHERE tenant_id=? AND purpose_code=? AND destination_digest=? AND browser_digest=? AND status_code IN ('prepared','sent','result_unknown','delivery_failed') AND retry_after_at>CURRENT_TIMESTAMP)`, notificationTenantID, onboardingChallengePurpose, binding.destinationDigest, binding.browserDigest)
		if limitErr != nil {
			return limitErr
		}
		if tooSoon.Bool() {
			return bizerr.NewCode(CodeVerificationRateLimited)
		}
		if _, consumeErr := s.challenges.Consume(txCtx, challengecap.ConsumeRequest{Token: input.ProofToken, Purpose: onboardingChallengePurpose, Binding: binding.challengeBinding}); consumeErr != nil {
			return bizerr.WrapCode(consumeErr, CodeVerificationInvalid)
		}
		limits := []rateLimit{{"destination", rateSubjects["rate_destination"], 3600, 5}, {"destination", rateSubjects["rate_destination"], 86400, 10}, {"browser", rateSubjects["rate_browser"], 3600, 10}, {"browser", rateSubjects["rate_browser"], 86400, 20}, {"ip", rateSubjects["rate_ip"], 3600, 20}, {"ip", rateSubjects["rate_ip"], 86400, 100}, {"template", rateSubjects["rate_template"], 60, 60}, {"template", rateSubjects["rate_template"], 3600, 500}}
		if binding.destinationKind == "sms" {
			limits = append(limits, rateLimit{"sms_global", rateSubjects["rate_sms_global"], 86400, s.runtimeConfig.SMSVerificationDailyLimit})
		}
		for _, limit := range limits {
			if limitErr := s.consumeRateLimit(txCtx, limit); limitErr != nil {
				return limitErr
			}
		}
		if _, updateErr := s.database.Model(contactVerificationTable).Ctx(txCtx).Where(modeldo.ContactVerification{TenantId: notificationTenantID, PurposeCode: onboardingChallengePurpose, DestinationDigest: binding.destinationDigest, BrowserDigest: binding.browserDigest}).WhereIn("status_code", []string{"prepared", "sent", "result_unknown", "delivery_failed"}).Data(modeldo.ContactVerification{StatusCode: "superseded"}).Update(); updateErr != nil {
			return updateErr
		}
		_, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_notify_contact_verification (tenant_id,verification_no,purpose_code,operation_code,destination_kind,destination_digest,browser_digest,ticket_digest,locale_code,template_no,template_version,request_shape_digest,idempotency_digest,code_digest,masked_destination,status_code,delivery_state,failed_attempts,retry_after_at,expires_at,provider_boundary_crossed) VALUES (?,?,?,?,?,?,?,? ,?,?,?,?,?,?,?,'prepared','prepared',0,CURRENT_TIMESTAMP + INTERVAL '60 seconds',CURRENT_TIMESTAMP + INTERVAL '10 minutes',FALSE)`, notificationTenantID, verificationNo, onboardingChallengePurpose, "create", binding.destinationKind, binding.destinationDigest, binding.browserDigest, shaText(binding.username+":"+binding.customerType+":"+binding.returnPath), binding.locale, plan.template.TemplateNo, plan.template.Version, requestShape, digests["idempotency"], codeDigest["code"], maskTestTarget(binding.destinationKind, string(binding.destination)))
		if insertErr != nil {
			return insertErr
		}
		prepared, insertErr = s.loadVerificationByNo(txCtx, verificationNo)
		createdHere = insertErr == nil
		return insertErr
	})
	if err != nil {
		if existing, loadErr := s.loadVerificationByIdempotency(ctx, digests["idempotency"]); loadErr == nil && existing != nil {
			return replayVerification(existing, requestShape)
		}
		return nil, publicError(err)
	}
	if prepared == nil || prepared.RequestShapeDigest != requestShape {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if !createdHere || prepared.StatusCode != "prepared" {
		return replayVerification(prepared, requestShape)
	}
	result := s.deliverPresalesOnce(ctx, plan, verificationNo, onboardingVerificationEvent, binding.destination, map[string]string{"code": string(code), "expires_minutes": "10"})
	state, status := providerDeliveryState(result)
	_, updateErr := s.database.Model(contactVerificationTable).Ctx(ctx).Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: verificationNo, StatusCode: "prepared"}).Data(modeldo.ContactVerification{StatusCode: status, DeliveryState: state, ProviderBoundaryCrossed: result.CrossedExternalBorder}).Update()
	if updateErr != nil {
		return nil, publicError(updateErr)
	}
	prepared.StatusCode, prepared.DeliveryState, prepared.ProviderBoundaryCrossed = status, state, result.CrossedExternalBorder
	return projectVerification(prepared), nil
}

func (s *serviceImpl) CreateVerifiedOnboardingIntent(ctx context.Context, input OnboardingIntentInput) (*onboardingcap.VerifiedIntent, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.Code)
	if !validateIdempotencyKey(input.IdempotencyKey) || input.VerificationNo == "" || len(input.VerificationNo) > 48 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.prepareOnboardingBinding(ctx, input.OnboardingBindingInput)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	operationDigest := shaText("customer-onboarding-operation-v1\x00" + input.IdempotencyKey)
	requestDigest := digestText(operationDigest, input.VerificationNo, binding.usernameSHA, binding.contactSHA, binding.browserSHA, binding.customerType, binding.returnPath)
	codeValues := append([]byte(input.VerificationNo+":"), input.Code...)
	codeDigest, err := s.hmacDigests(ctx, map[string][]byte{"code": codeValues})
	clear(codeValues)
	if err != nil {
		return nil, err
	}
	var verifiedAt time.Time
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var verification entity.ContactVerification
		if scanErr := s.database.Model(contactVerificationTable).Ctx(txCtx).Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: input.VerificationNo}).LockUpdate().Scan(&verification); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return bizerr.NewCode(CodeVerificationInvalid)
			}
			return scanErr
		}
		if verification.PurposeCode != onboardingChallengePurpose || verification.OperationCode != "create" || verification.DestinationKind != binding.destinationKind || verification.DestinationDigest != binding.destinationDigest || verification.BrowserDigest != binding.browserDigest || verification.LocaleCode != binding.locale || verification.RequestShapeDigest != digestText(onboardingChallengePurpose, binding.usernameSHA, binding.contactSHA, binding.browserSHA, binding.customerType, binding.returnPath, binding.locale) {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		nowValue, nowErr := s.database.GetValue(txCtx, "SELECT CURRENT_TIMESTAMP")
		if nowErr != nil {
			return nowErr
		}
		now := nowValue.Time().UTC()
		if verification.ExpiresAt == nil || !verification.ExpiresAt.After(now) {
			_, _ = s.database.Model(contactVerificationTable).Ctx(txCtx).Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: input.VerificationNo}).Data(modeldo.ContactVerification{StatusCode: "expired"}).Update()
			return bizerr.NewCode(CodeVerificationExpired)
		}
		if !hmac.Equal([]byte(verification.CodeDigest), []byte(codeDigest["code"])) {
			attempts := verification.FailedAttempts + 1
			status := verification.StatusCode
			if attempts >= 5 {
				status = "locked"
			}
			_, updateErr := s.database.Model(contactVerificationTable).Ctx(txCtx).Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: input.VerificationNo}).Data(modeldo.ContactVerification{FailedAttempts: attempts, StatusCode: status}).Update()
			if updateErr != nil {
				return updateErr
			}
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		if verification.StatusCode == "verified" && verification.VerifiedAt != nil {
			verifiedAt = verification.VerifiedAt.UTC()
			return nil
		}
		if verification.StatusCode != "sent" && verification.StatusCode != "result_unknown" {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		verifiedAt = now
		updated, updateErr := s.database.Model(contactVerificationTable).Ctx(txCtx).Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: input.VerificationNo, StatusCode: verification.StatusCode, FailedAttempts: verification.FailedAttempts}).Data(modeldo.ContactVerification{StatusCode: "verified", VerifiedAt: &verifiedAt}).Update()
		if updateErr != nil {
			return updateErr
		}
		return requireOne(updated)
	})
	if err != nil {
		return nil, publicError(err)
	}
	result, err := s.onboarding.CreateVerifiedIntent(ctx, onboardingcap.CreateVerifiedIntentRequest{OperationDigest: operationDigest, RequestDigest: requestDigest, UsernameDigest: binding.usernameSHA, ContactDigest: binding.contactSHA, BrowserDigest: binding.browserSHA, ContactChannel: map[string]string{"sms": "phone", "email": "email"}[binding.destinationKind], CustomerType: binding.customerType, ReturnPath: binding.returnPath, VerifiedAt: verifiedAt})
	if err != nil || result == nil || onboardingcap.ValidateResult(*result) != nil {
		return nil, errorOrCode(err, CodeUnavailable)
	}
	return result, nil
}
