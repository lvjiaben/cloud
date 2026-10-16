package notification

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const channelTestTable = "plugin_idc_notify_channel_test_operation"

type channelTestRow struct {
	OperationNo             string     `orm:"operation_no"`
	ChannelCode             string     `orm:"channel_code"`
	ChannelVersion          int64      `orm:"channel_version"`
	TemplateNo              string     `orm:"template_no"`
	TemplateVersion         int64      `orm:"template_version"`
	TargetMask              string     `orm:"target_mask"`
	RequestDigest           string     `orm:"request_digest"`
	StatusCode              string     `orm:"status_code"`
	ResultClass             string     `orm:"result_class"`
	CrossedExternalBoundary bool       `orm:"crossed_external_boundary"`
	ProviderReference       *string    `orm:"provider_reference"`
	ActorUserID             int        `orm:"actor_user_id"`
	CompletedAt             *time.Time `orm:"completed_at"`
}

// TestChannel performs exactly one real provider call. The raw target and
// variable values are deliberately never written to the database or audit log.
// A prepared operation is never automatically replayed: after a crash its
// result is unknown, which is safer than delivering a duplicate paid message.
func (s *serviceImpl) TestChannel(ctx context.Context, input ChannelTestInput) (_ *ChannelTestResult, err error) {
	defer clear(input.Target)
	if !validChannelTestInput(input) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, "notification-channel-"+input.ChannelCode)
	if err != nil {
		return nil, err
	}
	if err = s.requireRecentLogin(ctx, actor); err != nil {
		return nil, err
	}

	var channel channelRow
	if err = s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("channel_code", input.ChannelCode).Scan(&channel); err != nil {
		return nil, publicError(err)
	}
	if channel.ChannelCode == "" || channel.ConfigurationVersion != input.ExpectedVersion ||
		channel.ProviderCode == nil || channel.CredentialSecretRef == nil ||
		ChannelStatus(channel.StatusCode) != ChannelEnabled {
		return nil, bizerr.NewCode(CodeConflict)
	}
	binding, exists := s.adapters[*channel.ProviderCode]
	if !exists || string(binding.descriptor.Kind) != input.ChannelCode {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	if err = binding.adapter.ValidateDestination(input.Target); err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	template, err := s.loadTemplate(ctx, input.TemplateNo, input.TemplateVersion)
	if err != nil || template == nil || template.ChannelCode != input.ChannelCode || template.StatusCode != "active" {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	if input.ChannelCode == "sms" && template.ProviderCode != *channel.ProviderCode {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	event, err := s.loadEvent(ctx, template.EventCode)
	if err != nil || event == nil {
		return nil, errorOrCode(err, CodeUnavailable)
	}
	if err = validateTestVariables(input.Variables, event); err != nil {
		return nil, err
	}
	subject, err := renderTemplate(template.SubjectTemplate, input.Variables)
	if err != nil {
		return nil, err
	}
	body, err := renderTemplate(template.BodyTemplate, input.Variables)
	if err != nil {
		return nil, err
	}
	providerTemplateID, providerParameters, providerData, err := materializeTestProviderMapping(template, input.Variables)
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
	variableJSON, err := jsonObject(input.Variables)
	if err != nil {
		return nil, err
	}
	targetDigest := digestText("notification-test-target", string(input.Target))
	requestDigest := digestText(input.ChannelCode, input.TemplateNo, int64Text(input.TemplateVersion),
		int64Text(input.ExpectedVersion), targetDigest, variableJSON)
	operationNo := stableNumber("ntest_", intText(actor), input.ChannelCode, input.IdempotencyKey)
	targetMask := maskTestTarget(input.ChannelCode, string(input.Target))
	row, inserted, err := s.prepareChannelTest(ctx, channelTestRow{
		OperationNo: operationNo, ChannelCode: input.ChannelCode, ChannelVersion: input.ExpectedVersion,
		TemplateNo: input.TemplateNo, TemplateVersion: input.TemplateVersion, TargetMask: targetMask,
		RequestDigest: requestDigest, ActorUserID: actor,
	})
	if err != nil {
		return nil, publicError(err)
	}
	if !inserted {
		if row.RequestDigest != requestDigest || row.ActorUserID != actor {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return projectChannelTest(row, true), nil
	}

	result, deliveryErr := binding.adapter.Deliver(ctx, channelcap.Delivery{
		Test: true, TaskNo: operationNo, IdempotencyKey: operationNo,
		EventNo: operationNo, EventType: template.EventCode, EventVersion: template.Version,
		OccurredAt: s.now().UTC(), Credential: handle, Destination: input.Target,
		Subject: subject, Body: body, Locale: template.LocaleCode,
		ProviderTemplateID: providerTemplateID, ProviderParameters: providerParameters,
		ProviderData: providerData, NonSecretConfig: configuration,
	})
	if deliveryErr != nil || !validResultClass(result.Class) {
		result = channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true}
	}
	if err = s.completeChannelTest(ctx, row, result); err != nil {
		return nil, publicError(err)
	}
	row.StatusCode = "completed"
	row.ResultClass = string(result.Class)
	row.CrossedExternalBoundary = result.CrossedExternalBorder
	row.ProviderReference = stringPointer(result.ProviderMessageRef)
	completedAt := s.now().UTC()
	row.CompletedAt = &completedAt
	return projectChannelTest(row, false), nil
}

func validChannelTestInput(input ChannelTestInput) bool {
	switch input.ChannelCode {
	case "email", "sms", "wechat_official":
	default:
		return false
	}
	return len(input.Target) > 0 && len(input.Target) <= 512 && stableCodePattern.MatchString(input.TemplateNo) &&
		input.TemplateVersion > 0 && input.ExpectedVersion > 0 && validateIdempotencyKey(input.IdempotencyKey) && input.Confirmed
}

func validateTestVariables(values map[string]string, event *eventRow) error {
	if len(values) > MaximumVariables {
		return bizerr.NewCode(CodeInvalid)
	}
	allowed := eventVariableSet(event)
	for key, value := range values {
		if _, exists := allowed[key]; !exists || len(value) > 4096 {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	var required []string
	if event == nil || json.Unmarshal([]byte(event.RequiredVariables), &required) != nil {
		return bizerr.NewCode(CodeInvalid)
	}
	for _, key := range required {
		if value, exists := values[key]; !exists || value == "" {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	return nil
}

func materializeTestProviderMapping(template *templateRow, variables map[string]string) (string, []string, map[string]string, error) {
	if template == nil {
		return "", nil, nil, bizerr.NewCode(CodeInvalid)
	}
	switch template.ChannelCode {
	case "sms":
		return materializeSMSProviderMapping(template, variables)
	case "wechat_official":
		mapping := make(map[string]string)
		if template.ProviderTemplateID == "" || json.Unmarshal([]byte(template.ProviderWeChatDataMapping), &mapping) != nil || len(mapping) == 0 || len(mapping) > MaximumVariables {
			return "", nil, nil, bizerr.NewCode(CodeInvalid)
		}
		data := make(map[string]string, len(mapping))
		for key, variable := range mapping {
			value, exists := variables[variable]
			if !providerDataKey(key) || !exists {
				return "", nil, nil, bizerr.NewCode(CodeInvalid)
			}
			data[key] = value
		}
		return template.ProviderTemplateID, nil, data, nil
	default:
		return "", nil, nil, nil
	}
}

func materializeSMSProviderMapping(template *templateRow, variables map[string]string) (string, []string, map[string]string, error) {
	if template == nil {
		return "", nil, nil, bizerr.NewCode(CodeInvalid)
	}
	var order []string
	mapping := make(map[string]string)
	if json.Unmarshal([]byte(template.ProviderSMSVariableOrder), &order) != nil || len(order) > MaximumVariables ||
		json.Unmarshal([]byte(template.ProviderSMSDataMapping), &mapping) != nil || len(mapping) > MaximumVariables {
		return "", nil, nil, bizerr.NewCode(CodeInvalid)
	}
	switch template.ProviderCode {
	case providerTencentSMS:
		if !numericTemplateID(template.ProviderTemplateID) || len(mapping) != 0 {
			return "", nil, nil, bizerr.NewCode(CodeInvalid)
		}
		parameters := make([]string, len(order))
		for index, variable := range order {
			value, exists := variables[variable]
			if !exists {
				return "", nil, nil, bizerr.NewCode(CodeInvalid)
			}
			parameters[index] = value
		}
		return template.ProviderTemplateID, parameters, nil, nil
	case providerAliyunSMS:
		if !providerTemplatePattern.MatchString(template.ProviderTemplateID) || len(order) != 0 {
			return "", nil, nil, bizerr.NewCode(CodeInvalid)
		}
		data := make(map[string]string, len(mapping))
		for key, variable := range mapping {
			value, exists := variables[variable]
			if !providerDataKey(key) || !exists {
				return "", nil, nil, bizerr.NewCode(CodeInvalid)
			}
			data[key] = value
		}
		return template.ProviderTemplateID, nil, data, nil
	case providerSMSBao:
		if template.ProviderTemplateID != "" || len(order) != 0 || len(mapping) != 0 {
			return "", nil, nil, bizerr.NewCode(CodeInvalid)
		}
		return "", nil, nil, nil
	default:
		return "", nil, nil, bizerr.NewCode(CodeChannelUnavailable)
	}
}

func (s *serviceImpl) prepareChannelTest(ctx context.Context, requested channelTestRow) (channelTestRow, bool, error) {
	result, err := s.database.Model(channelTestTable).Ctx(ctx).Data(modeldo.ChannelTestOperation{
		TenantId: notificationTenantID, OperationNo: requested.OperationNo,
		ChannelCode: requested.ChannelCode, ChannelVersion: requested.ChannelVersion,
		TemplateNo: requested.TemplateNo, TemplateVersion: requested.TemplateVersion,
		TargetMask: requested.TargetMask, RequestDigest: requested.RequestDigest,
		StatusCode: "prepared", ResultClass: string(channelcap.ResultUnknown),
		CrossedExternalBoundary: false, ActorUserId: requested.ActorUserID,
	}).InsertIgnore()
	if err != nil {
		return channelTestRow{}, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return channelTestRow{}, false, err
	}
	var persisted channelTestRow
	if err = s.database.Model(channelTestTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("operation_no", requested.OperationNo).Scan(&persisted); err != nil {
		return channelTestRow{}, false, err
	}
	if persisted.OperationNo == "" {
		return channelTestRow{}, false, bizerr.NewCode(CodeUnavailable)
	}
	return persisted, rows == 1, nil
}

func (s *serviceImpl) completeChannelTest(ctx context.Context, prepared channelTestRow, result channelcap.Result) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		updated, err := s.database.Model(channelTestTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("operation_no", prepared.OperationNo).
			Where("status_code", "prepared").Data(modeldo.ChannelTestOperation{
			StatusCode: "completed", ResultClass: string(result.Class),
			CrossedExternalBoundary: result.CrossedExternalBorder,
			ProviderReference:       nullableString(result.ProviderMessageRef),
			CompletedAt:             gdb.Raw("CURRENT_TIMESTAMP"),
		}).Update()
		if err != nil || requireOne(updated) != nil {
			return firstChannelIntentError(err, CodeConflict)
		}
		return s.management.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "notification.channel.test", IdempotencyKey: prepared.OperationNo,
			OperationCode: "notification.channel.test", TargetType: "notification_channel",
			TargetNo: prepared.ChannelCode, ResultCode: channelTestAuditResult(result.Class), ReasonCode: "merchant_test_send", RiskCode: "high",
			RequiredPermission: PermissionChannelManage,
			BeforeSnapshot:     map[string]any{"channel_version": prepared.ChannelVersion, "target_mask": prepared.TargetMask},
			AfterSnapshot:      map[string]any{"result_class": string(result.Class), "crossed_external_boundary": result.CrossedExternalBorder},
		})
	})
}

func projectChannelTest(row channelTestRow, reused bool) *ChannelTestResult {
	completed := time.Time{}
	resultClass := row.ResultClass
	if row.CompletedAt != nil {
		completed = row.CompletedAt.UTC()
	} else {
		resultClass = string(channelcap.ResultUnknown)
	}
	return &ChannelTestResult{OperationNo: row.OperationNo, ChannelCode: row.ChannelCode, TargetMask: row.TargetMask,
		ResultClass: resultClass, Reused: reused, CompletedAt: completed}
}

func maskTestTarget(channelCode, target string) string {
	target = strings.TrimSpace(target)
	if channelCode == "email" {
		parts := strings.Split(target, "@")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0][:1] + "***@" + parts[1]
		}
	}
	runes := []rune(target)
	if len(runes) <= 4 {
		return "***"
	}
	if len(runes) <= 8 {
		return string(runes[:2]) + "***" + string(runes[len(runes)-2:])
	}
	return string(runes[:3]) + "***" + string(runes[len(runes)-4:])
}

func validResultClass(value channelcap.ResultClass) bool {
	switch value {
	case channelcap.ResultAccepted, channelcap.ResultDelivered, channelcap.ResultDefinitiveNotReceivedTemporary,
		channelcap.ResultDefinitiveNotReceivedPermanent, channelcap.ResultInvalidRecipient,
		channelcap.ResultInsufficientBalance, channelcap.ResultConfigurationError, channelcap.ResultUnknown:
		return true
	default:
		return false
	}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intText(value int) string     { return strconv.Itoa(value) }
func int64Text(value int64) string { return strconv.FormatInt(value, 10) }

// channelTestAuditResult keeps delivery outcomes in snapshots while classifying
// the audit result using the sealed writer's finite result vocabulary.
func channelTestAuditResult(result channelcap.ResultClass) string {
	switch result {
	case channelcap.ResultAccepted, channelcap.ResultDelivered:
		return idccap.BusinessAuditResultSucceeded
	case channelcap.ResultUnknown:
		return "result_unknown"
	default:
		return "failed"
	}
}
