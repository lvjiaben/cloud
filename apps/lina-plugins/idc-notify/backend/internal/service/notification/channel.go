package notification

import (
	"context"
	"database/sql"
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

const channelTable = "plugin_idc_notify_channel"

type channelRow struct {
	ChannelCode                string     `orm:"channel_code"`
	ChannelKind                string     `orm:"channel_kind"`
	ProviderCode               *string    `orm:"provider_code"`
	DisplayName                string     `orm:"display_name"`
	StatusCode                 string     `orm:"status_code"`
	BalanceState               string     `orm:"balance_state"`
	ConfigurationGeneration    int64      `orm:"configuration_generation"`
	ConfigurationVersion       int64      `orm:"configuration_version"`
	CredentialSecretRef        *string    `orm:"credential_secret_ref"`
	CredentialMaskedTail       string     `orm:"credential_masked_tail"`
	CredentialUpdatedAt        *time.Time `orm:"credential_updated_at"`
	NonSecretConfiguration     string     `orm:"non_secret_configuration"`
	CallbackVerifiedGeneration int64      `orm:"callback_verified_generation"`
	CallbackVerifiedAt         *time.Time `orm:"callback_verified_at"`
	UpdatedAt                  time.Time  `orm:"updated_at"`
	PendingQueryCount          int        `orm:"pending_query_count"`
}

func (s *serviceImpl) ListChannels(ctx context.Context) ([]Channel, error) {
	if _, err := s.authorize(ctx, "notification-channels"); err != nil {
		return nil, err
	}
	var rows []channelRow
	err := s.database.Model(channelTable+" c").Ctx(ctx).
		Fields(`c.channel_code,c.channel_kind,c.provider_code,c.display_name,c.status_code,
            c.balance_state,c.configuration_generation,c.configuration_version,
			c.credential_secret_ref,c.credential_masked_tail,c.credential_updated_at,c.non_secret_configuration,
			c.callback_verified_generation,c.callback_verified_at,c.updated_at,
            (SELECT COUNT(*) FROM plugin_idc_notify_delivery_task t
             WHERE t.tenant_id=c.tenant_id AND t.channel_code=c.channel_code AND t.external_boundary_crossed=TRUE
               AND t.status_code IN ('submitted','result_unknown','manual_review')) AS pending_query_count`).
		Where("c.tenant_id", notificationTenantID).
		OrderAsc("c.channel_code").Scan(&rows)
	if err != nil {
		return nil, publicError(err)
	}
	items := make([]Channel, len(rows))
	for index := range rows {
		items[index] = projectChannel(rows[index])
	}
	return items, nil
}

func (s *serviceImpl) ConfigureChannel(ctx context.Context, input ChannelConfigurationInput) (_ *Channel, err error) {
	defer clearSecrets(input.Secrets)
	if err = validateChannelConfigurationInput(input); err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, "notification-channel-"+input.ChannelCode)
	if err != nil {
		return nil, err
	}
	if err = s.requireRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	binding, exists := s.adapters[input.ProviderCode]
	if !exists || string(binding.descriptor.Kind) != input.ChannelCode {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	secretCopy := make(map[string][]byte, len(input.Secrets))
	for key, value := range input.Secrets {
		secretCopy[key] = append([]byte(nil), value...)
	}
	defer clearSecrets(secretCopy)
	if err = validateAdapterConfiguration(binding.descriptor, input.NonSecret, secretCopy); err != nil {
		return nil, err
	}
	if err = binding.adapter.ValidateConfiguration(ctx, input.NonSecret, secretCopy); err != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	nonSecretJSON, err := jsonObject(input.NonSecret)
	if err != nil {
		return nil, err
	}
	secretPayload, err := json.Marshal(secretCopy)
	if err != nil || len(secretPayload) > secretcap.MaximumManagedPlaintextBytes {
		clear(secretPayload)
		return nil, bizerr.NewCode(CodeInvalid)
	}
	defer clear(secretPayload)

	intent, err := s.prepareConfigureIntent(ctx, actor, input, nonSecretJSON)
	if err != nil {
		return nil, publicError(err)
	}
	if err = s.executeConfigureIntent(ctx, intent, secretPayload); err != nil {
		return nil, publicError(err)
	}
	return s.getChannelAuthorized(ctx, input.ChannelCode)
}

func validateChannelConfigurationInput(input ChannelConfigurationInput) error {
	if !stableCodePattern.MatchString(input.ChannelCode) || !stableCodePattern.MatchString(input.ProviderCode) ||
		input.ExpectedVersion <= 0 || !validateIdempotencyKey(input.IdempotencyKey) || len(input.Secrets) == 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

func (s *serviceImpl) DisableChannel(ctx context.Context, input ChannelLifecycleInput) (*Channel, error) {
	return s.changeChannelStatus(ctx, input, ChannelDisabled)
}

func (s *serviceImpl) EnableChannel(ctx context.Context, input ChannelLifecycleInput) (*Channel, error) {
	return s.changeChannelStatus(ctx, input, ChannelEnabled)
}

func (s *serviceImpl) changeChannelStatus(ctx context.Context, input ChannelLifecycleInput, target ChannelStatus) (*Channel, error) {
	if !validLifecycleInput(input) || (target != ChannelEnabled && target != ChannelDisabled) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, "notification-channel-"+input.ChannelCode)
	if err != nil {
		return nil, err
	}
	if err = s.requireRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	var cancelled int64
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row channelRow
		if loadErr := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", input.ChannelCode).LockUpdate().Scan(&row); loadErr != nil {
			return loadErr
		}
		if row.ChannelCode == "" || row.ConfigurationVersion != input.ExpectedVersion ||
			row.CredentialSecretRef == nil && input.ChannelCode != "in_app" {
			return bizerr.NewCode(CodeConflict)
		}
		if ChannelStatus(row.StatusCode) == target {
			return nil
		}
		if target == ChannelDisabled {
			value, cancelErr := s.database.GetValue(ctx, `
WITH targets AS (
    SELECT tenant_id,task_no FROM plugin_idc_notify_delivery_task
    WHERE tenant_id=? AND channel_code=? AND status_code IN ('queued','retry_wait') AND external_boundary_crossed=FALSE
    ORDER BY queued_at,task_no FOR UPDATE SKIP LOCKED LIMIT 500
), cancelled AS (
    UPDATE plugin_idc_notify_delivery_task t
    SET status_code='cancelled',result_class='channel_disabled',completed_at=CURRENT_TIMESTAMP,
        updated_at=CURRENT_TIMESTAMP,version=version+1
    FROM targets WHERE t.tenant_id=targets.tenant_id AND t.task_no=targets.task_no RETURNING 1
)
SELECT count(*) FROM cancelled`, notificationTenantID, input.ChannelCode)
			if cancelErr != nil {
				return cancelErr
			}
			cancelled = int64(value.Int())
		}
		if target == ChannelEnabled {
			remaining, countErr := s.database.Model(deliveryTable).Ctx(ctx).
				Where(tenantIDColumn, notificationTenantID).Where("channel_code", input.ChannelCode).
				WhereIn("status_code", []string{string(DeliveryQueued), string(DeliveryRetryWait)}).
				Where("external_boundary_crossed", false).Count()
			if countErr != nil {
				return countErr
			}
			if remaining > 0 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		if target == ChannelEnabled && input.ChannelCode == "wechat_official" &&
			(row.CallbackVerifiedAt == nil || row.CallbackVerifiedGeneration != row.ConfigurationGeneration) {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		result, updateErr := s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("channel_code", input.ChannelCode).
			Where("configuration_version", input.ExpectedVersion).
			Data(modeldo.Channel{StatusCode: string(target), DisabledAt: disabledTime(target), ConfigurationVersion: gdb.Raw("configuration_version + 1")}).Update()
		if updateErr != nil || requireOne(result) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		return s.appendChannelAudit(ctx, actor, input.ChannelCode, string(target), "merchant_lifecycle", input.ExpectedVersion, int(cancelled), input.IdempotencyKey)
	})
	if err != nil {
		return nil, publicError(err)
	}
	return s.getChannelAuthorized(ctx, input.ChannelCode)
}

