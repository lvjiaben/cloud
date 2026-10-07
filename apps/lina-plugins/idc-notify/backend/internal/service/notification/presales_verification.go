package notification

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-idc-notify/backend/internal/model/entity"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

const (
	contactVerificationTable  = "plugin_idc_notify_contact_verification"
	rateWindowTable           = "plugin_idc_notify_rate_window"
	presalesOperationTable    = "plugin_idc_notify_presales_operation"
	presalesVerificationEvent = "support.presales_verification_code"
)

type presalesDeliveryPlan struct {
	channel            channelRow
	template           *templateRow
	adapter            adapterBinding
	credential         secretcap.Handle
	configuration      map[string]string
	providerTemplateID string
	providerParameters []string
}

func (s *serviceImpl) SendPresalesVerificationCode(ctx context.Context, input PresalesVerificationInput) (*PresalesVerification, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.ClientIP)
	if !validateIdempotencyKey(input.IdempotencyKey) || input.ProofToken == "" || len(input.ProofToken) > 4096 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.preparePresalesBinding(ctx, input.PresalesBindingInput)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	ip, err := normalizeClientIP(input.ClientIP)
	if err != nil {
		return nil, err
	}
	defer clear(ip)
	digests, err := s.hmacDigests(ctx, map[string][]byte{
		"idempotency": []byte(input.IdempotencyKey), "ip": ip,
	})
	if err != nil {
		return nil, err
	}
	requestShape := digestText(binding.operation, binding.destinationKind, binding.destinationDigest,
		binding.browserDigest, binding.ticketDigest, binding.locale)
	if existing, loadErr := s.loadVerificationByIdempotency(ctx, digests["idempotency"]); loadErr != nil {
		return nil, publicError(loadErr)
	} else if existing != nil {
		return replayVerification(existing, requestShape)
	}
	plan, err := s.loadPresalesDeliveryPlan(ctx, binding.destinationKind, presalesVerificationEvent, binding.locale,
		map[string]string{"code": "000000", "expires_minutes": "10"})
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
	rateSubjects, err := s.hmacDigests(ctx, map[string][]byte{
		"rate_destination": binding.destination, "rate_browser": input.BrowserNonce,
		"rate_ip": ip, "rate_template": []byte(plan.template.TemplateNo + ":" + strconv.FormatInt(plan.template.Version, 10)),
		"rate_sms_global": []byte("installation"),
	})
	if err != nil {
		return nil, err
	}
	var prepared *entity.ContactVerification
	createdHere := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.GetValue(txCtx,
			"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", presalesChallengePurpose+":"+binding.destinationDigest+":"+binding.browserDigest); lockErr != nil {
			return lockErr
		}
		if existing, loadErr := s.loadVerificationByIdempotency(txCtx, digests["idempotency"]); loadErr != nil {
			return loadErr
		} else if existing != nil {
			prepared = existing
			return nil
		}
		tooSoon, limitErr := s.database.GetValue(txCtx, `SELECT EXISTS(
SELECT 1 FROM plugin_idc_notify_contact_verification
WHERE tenant_id=? AND purpose_code=? AND destination_digest=? AND browser_digest=?
  AND status_code IN ('prepared','sent','result_unknown','delivery_failed') AND retry_after_at>CURRENT_TIMESTAMP
)`, notificationTenantID, presalesChallengePurpose, binding.destinationDigest, binding.browserDigest)
		if limitErr != nil {
			return limitErr
		}
		if tooSoon.Bool() {
			return bizerr.NewCode(CodeVerificationRateLimited)
		}
		if _, consumeErr := s.challenges.Consume(txCtx, challengecap.ConsumeRequest{
			Token: input.ProofToken, Purpose: presalesChallengePurpose, Binding: binding.challengeBinding,
		}); consumeErr != nil {
			return bizerr.WrapCode(consumeErr, CodeVerificationInvalid)
		}
		limits := []rateLimit{
			{"destination", rateSubjects["rate_destination"], 3600, 5}, {"destination", rateSubjects["rate_destination"], 86400, 10},
			{"browser", rateSubjects["rate_browser"], 3600, 10}, {"browser", rateSubjects["rate_browser"], 86400, 20},
			{"ip", rateSubjects["rate_ip"], 3600, 20}, {"ip", rateSubjects["rate_ip"], 86400, 100},
			{"template", rateSubjects["rate_template"], 60, 60}, {"template", rateSubjects["rate_template"], 3600, 500},
		}
		if binding.destinationKind == "sms" {
			limits = append(limits, rateLimit{"sms_global", rateSubjects["rate_sms_global"], 86400, s.runtimeConfig.SMSVerificationDailyLimit})
		}
		for _, limit := range limits {
			if limitErr := s.consumeRateLimit(txCtx, limit); limitErr != nil {
				return limitErr
			}
		}
		if _, updateErr := s.database.Model(contactVerificationTable).Ctx(txCtx).
			Where(modeldo.ContactVerification{TenantId: notificationTenantID, PurposeCode: presalesChallengePurpose,
				DestinationDigest: binding.destinationDigest, BrowserDigest: binding.browserDigest}).
			WhereIn("status_code", []string{"prepared", "sent", "result_unknown", "delivery_failed"}).
			Data(modeldo.ContactVerification{StatusCode: "superseded"}).Update(); updateErr != nil {
			return updateErr
		}
		_, insertErr := s.database.Exec(txCtx, `
INSERT INTO plugin_idc_notify_contact_verification
 (tenant_id,verification_no,purpose_code,operation_code,destination_kind,destination_digest,browser_digest,ticket_digest,
  locale_code,template_no,template_version,request_shape_digest,idempotency_digest,code_digest,masked_destination,
  status_code,delivery_state,failed_attempts,retry_after_at,expires_at,provider_boundary_crossed)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'prepared','prepared',0,CURRENT_TIMESTAMP + INTERVAL '60 seconds',
        CURRENT_TIMESTAMP + INTERVAL '10 minutes',FALSE)`,
			notificationTenantID, verificationNo, presalesChallengePurpose, binding.operation, binding.destinationKind,
			binding.destinationDigest, binding.browserDigest, binding.ticketDigest, binding.locale, plan.template.TemplateNo,
			plan.template.Version, requestShape, digests["idempotency"], codeDigest["code"],
			maskTestTarget(binding.destinationKind, string(binding.destination)))
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
	variables := map[string]string{"code": string(code), "expires_minutes": "10"}
	result := s.deliverPresalesOnce(ctx, plan, verificationNo, presalesVerificationEvent, binding.destination, variables)
	state, status := providerDeliveryState(result)
	_, updateErr := s.database.Model(contactVerificationTable).Ctx(ctx).
		Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: verificationNo, StatusCode: "prepared"}).
		Data(modeldo.ContactVerification{StatusCode: status, DeliveryState: state,
			ProviderBoundaryCrossed: result.CrossedExternalBorder}).Update()
	if updateErr != nil {
		return nil, publicError(updateErr)
	}
	prepared.StatusCode, prepared.DeliveryState = status, state
	prepared.ProviderBoundaryCrossed = result.CrossedExternalBorder
	return projectVerification(prepared), nil
}

