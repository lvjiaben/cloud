package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	"lina-plugin-idc-notify/backend/cap/securitycodecap"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

const (
	securityCodePurposeTable      = "plugin_idc_notify_security_code_purpose"
	securityCodeVerificationTable = "plugin_idc_notify_security_code_verification"
)

var securityCodeSubjectPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{2,127}$`)

type securityCodeService struct {
	database             gdb.DB
	callerPluginID       string
	challenges           challengecap.Service
	verificationAccessor secretcap.Accessor
	verificationHandle   secretcap.Handle
	runtimeConfig        RuntimeConfig
	adapters             map[string]adapterBinding
	now                  func() time.Time
}

type securityCodePurposeRow struct {
	CallerPluginID string `orm:"caller_plugin_id"`
	PurposeCode    string `orm:"purpose_code"`
	EventCode      string `orm:"event_code"`
	Enabled        bool   `orm:"enabled"`
	Version        int64  `orm:"version"`
}

type securityCodeVerificationRow struct {
	VerificationNo          string     `orm:"verification_no"`
	CallerPluginID          string     `orm:"caller_plugin_id"`
	PurposeCode             string     `orm:"purpose_code"`
	PurposeVersion          int64      `orm:"purpose_version"`
	SubjectDigest           string     `orm:"subject_digest"`
	DestinationDigest       string     `orm:"destination_digest"`
	BrowserDigest           string     `orm:"browser_digest"`
	LocaleCode              string     `orm:"locale_code"`
	TemplateNo              string     `orm:"template_no"`
	TemplateVersion         int64      `orm:"template_version"`
	RequestShapeDigest      string     `orm:"request_shape_digest"`
	IdempotencyDigest       string     `orm:"idempotency_digest"`
	CodeDigest              string     `orm:"code_digest"`
	MaskedDestination       string     `orm:"masked_destination"`
	StatusCode              string     `orm:"status_code"`
	DeliveryState           string     `orm:"delivery_state"`
	FailedAttempts          int        `orm:"failed_attempts"`
	VerificationVersion     int64      `orm:"verification_version"`
	RetryAfterAt            *time.Time `orm:"retry_after_at"`
	ExpiresAt               *time.Time `orm:"expires_at"`
	ProviderBoundaryCrossed bool       `orm:"provider_boundary_crossed"`
	VerifiedAt              *time.Time `orm:"verified_at"`
}

type securityCodeBinding struct {
	purpose           string
	subjectNo         string
	destination       []byte
	locale            string
	subjectDigest     string
	destinationDigest string
	browserDigest     string
	challengePurpose  string
	challengeBinding  string
}

type securityCodeDeliveryPlan struct {
	channel       channelRow
	template      *templateRow
	adapter       adapterBinding
	credential    secretcap.Handle
	configuration map[string]string
}

var _ securitycodecap.Service = (*securityCodeService)(nil)

func NewSecurityCodeService(
	database gdb.DB,
	callerPluginID string,
	challenges challengecap.Service,
	verificationAccessor secretcap.Accessor,
	verificationHandle secretcap.Handle,
	runtimeConfig RuntimeConfig,
	secrets map[string]SecretBinding,
	descriptors []channelcap.Descriptor,
) (securitycodecap.Service, error) {
	if database == nil || !stableCodePattern.MatchString(callerPluginID) || nilDependency(challenges) ||
		nilDependency(verificationAccessor) || verificationHandle.IsZero() || runtimeConfig.SMSVerificationDailyLimit < 1 {
		return nil, gerror.New("security-code service dependencies are invalid")
	}
	smsSecrets, exists := secrets["sms"]
	if !exists || nilDependency(smsSecrets.Accessor) {
		return nil, gerror.New("security-code SMS secret binding is unavailable")
	}
	adapters := make(map[string]adapterBinding)
	for _, descriptor := range descriptors {
		if descriptor.Kind != channelcap.KindSMS {
			continue
		}
		if err := channelcap.ValidateDescriptor(descriptor); err != nil {
			return nil, err
		}
		adapter, err := descriptor.Factory(smsSecrets.Accessor)
		if err != nil {
			return nil, gerror.Wrap(err, "construct security-code SMS adapter")
		}
		if validationErr := channelcap.ValidateAdapter(adapter); validationErr != nil {
			return nil, gerror.Wrap(validationErr, "validate security-code SMS adapter")
		}
		adapters[descriptor.ProviderCode] = adapterBinding{descriptor: descriptor, adapter: adapter}
	}
	if len(adapters) == 0 {
		return nil, gerror.New("security-code SMS adapter is unavailable")
	}
	return &securityCodeService{database: database, callerPluginID: callerPluginID, challenges: challenges,
		verificationAccessor: verificationAccessor, verificationHandle: verificationHandle,
		runtimeConfig: runtimeConfig, adapters: adapters, now: time.Now}, nil
}

func (s *securityCodeService) CreateHumanChallenge(ctx context.Context, input securitycodecap.HumanChallengeInput) (*securitycodecap.HumanChallenge, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	binding, err := s.prepareBinding(ctx, input.Binding)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	kind := challengecap.PresentationKind(input.PresentationKind)
	if kind != challengecap.PresentationAccessible && kind != challengecap.PresentationImage {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	presentation, err := s.challenges.Create(ctx, challengecap.CreateRequest{Purpose: binding.challengePurpose,
		Binding: binding.challengeBinding, Kind: kind})
	if err != nil || presentation == nil {
		return nil, errorOrCode(err, CodeHumanVerificationUnavailable)
	}
	item := &securitycodecap.HumanChallenge{ChallengeNo: presentation.ChallengeNo, ProviderCode: presentation.ProviderCode,
		ProviderAction: presentation.ProviderAction, Kind: string(presentation.Kind), AccessiblePrompt: presentation.AccessiblePrompt,
		SiteKey: presentation.SiteKey, ExpiresAt: presentation.ExpiresAt.UTC()}
	if len(presentation.ImageBytes) > 0 {
		if presentation.ImageMediaType != "image/png" {
			return nil, bizerr.NewCode(CodeHumanVerificationUnavailable)
		}
		item.ImageDataURL = "data:" + presentation.ImageMediaType + ";base64," + base64.StdEncoding.EncodeToString(presentation.ImageBytes)
	}
	return item, nil
}

func (s *securityCodeService) CompleteHumanChallenge(ctx context.Context, input securitycodecap.HumanCompletionInput) (*securitycodecap.HumanProof, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.Answer)
	defer clear(input.ProviderToken)
	binding, err := s.prepareBinding(ctx, input.Binding)
	if err != nil || input.ChallengeNo == "" || len(input.ChallengeNo) > 96 || len(input.Answer) > 256 || len(input.ProviderToken) > 4096 {
		return nil, errorOrCode(err, CodeInvalid)
	}
	defer clear(binding.destination)
	proof, err := s.challenges.Complete(ctx, challengecap.CompleteRequest{ChallengeNo: input.ChallengeNo,
		Purpose: binding.challengePurpose, Binding: binding.challengeBinding,
		Answer: string(input.Answer), ProviderToken: string(input.ProviderToken)})
	if err != nil || proof == nil {
		return nil, errorOrCode(err, CodeVerificationInvalid)
	}
	return &securitycodecap.HumanProof{ProofToken: proof.Token, ExpiresAt: proof.ExpiresAt.UTC()}, nil
}

func (s *securityCodeService) Send(ctx context.Context, input securitycodecap.SendInput) (*securitycodecap.Verification, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.ClientIP)
	if !validateIdempotencyKey(input.IdempotencyKey) || input.ProofToken == "" || len(input.ProofToken) > 4096 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.prepareBinding(ctx, input.Binding)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	ip, err := normalizeClientIP(input.ClientIP)
	if err != nil {
		return nil, err
	}
	defer clear(ip)
	purpose, err := s.loadPurpose(ctx, binding.purpose)
	if err != nil || purpose == nil {
		return nil, errorOrCode(err, CodeChannelUnavailable)
	}
	variables := map[string]string{"code": "000000", "expires_minutes": "10"}
	plan, err := s.loadDeliveryPlan(ctx, purpose.EventCode, binding.locale, variables)
	if err != nil {
		return nil, err
	}
	digests, err := s.digests(ctx, map[string][]byte{
		"idempotency": []byte(input.IdempotencyKey), "ip": ip,
		"rate_destination": binding.destination, "rate_subject": []byte(binding.subjectNo),
		"rate_browser":    input.BrowserNonce,
		"rate_template":   []byte(plan.template.TemplateNo + ":" + strconv.FormatInt(plan.template.Version, 10)),
		"rate_sms_global": []byte("installation"),
	})
	if err != nil {
		return nil, err
	}
	requestShape := digestText(s.callerPluginID, binding.purpose, binding.subjectDigest, binding.destinationDigest,
		binding.browserDigest, binding.locale, strconv.FormatInt(purpose.Version, 10))
	if existing, loadErr := s.loadByIdempotency(ctx, digests["idempotency"]); loadErr != nil {
		return nil, publicError(loadErr)
	} else if existing != nil {
		return replaySecurityCode(existing, requestShape)
	}
	code, err := randomSixDigitCode()
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer clear(code)
	verificationNo := stableNumber("nsec_", s.callerPluginID, digests["idempotency"], requestShape)
	codeValue := append([]byte(verificationNo+":"), code...)
	codeDigests, err := s.digests(ctx, map[string][]byte{"code": codeValue})
	clear(codeValue)
	if err != nil {
		return nil, err
	}
	var prepared *securityCodeVerificationRow
	createdHere := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.GetValue(txCtx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
			s.callerPluginID+":"+binding.purpose+":"+binding.subjectDigest+":"+binding.destinationDigest); lockErr != nil {
			return lockErr
		}
		if existing, loadErr := s.loadByIdempotency(txCtx, digests["idempotency"]); loadErr != nil {
			return loadErr
		} else if existing != nil {
			prepared = existing
			return nil
		}
		tooSoon, limitErr := s.database.GetValue(txCtx, `SELECT EXISTS(
SELECT 1 FROM plugin_idc_notify_security_code_verification
WHERE tenant_id=? AND caller_plugin_id=? AND purpose_code=? AND subject_digest=?
  AND destination_digest=? AND browser_digest=?
  AND status_code IN ('prepared','sent','result_unknown','delivery_failed') AND retry_after_at>CURRENT_TIMESTAMP
)`, notificationTenantID, s.callerPluginID, binding.purpose, binding.subjectDigest,
			binding.destinationDigest, binding.browserDigest)
		if limitErr != nil {
			return limitErr
		}
		if tooSoon.Bool() {
			return bizerr.NewCode(CodeVerificationRateLimited)
		}
		if _, consumeErr := s.challenges.Consume(txCtx, challengecap.ConsumeRequest{Token: input.ProofToken,
			Purpose: binding.challengePurpose, Binding: binding.challengeBinding}); consumeErr != nil {
			return bizerr.WrapCode(consumeErr, CodeVerificationInvalid)
		}
		limits := []rateLimit{
			{"destination", digests["rate_destination"], 3600, 5}, {"destination", digests["rate_destination"], 86400, 10},
			{"account", digests["rate_subject"], 3600, 10}, {"account", digests["rate_subject"], 86400, 20},
			{"browser", digests["rate_browser"], 3600, 10}, {"browser", digests["rate_browser"], 86400, 20},
			{"ip", digests["ip"], 3600, 20}, {"ip", digests["ip"], 86400, 100},
			{"template", digests["rate_template"], 60, 60}, {"template", digests["rate_template"], 3600, 500},
			{"sms_global", digests["rate_sms_global"], 86400, s.runtimeConfig.SMSVerificationDailyLimit},
		}
		for _, limit := range limits {
			if limitErr := s.consumeRateLimit(txCtx, limit); limitErr != nil {
				return limitErr
			}
		}
		if _, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_notify_security_code_verification
SET status_code='superseded',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND caller_plugin_id=? AND purpose_code=? AND subject_digest=? AND destination_digest=?
  AND browser_digest=? AND status_code IN ('prepared','sent','result_unknown','delivery_failed')`,
			notificationTenantID, s.callerPluginID, binding.purpose, binding.subjectDigest,
			binding.destinationDigest, binding.browserDigest); updateErr != nil {
			return updateErr
		}
		_, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_notify_security_code_verification