func (s *serviceImpl) ClearChannel(ctx context.Context, input ChannelLifecycleInput) (*Channel, error) {
	if !validLifecycleInput(input) || input.ChannelCode == "in_app" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, "notification-channel-"+input.ChannelCode)
	if err != nil {
		return nil, err
	}
	if err = s.requireRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	intent, err := s.prepareClearIntent(ctx, actor, input)
	if err != nil {
		return nil, publicError(err)
	}
	if err = s.executeClearIntent(ctx, intent); err != nil {
		return nil, publicError(err)
	}
	return s.getChannelAuthorized(ctx, input.ChannelCode)
}

func (s *serviceImpl) getChannelAuthorized(ctx context.Context, code string) (*Channel, error) {
	channels, err := s.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	for index := range channels {
		if channels[index].Code == code {
			return &channels[index], nil
		}
	}
	return nil, bizerr.NewCode(CodeNotAccessible)
}

func (s *serviceImpl) appendChannelAudit(ctx context.Context, actor int, code, operation, reason string, before int64, affected int, operationKey string) error {
	operationNo := stableNumber("nop_", code, operation, operationKey)
	if _, err := s.database.Model("plugin_idc_notify_channel_operation").Ctx(ctx).Data(modeldo.ChannelOperation{
		TenantId: notificationTenantID, OperationNo: operationNo,
		ChannelCode: code, OperationCode: operation,
		ActorUserId: actor, ReasonCode: reason, BeforeVersion: before,
		AfterVersion: before + 1, AffectedTaskCount: affected,
	}).Insert(); err != nil {
		return err
	}
	return s.management.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "notification.channel." + operation, IdempotencyKey: operationKey,
		OperationCode: "notification.channel." + operation, TargetType: "notification_channel",
		TargetNo: code, ResultCode: "succeeded", ReasonCode: reason, RiskCode: "high",
		RequiredPermission: PermissionChannelManage,
		BeforeSnapshot:     map[string]any{"version": before}, AfterSnapshot: map[string]any{"version": before + 1, "status": operation},
	})
}