type rateLimit struct {
	dimension string
	subject   string
	seconds   int
	limit     int
}

func (s *serviceImpl) consumeRateLimit(ctx context.Context, limit rateLimit) error {
	unit := "minute"
	if limit.seconds == 3600 {
		unit = "hour"
	} else if limit.seconds == 86400 {
		unit = "day"
	}
	value, err := s.database.GetValue(ctx, fmt.Sprintf(`
INSERT INTO %s
 (tenant_id,dimension_code,subject_digest,window_seconds,window_started_at,counter,expires_at)
VALUES (?,?,?, ?,date_trunc('%s',CURRENT_TIMESTAMP),1,date_trunc('%s',CURRENT_TIMESTAMP)+INTERVAL '%d seconds')
ON CONFLICT (tenant_id,dimension_code,subject_digest,window_seconds,window_started_at)
DO UPDATE SET counter=%s.counter+1,updated_at=CURRENT_TIMESTAMP
WHERE %s.counter < ?
RETURNING counter`, rateWindowTable, unit, unit, limit.seconds, rateWindowTable, rateWindowTable),
		notificationTenantID, limit.dimension, limit.subject, limit.seconds, limit.limit)
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

func (s *serviceImpl) loadPresalesDeliveryPlan(ctx context.Context, channelCode, eventCode, locale string, variables map[string]string) (*presalesDeliveryPlan, error) {
	var channel channelRow
	if err := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("channel_code", channelCode).Scan(&channel); err != nil {
		return nil, publicError(err)
	}
	if channel.ChannelCode == "" || channel.ProviderCode == nil || channel.CredentialSecretRef == nil || ChannelStatus(channel.StatusCode) != ChannelEnabled {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	adapter, exists := s.adapters[*channel.ProviderCode]
	if !exists || string(adapter.descriptor.Kind) != channelCode {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	var template templateRow
	if err := s.database.Model("plugin_idc_notify_template_version").Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("event_code", eventCode).
		Where("channel_code", channelCode).Where("locale_code", locale).Where("status_code", "active").
		OrderDesc("version").Limit(1).Scan(&template); err != nil {
		return nil, publicError(err)
	}
	if template.TemplateNo == "" {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	providerTemplateID, providerParameters, _, err := materializeTestProviderMapping(&template, variables)
	if err != nil {
		return nil, err
	}
	configuration := make(map[string]string)
	if json.Unmarshal([]byte(channel.NonSecretConfiguration), &configuration) != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	handle, err := secretcap.NewHandle(*channel.CredentialSecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	return &presalesDeliveryPlan{channel: channel, template: &template, adapter: adapter, credential: handle,
		configuration: configuration, providerTemplateID: providerTemplateID, providerParameters: providerParameters}, nil
}

func (s *serviceImpl) deliverPresalesOnce(ctx context.Context, plan *presalesDeliveryPlan, operationNo, eventCode string, destination []byte, variables map[string]string) channelcap.Result {
	if plan == nil || plan.template == nil {
		return channelcap.Result{Class: channelcap.ResultUnknown}
	}
	if err := plan.adapter.adapter.ValidateDestination(destination); err != nil {
		return channelcap.Result{Class: channelcap.ResultConfigurationError}
	}
	subject, subjectErr := renderTemplate(plan.template.SubjectTemplate, variables)
	body, bodyErr := renderTemplate(plan.template.BodyTemplate, variables)
	if subjectErr != nil || bodyErr != nil {
		return channelcap.Result{Class: channelcap.ResultConfigurationError}
	}
	providerTemplateID, providerParameters, providerData, mappingErr := materializeTestProviderMapping(plan.template, variables)
	if mappingErr != nil {
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

func providerDeliveryState(result channelcap.Result) (string, string) {
	switch result.Class {
	case channelcap.ResultAccepted:
		return "accepted", "sent"
	case channelcap.ResultDelivered:
		return "delivered", "sent"
	case channelcap.ResultUnknown:
		return "result_unknown", "result_unknown"
	default:
		return "failed", "delivery_failed"
	}
}

func randomSixDigitCode() ([]byte, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("%06d", value.Int64())), nil
}

func (s *serviceImpl) loadVerificationByIdempotency(ctx context.Context, digest string) (*entity.ContactVerification, error) {
	var row entity.ContactVerification
	if err := s.database.Model(contactVerificationTable).Ctx(ctx).
		Where(modeldo.ContactVerification{TenantId: notificationTenantID, IdempotencyDigest: digest}).Scan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if row.VerificationNo == "" {
		return nil, nil
	}
	return &row, nil
}

func (s *serviceImpl) loadVerificationByNo(ctx context.Context, number string) (*entity.ContactVerification, error) {
	var row entity.ContactVerification
	if err := s.database.Model(contactVerificationTable).Ctx(ctx).
		Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: number}).Scan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if row.VerificationNo == "" {
		return nil, nil
	}
	return &row, nil
}

func replayVerification(row *entity.ContactVerification, expectedShape string) (*PresalesVerification, error) {
	if row == nil || row.RequestShapeDigest != expectedShape {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return projectVerification(row), nil
}

func projectVerification(row *entity.ContactVerification) *PresalesVerification {
	state := row.DeliveryState
	if row.StatusCode == "prepared" {
		state = "result_unknown"
	}
	item := &PresalesVerification{VerificationNo: row.VerificationNo, MaskedDestination: row.MaskedDestination,
		DeliveryState: state}
	if row.ExpiresAt != nil {
		item.ExpiresAt = row.ExpiresAt.UTC()
	}
	if row.RetryAfterAt != nil {
		item.RetryAfterAt = row.RetryAfterAt.UTC()
	}
	return item
}