(tenant_id,verification_no,caller_plugin_id,purpose_code,purpose_version,subject_digest,destination_digest,browser_digest,
 locale_code,template_no,template_version,request_shape_digest,idempotency_digest,code_digest,masked_destination,
 status_code,delivery_state,failed_attempts,verification_version,retry_after_at,expires_at,provider_boundary_crossed)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'prepared','prepared',0,0,CURRENT_TIMESTAMP + INTERVAL '60 seconds',
        CURRENT_TIMESTAMP + INTERVAL '10 minutes',FALSE)`, notificationTenantID, verificationNo, s.callerPluginID,
			binding.purpose, purpose.Version, binding.subjectDigest, binding.destinationDigest, binding.browserDigest,
			binding.locale, plan.template.TemplateNo, plan.template.Version, requestShape, digests["idempotency"],
			codeDigests["code"], maskTestTarget("sms", string(binding.destination)))
		if insertErr != nil {
			return insertErr
		}
		prepared, insertErr = s.loadByNo(txCtx, verificationNo)
		createdHere = insertErr == nil
		return insertErr
	})
	if err != nil {
		if existing, loadErr := s.loadByIdempotency(ctx, digests["idempotency"]); loadErr == nil && existing != nil {
			return replaySecurityCode(existing, requestShape)
		}
		return nil, publicError(err)
	}
	if prepared == nil || prepared.RequestShapeDigest != requestShape {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if !createdHere || prepared.StatusCode != "prepared" {
		return replaySecurityCode(prepared, requestShape)
	}
	variables["code"] = string(code)
	result := s.deliverOnce(ctx, plan, verificationNo, purpose.EventCode, binding.destination, variables)
	state, status := providerDeliveryState(result)
	if _, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_notify_security_code_verification
SET status_code=?,delivery_state=?,provider_boundary_crossed=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND verification_no=? AND status_code='prepared'`, status, state, result.CrossedExternalBorder,
		notificationTenantID, verificationNo); updateErr != nil {
		return nil, publicError(updateErr)
	}
	prepared.StatusCode, prepared.DeliveryState, prepared.ProviderBoundaryCrossed = status, state, result.CrossedExternalBorder
	return projectSecurityCode(prepared), nil
}

