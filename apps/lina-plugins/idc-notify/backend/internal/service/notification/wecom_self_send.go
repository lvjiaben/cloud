// This file sends one administrator-confirmed WeCom test to the current
// operator's active self-service binding without accepting a raw UserID.

package notification

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const wecomSelfTestTargetMask = "current_staff_binding"

// TestOwnWeCom sends exactly once to the authorized operator's current active
// self-service binding. It never accepts or returns a WeCom UserID.
func (s *serviceImpl) TestOwnWeCom(ctx context.Context, input ChannelTestInput) (*ChannelTestResult, error) {
	defer clear(input.Target)
	if !validWeComSelfTestInput(input) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, "notification-channel-wechat_work")
	if err != nil {
		return nil, err
	}
	staff, _, _, err := s.currentWeComStaffSession(ctx, true)
	if err != nil || staff.HostUserID != actor {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	channel, configuration, err := s.loadWeComChannel(ctx)
	if err != nil || !validWeComSelfTestChannel(channel, input.ExpectedVersion) {
		return nil, errorOrCode(err, CodeConflict)
	}
	binding, err := s.loadActiveWeComBinding(ctx, staff.HostUserID)
	if err != nil || !validWeComSelfTestBinding(binding, staff, configuration["corp_id"]) {
		return nil, errorOrCode(err, CodeChannelUnavailable)
	}
	adapter, exists := s.adapters[*channel.ProviderCode]
	if !exists {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	template, err := s.loadTemplate(ctx, input.TemplateNo, input.TemplateVersion)
	if err != nil || !validWeComSelfTestTemplate(template, input.ChannelCode) {
		return nil, errorOrCode(err, CodeNotAccessible)
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
	credential, err := secretcap.NewHandle(*channel.CredentialSecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	variableJSON, err := jsonObject(input.Variables)
	if err != nil {
		return nil, err
	}
	requestDigest := digestText(input.ChannelCode, input.TemplateNo, int64Text(input.TemplateVersion),
		int64Text(input.ExpectedVersion), binding.IdentityDigest, int64Text(binding.Version), variableJSON)
	operationNo := stableNumber("ntest_", intText(actor), input.ChannelCode, input.IdempotencyKey)
	prepared, inserted, err := s.prepareChannelTest(ctx, channelTestRow{
		OperationNo: operationNo, ChannelCode: input.ChannelCode, ChannelVersion: input.ExpectedVersion,
		TemplateNo: input.TemplateNo, TemplateVersion: input.TemplateVersion, TargetMask: wecomSelfTestTargetMask,
		RequestDigest: requestDigest, ActorUserID: actor,
	})
	if err != nil {
		return nil, publicError(err)
	}
	if !inserted {
		if prepared.RequestDigest != requestDigest || prepared.ActorUserID != actor {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return projectChannelTest(prepared, true), nil
	}

	result := channelcap.Result{Class: channelcap.ResultConfigurationError}
	providerCalled := false
	accessErr := s.WithWeComDestination(ctx, actor, func(destination []byte) error {
		providerCalled = true
		providerResult, deliveryErr := adapter.adapter.Deliver(ctx, channelcap.Delivery{
			Test: true, TaskNo: operationNo, IdempotencyKey: operationNo,
			EventNo: operationNo, EventType: template.EventCode, EventVersion: template.Version,
			OccurredAt: s.now().UTC(), Credential: credential, Destination: destination,
			Subject: subject, Body: body, Locale: template.LocaleCode, NonSecretConfig: configuration,
		})
		if deliveryErr != nil || !validResultClass(providerResult.Class) {
			result = channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true}
			return nil
		}
		result = providerResult
		return nil
	})
	if accessErr != nil && providerCalled {
		result = channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true}
	}
	if err = s.completeWeComSelfTest(ctx, prepared, binding, result); err != nil {
		return nil, publicError(err)
	}
	prepared.StatusCode = "completed"
	prepared.ResultClass = string(result.Class)
	prepared.CrossedExternalBoundary = result.CrossedExternalBorder
	prepared.ProviderReference = stringPointer(result.ProviderMessageRef)
	completedAt := s.now().UTC()
	prepared.CompletedAt = &completedAt
	return projectChannelTest(prepared, false), nil
}

func validWeComSelfTestInput(input ChannelTestInput) bool {
	return input.ChannelCode == "wechat_work" && len(input.Target) == 0 &&
		stableCodePattern.MatchString(input.TemplateNo) && input.TemplateVersion > 0 && input.ExpectedVersion > 0 &&
		validateIdempotencyKey(input.IdempotencyKey) && input.Confirmed
}

func validWeComSelfTestChannel(channel *channelRow, expectedVersion int64) bool {
	return channel != nil && ChannelStatus(channel.StatusCode) == ChannelEnabled &&
		channel.ConfigurationVersion == expectedVersion && channel.CredentialSecretRef != nil && channel.ProviderCode != nil
}

func validWeComSelfTestBinding(
	binding *wecomBindingRow,
	staff notificationcap.CurrentStaffProjection,
	corpID string,
) bool {
	return binding != nil && binding.StaffVersion == staff.StaffVersion && binding.CorpID == corpID
}

func validWeComSelfTestTemplate(template *templateRow, channelCode string) bool {
	return template != nil && template.ChannelCode == channelCode && template.StatusCode == "active"
}

func (s *serviceImpl) completeWeComSelfTest(
	ctx context.Context,
	prepared channelTestRow,
	binding *wecomBindingRow,
	result channelcap.Result,
) error {
	if binding == nil {
		return bizerr.NewCode(CodeConflict)
	}
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
		_, err = s.database.Model(wecomBindingAuditTable).Ctx(ctx).Data(modeldo.WecomBindingAudit{
			TenantId: notificationTenantID, AuditNo: stableNumber("nwca_", binding.BindingNo, "test", prepared.OperationNo),
			BindingNo: binding.BindingNo, ActorUserId: prepared.ActorUserID, ActionCode: "test",
			ReasonCode: "employee_self_test", PreviousStatus: wecomBindingActive,
			CurrentStatus: wecomBindingActive, BindingVersion: binding.Version,
		}).InsertIgnore()
		if err != nil {
			return err
		}
		return s.management.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "notification.channel.test", IdempotencyKey: prepared.OperationNo,
			OperationCode: "notification.channel.test", TargetType: "notification_channel",
			TargetNo: prepared.ChannelCode, ResultCode: channelTestAuditResult(result.Class), ReasonCode: "employee_self_test", RiskCode: "high",
			RequiredPermission: PermissionChannelManage,
			BeforeSnapshot:     map[string]any{"channel_version": prepared.ChannelVersion, "target_mask": prepared.TargetMask},
			AfterSnapshot:      map[string]any{"result_class": string(result.Class), "crossed_external_boundary": result.CrossedExternalBorder},
		})
	})
}
