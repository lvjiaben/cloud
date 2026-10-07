package notification

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const wechatCallbackTable = "plugin_idc_notify_wechat_callback_event"

func (s *serviceImpl) VerifyWeChatCallback(ctx context.Context, input EncryptedCallbackInput) (_ string, err error) {
	defer clear(input.Encrypted)
	adapter, callbackInput, generation, err := s.wechatCallbackRuntime(ctx, input)
	if err != nil {
		return "", err
	}
	plaintext, err := adapter.VerifyChallenge(ctx, callbackInput)
	if err != nil {
		return "", bizerr.NewCode(CodeNotAccessible)
	}
	updated, err := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("channel_code", "wechat_official").Where("configuration_generation", generation).
		Data(modeldo.Channel{CallbackVerifiedGeneration: generation, CallbackVerifiedAt: gdb.Raw("CURRENT_TIMESTAMP")}).Update()
	if err != nil || requireOne(updated) != nil {
		return "", bizerr.NewCode(CodeConflict)
	}
	return plaintext, nil
}

func (s *serviceImpl) AcceptWeChatCallback(ctx context.Context, input EncryptedCallbackInput) (err error) {
	defer clear(input.Encrypted)
	adapter, callbackInput, _, err := s.wechatCallbackRuntime(ctx, input)
	if err != nil {
		return err
	}
	event, err := adapter.DecodeCallback(ctx, callbackInput)
	if err != nil || event.ProviderMessageRef == "" || !isDigest(event.EvidenceDigest) || !validCallbackResult(event.ResultClass) {
		return bizerr.NewCode(CodeNotAccessible)
	}
	callbackNo := stableNumber("nwccb_", event.EvidenceDigest)
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, insertErr := s.database.Model(wechatCallbackTable).Ctx(ctx).Data(modeldo.WechatCallbackEvent{
			TenantId: notificationTenantID, CallbackNo: callbackNo, ProviderMessageRef: event.ProviderMessageRef,
			EventDigest: event.EvidenceDigest, ResultClass: string(event.ResultClass),
		}).InsertIgnore()
		if insertErr != nil {
			return insertErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows == 0 {
			return rowsErr
		}
		var tasks []deliveryRow
		if queryErr := s.database.Model(deliveryTable+" t").Ctx(ctx).
			Fields("t.task_no,t.status_code,t.retry_sequence,t.version").
			Where("t.tenant_id", notificationTenantID).Where("t.channel_code", "wechat_official").
			Where("t.provider_message_ref", event.ProviderMessageRef).Limit(2).LockUpdate().Scan(&tasks); queryErr != nil {
			return queryErr
		}
		if len(tasks) != 1 {
			return nil
		}
		task := tasks[0]
		if task.StatusCode != string(DeliverySubmitted) && task.StatusCode != string(DeliveryResultUnknown) && task.StatusCode != string(DeliveryManualReview) {
			return nil
		}
		status := callbackDeliveryStatus(event.ResultClass, task.RetrySequence)
		data := modeldo.DeliveryTask{StatusCode: string(status), ResultClass: string(event.ResultClass),
			ProviderQueryDeadline: gdb.Raw("NULL"), ProviderQueryDeadlineSource: "wechat_encrypted_callback",
			Version: gdb.Raw("version + 1")}
		if status == DeliveryRetryWait {
			next := task.RetrySequence + 1
			data.RetrySequence = next
			data.NextAttemptAt = gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '" + retryIntervalLiteral(next) + "'")
		} else if isTerminalDeliveryStatus(status) {
			data.NextAttemptAt = gdb.Raw("NULL")
			data.CompletedAt = gdb.Raw("CURRENT_TIMESTAMP")
		}
		updated, updateErr := s.database.Model(deliveryTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
			Where("task_no", task.TaskNo).Where("version", task.Version).Data(data).Update()
		if updateErr != nil || requireOne(updated) != nil {
			return firstChannelIntentError(updateErr, CodeConflict)
		}
		if _, updateErr = s.database.Model(wechatCallbackTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
			Where("callback_no", callbackNo).Data(modeldo.WechatCallbackEvent{AppliedTaskNo: task.TaskNo}).Update(); updateErr != nil {
			return updateErr
		}
		return s.appendDeliveryAudit(ctx, task.TaskNo, "wechat_callback", "system", 0,
			"verified_encrypted_callback", task.StatusCode, string(status), task.Version+1)
	})
}

func (s *serviceImpl) wechatCallbackRuntime(ctx context.Context, input EncryptedCallbackInput) (channelcap.EncryptedCallbackAdapter, channelcap.CallbackInput, int64, error) {
	if input.Signature == "" || input.Timestamp == "" || input.Nonce == "" || len(input.Encrypted) == 0 || len(input.Encrypted) > 256*1024 {
		return nil, channelcap.CallbackInput{}, 0, bizerr.NewCode(CodeInvalid)
	}
	var channel channelRow
	if err := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("channel_code", "wechat_official").Scan(&channel); err != nil {
		return nil, channelcap.CallbackInput{}, 0, publicError(err)
	}
	if channel.ProviderCode == nil || channel.CredentialSecretRef == nil || channel.ConfigurationGeneration <= 0 {
		return nil, channelcap.CallbackInput{}, 0, bizerr.NewCode(CodeChannelUnavailable)
	}
	binding, exists := s.adapters[*channel.ProviderCode]
	if !exists {
		return nil, channelcap.CallbackInput{}, 0, bizerr.NewCode(CodeChannelUnavailable)
	}
	adapter, ok := binding.adapter.(channelcap.EncryptedCallbackAdapter)
	if !ok {
		return nil, channelcap.CallbackInput{}, 0, bizerr.NewCode(CodeChannelUnavailable)
	}
	handle, err := secretcap.NewHandle(*channel.CredentialSecretRef)
	if err != nil {
		return nil, channelcap.CallbackInput{}, 0, bizerr.NewCode(CodeChannelUnavailable)
	}
	configuration := make(map[string]string)
	if json.Unmarshal([]byte(channel.NonSecretConfiguration), &configuration) != nil {
		return nil, channelcap.CallbackInput{}, 0, bizerr.NewCode(CodeChannelUnavailable)
	}
	return adapter, channelcap.CallbackInput{Credential: handle, Configuration: configuration,
		Signature: input.Signature, Timestamp: input.Timestamp, Nonce: input.Nonce, Encrypted: input.Encrypted}, channel.ConfigurationGeneration, nil
}

func validCallbackResult(value channelcap.ResultClass) bool {
	switch value {
	case channelcap.ResultDelivered, channelcap.ResultDefinitiveNotReceivedTemporary,
		channelcap.ResultDefinitiveNotReceivedPermanent, channelcap.ResultInvalidRecipient, channelcap.ResultUnknown:
		return true
	default:
		return false
	}
}

func callbackDeliveryStatus(value channelcap.ResultClass, sequence int) DeliveryStatus {
	switch value {
	case channelcap.ResultDelivered:
		return DeliveryDelivered
	case channelcap.ResultDefinitiveNotReceivedTemporary:
		if sequence < len(retryDelays) {
			return DeliveryRetryWait
		}
		return DeliveryFailed
	case channelcap.ResultDefinitiveNotReceivedPermanent, channelcap.ResultInvalidRecipient:
		return DeliveryFailed
	default:
		return DeliveryManualReview
	}
}