func (s *securityCodeService) Verify(ctx context.Context, input securitycodecap.VerifyInput) (*securitycodecap.Verified, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.Code)
	if len(input.Code) != 6 || input.VerificationNo == "" || len(input.VerificationNo) > 48 {
		return nil, bizerr.NewCode(CodeVerificationInvalid)
	}
	for _, digit := range input.Code {
		if digit < '0' || digit > '9' {
			return nil, bizerr.NewCode(CodeVerificationInvalid)
		}
	}
	binding, err := s.prepareBinding(ctx, input.Binding)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	codeValue := append([]byte(input.VerificationNo+":"), input.Code...)
	digests, err := s.digests(ctx, map[string][]byte{"code": codeValue})
	clear(codeValue)
	if err != nil {
		return nil, err
	}
	var verified *securitycodecap.Verified
	var businessErr error
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var row securityCodeVerificationRow
		if loadErr := s.database.Model(securityCodeVerificationTable).Ctx(txCtx).
			Where(tenantIDColumn, notificationTenantID).Where("verification_no", input.VerificationNo).
			LockUpdate().Scan(&row); loadErr != nil {
			return loadErr
		}
		if !securityCodeMatches(row, s.callerPluginID, binding) || securityCodeTerminal(row.StatusCode) {
			businessErr = bizerr.NewCode(CodeVerificationInvalid)
			return nil
		}
		nowValue, nowErr := s.database.GetValue(txCtx, "SELECT CURRENT_TIMESTAMP")
		if nowErr != nil {
			return nowErr
		}
		now := nowValue.Time()
		if row.ExpiresAt == nil || !row.ExpiresAt.After(now) {
			_, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_notify_security_code_verification
SET status_code='expired',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND verification_no=?`,
				notificationTenantID, input.VerificationNo)
			businessErr = bizerr.NewCode(CodeVerificationExpired)
			return updateErr
		}
		if row.StatusCode == "result_unknown" || row.DeliveryState == "result_unknown" {
			businessErr = bizerr.NewCode(CodeVerificationResultUnknown)
			return nil
		}
		if row.StatusCode != "sent" || row.DeliveryState != "accepted" && row.DeliveryState != "delivered" {
			businessErr = bizerr.NewCode(CodeVerificationInvalid)
			return nil
		}
		if !hmac.Equal([]byte(row.CodeDigest), []byte(digests["code"])) {
			attempts := row.FailedAttempts + 1
			status := row.StatusCode
			if attempts >= 5 {
				status = "locked"
			}
			_, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_notify_security_code_verification
SET failed_attempts=?,status_code=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND verification_no=?`,
				attempts, status, notificationTenantID, input.VerificationNo)
			businessErr = bizerr.NewCode(CodeVerificationInvalid)
			return updateErr
		}
		result, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_notify_security_code_verification