func projectChannel(row channelRow) Channel {
	provider := ""
	if row.ProviderCode != nil {
		provider = *row.ProviderCode
	}
	item := Channel{
		Code: row.ChannelCode, Kind: row.ChannelKind, ProviderCode: provider, DisplayName: row.DisplayName,
		Status: ChannelStatus(row.StatusCode), BalanceState: row.BalanceState,
		Configured:           row.CredentialSecretRef != nil || row.ChannelCode == "in_app",
		CredentialMaskedTail: row.CredentialMaskedTail, ConfigurationGeneration: row.ConfigurationGeneration,
		PendingQueryCount: row.PendingQueryCount, Version: row.ConfigurationVersion, UpdatedAt: row.UpdatedAt.UTC(),
	}
	var values map[string]string
	if json.Unmarshal([]byte(row.NonSecretConfiguration), &values) == nil {
		switch row.ChannelCode {
		case "email":
			port, _ := strconv.Atoi(values["port"])
			item.Configuration.SMTP = &SMTPPublicConfiguration{Host: values["host"], Port: port, Security: values["security"], Username: values["username"], FromAddress: values["from_address"], FromName: values["from_name"]}
		case "sms":
			switch provider {
			case providerAliyunSMS:
				item.Configuration.AliyunSMS = &AliyunSMSPublicConfiguration{SignName: values["sign_name"]}
			case providerSMSBao:
				item.Configuration.SMSBao = &SMSBaoPublicConfiguration{SignName: values["sign_name"], ProductID: values["product_id"]}
			case providerTencentSMS:
				item.Configuration.TencentSMS = &TencentSMSPublicConfiguration{Region: values["region"], SmsSdkAppID: values["sms_sdk_app_id"], SignName: values["sign_name"]}
			}
		case "wechat_official":
			item.Configuration.WeChatOfficial = &WeChatOfficialPublicConfiguration{AppID: values["app_id"],
				CallbackVerified:   row.CallbackVerifiedAt != nil && row.CallbackVerifiedGeneration == row.ConfigurationGeneration,
				CallbackVerifiedAt: utcTimePointer(row.CallbackVerifiedAt)}
		case "wechat_work":
			agentID, _ := strconv.ParseInt(values["agent_id"], 10, 64)
			item.Configuration.WeCom = &WeComPublicConfiguration{CorpID: values["corp_id"], AgentID: agentID}
		case "signed_webhook":
			item.Configuration.SignedWebhook = &SignedWebhookPublicConfiguration{EndpointURL: values["endpoint_url"]}
		}
	}
	return item
}

func validateAdapterConfiguration(descriptor channelcap.Descriptor, nonSecret map[string]string, secrets map[string][]byte) error {
	knownNonSecret, knownSecret := map[string]channelcap.Field{}, map[string]channelcap.Field{}
	for _, field := range descriptor.Fields {
		if field.Secret {
			knownSecret[field.Key] = field
		} else {
			knownNonSecret[field.Key] = field
		}
	}
	for key, value := range nonSecret {
		field, ok := knownNonSecret[key]
		if !ok || len(value) > field.MaxLength {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	for key, value := range secrets {
		field, ok := knownSecret[key]
		if !ok || len(value) == 0 || len(value) > field.MaxLength {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	for _, field := range descriptor.Fields {
		if !field.Required {
			continue
		}
		if field.Secret && len(secrets[field.Key]) == 0 || !field.Secret && strings.TrimSpace(nonSecret[field.Key]) == "" {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	return nil
}

func validLifecycleInput(input ChannelLifecycleInput) bool {
	return stableCodePattern.MatchString(input.ChannelCode) && input.ExpectedVersion > 0 &&
		validateIdempotencyKey(input.IdempotencyKey) && input.Confirmed
}

func disabledTime(status ChannelStatus) any {
	if status == ChannelDisabled {
		return gdb.Raw("CURRENT_TIMESTAMP")
	}
	return gdb.Raw("NULL")
}

func requireOne(result sql.Result) error {
	if result == nil {
		return bizerr.NewCode(CodeConflict)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}