SET status_code='verified',verification_version=1,verified_at=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND verification_no=? AND status_code='sent' AND failed_attempts=?`, now,
			notificationTenantID, input.VerificationNo, row.FailedAttempts)
		if updateErr != nil || requireOne(result) != nil {
			return firstChannelIntentError(updateErr, CodeConflict)
		}
		verified = &securitycodecap.Verified{VerificationNo: input.VerificationNo, Version: 1, VerifiedAt: now.UTC()}
		return nil
	})
	if err != nil {
		return nil, publicError(err)
	}
	if businessErr != nil {
		return nil, businessErr
	}
	if verified == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return verified, nil
}

func (s *securityCodeService) prepareBinding(ctx context.Context, input securitycodecap.Binding) (*securityCodeBinding, error) {
	if !stableCodePattern.MatchString(input.Purpose) || len(input.Purpose) > 64 ||
		!securityCodeSubjectPattern.MatchString(input.SubjectNo) || !browserNoncePattern.Match(input.BrowserNonce) || !validLocale(input.Locale) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	destination, err := normalizePresalesDestination("sms", input.Destination)
	if err != nil {
		return nil, err
	}
	digests, err := s.digests(ctx, map[string][]byte{"subject": []byte(input.SubjectNo),
		"destination": destination, "browser": input.BrowserNonce})
	if err != nil {
		clear(destination)
		return nil, err
	}
	challengePurpose := "notify.security_code." + input.Purpose
	challengeBinding := strings.Join([]string{s.callerPluginID, digests["subject"], digests["destination"], digests["browser"]}, ":")
	return &securityCodeBinding{purpose: input.Purpose, subjectNo: input.SubjectNo, destination: destination,
		locale: input.Locale, subjectDigest: digests["subject"], destinationDigest: digests["destination"],
		browserDigest: digests["browser"], challengePurpose: challengePurpose, challengeBinding: challengeBinding}, nil
}

func (s *securityCodeService) digests(ctx context.Context, values map[string][]byte) (map[string]string, error) {
	purpose, err := secretcap.NewPurpose("notify.verify")
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	request, err := secretcap.NewAccessRequest(s.verificationHandle, purpose)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	result := make(map[string]string, len(values))
	err = s.verificationAccessor.Access(ctx, request, func(plaintext []byte) error {
		for label, value := range values {
			mac := hmac.New(sha256.New, plaintext)
			_, _ = mac.Write([]byte("idc-notify-security-code-v1\x00" + s.callerPluginID + "\x00" + label + "\x00"))
			_, _ = mac.Write(value)
			result[label] = hex.EncodeToString(mac.Sum(nil))
		}
		return nil
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return result, nil
}

func (s *securityCodeService) loadPurpose(ctx context.Context, purpose string) (*securityCodePurposeRow, error) {
	var row securityCodePurposeRow
	if err := s.database.Model(securityCodePurposeTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("caller_plugin_id", s.callerPluginID).Where("purpose_code", purpose).Where("enabled", true).Scan(&row); err != nil {
		return nil, err
	}
	if row.PurposeCode == "" || row.EventCode == "" || row.Version <= 0 {
		return nil, nil
	}
	return &row, nil
}

func (s *securityCodeService) loadDeliveryPlan(ctx context.Context, eventCode, locale string, variables map[string]string) (*securityCodeDeliveryPlan, error) {
	var channel channelRow
	if err := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("channel_code", "sms").Scan(&channel); err != nil {
		return nil, publicError(err)
	}
	if channel.ChannelCode == "" || channel.ProviderCode == nil || channel.CredentialSecretRef == nil || ChannelStatus(channel.StatusCode) != ChannelEnabled {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	adapter, exists := s.adapters[*channel.ProviderCode]
	if !exists || adapter.descriptor.Kind != channelcap.KindSMS {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	var template templateRow
	if err := s.database.Model("plugin_idc_notify_template_version").Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("event_code", eventCode).Where("channel_code", "sms").
		Where("locale_code", locale).Where("status_code", "active").OrderDesc("version").Limit(1).Scan(&template); err != nil {
		return nil, publicError(err)
	}
	if template.TemplateNo == "" {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	if template.ProviderCode != *channel.ProviderCode {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	if _, _, _, err := materializeTestProviderMapping(&template, variables); err != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	configuration := make(map[string]string)
	if json.Unmarshal([]byte(channel.NonSecretConfiguration), &configuration) != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	handle, err := secretcap.NewHandle(*channel.CredentialSecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	return &securityCodeDeliveryPlan{channel: channel, template: &template, adapter: adapter,
		credential: handle, configuration: configuration}, nil
}

func (s *securityCodeService) deliverOnce(ctx context.Context, plan *securityCodeDeliveryPlan, operationNo, eventCode string,
	destination []byte, variables map[string]string) channelcap.Result {
	if plan == nil || plan.template == nil || plan.adapter.adapter.ValidateDestination(destination) != nil {
		return channelcap.Result{Class: channelcap.ResultConfigurationError}
	}
	subject, subjectErr := renderTemplate(plan.template.SubjectTemplate, variables)
	body, bodyErr := renderTemplate(plan.template.BodyTemplate, variables)
	providerTemplateID, providerParameters, providerData, mappingErr := materializeTestProviderMapping(plan.template, variables)
	if subjectErr != nil || bodyErr != nil || mappingErr != nil {
		return channelcap.Result{Class: channelcap.ResultConfigurationError}
	}
	result, err := plan.adapter.adapter.Deliver(ctx, channelcap.Delivery{Test: false, TaskNo: operationNo,
		IdempotencyKey: operationNo, EventNo: operationNo, EventType: eventCode, EventVersion: plan.template.Version,
		OccurredAt: s.now().UTC(), Credential: plan.credential, Destination: destination, Subject: subject, Body: body,
		Locale: plan.template.LocaleCode, ProviderTemplateID: providerTemplateID, ProviderParameters: providerParameters,
		ProviderData: providerData, NonSecretConfig: plan.configuration})
	if err != nil || !validResultClass(result.Class) {
		return channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true}
	}
	return result
}

func (s *securityCodeService) consumeRateLimit(ctx context.Context, limit rateLimit) error {
	unit := "minute"
	if limit.seconds == 3600 {
		unit = "hour"
	} else if limit.seconds == 86400 {
		unit = "day"
	}
	value, err := s.database.GetValue(ctx, fmt.Sprintf(`INSERT INTO %s
(tenant_id,dimension_code,subject_digest,window_seconds,window_started_at,counter,expires_at)
VALUES (?,?,?, ?,date_trunc('%s',CURRENT_TIMESTAMP),1,date_trunc('%s',CURRENT_TIMESTAMP)+INTERVAL '%d seconds')
ON CONFLICT (tenant_id,dimension_code,subject_digest,window_seconds,window_started_at)
DO UPDATE SET counter=%s.counter+1,updated_at=CURRENT_TIMESTAMP WHERE %s.counter < ? RETURNING counter`,
		rateWindowTable, unit, unit, limit.seconds, rateWindowTable, rateWindowTable), notificationTenantID,
		limit.dimension, limit.subject, limit.seconds, limit.limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bizerr.NewCode(CodeVerificationRateLimited)
		}
		return err
	}
	if value == nil || value.Int() <= 0 {
		return bizerr.NewCode(CodeVerificationRateLimited)
	}
	return nil
}

func (s *securityCodeService) loadByIdempotency(ctx context.Context, digest string) (*securityCodeVerificationRow, error) {
	var row securityCodeVerificationRow
	if err := s.database.Model(securityCodeVerificationTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("caller_plugin_id", s.callerPluginID).Where("idempotency_digest", digest).Scan(&row); err != nil {
		return nil, err
	}
	if row.VerificationNo == "" {
		return nil, nil
	}
	return &row, nil
}

func (s *securityCodeService) loadByNo(ctx context.Context, number string) (*securityCodeVerificationRow, error) {
	var row securityCodeVerificationRow
	if err := s.database.Model(securityCodeVerificationTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("caller_plugin_id", s.callerPluginID).Where("verification_no", number).Scan(&row); err != nil {
		return nil, err
	}
	if row.VerificationNo == "" {
		return nil, nil
	}
	return &row, nil
}

func replaySecurityCode(row *securityCodeVerificationRow, expectedShape string) (*securitycodecap.Verification, error) {
	if row == nil || row.RequestShapeDigest != expectedShape {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return projectSecurityCode(row), nil
}

func projectSecurityCode(row *securityCodeVerificationRow) *securitycodecap.Verification {
	state := row.DeliveryState
	if row.StatusCode == "prepared" {
		state = "result_unknown"
	}
	item := &securitycodecap.Verification{VerificationNo: row.VerificationNo,
		MaskedDestination: row.MaskedDestination, DeliveryState: state}
	if row.ExpiresAt != nil {
		item.ExpiresAt = row.ExpiresAt.UTC()
	}
	if row.RetryAfterAt != nil {
		item.RetryAfterAt = row.RetryAfterAt.UTC()
	}
	return item
}

func securityCodeMatches(row securityCodeVerificationRow, caller string, binding *securityCodeBinding) bool {
	return binding != nil && row.VerificationNo != "" && row.CallerPluginID == caller && row.PurposeCode == binding.purpose &&
		row.SubjectDigest == binding.subjectDigest && row.DestinationDigest == binding.destinationDigest &&
		row.BrowserDigest == binding.browserDigest && row.LocaleCode == binding.locale
}

func securityCodeTerminal(status string) bool {
	return status == "verified" || status == "locked" || status == "superseded"
}
