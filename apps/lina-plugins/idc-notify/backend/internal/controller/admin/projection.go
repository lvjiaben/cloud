package admin

import (
	"time"

	v1 "lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func channelItem(item notification.Channel) *v1.ChannelItem {
	projection := &v1.ChannelItem{Code: item.Code, Kind: item.Kind, ProviderCode: item.ProviderCode,
		DisplayName: item.DisplayName, Status: string(item.Status), BalanceState: item.BalanceState,
		Configured: item.Configured, CredentialMaskedTail: item.CredentialMaskedTail,
		ConfigurationGeneration: item.ConfigurationGeneration, PendingQueryCount: item.PendingQueryCount,
		Version: item.Version, UpdatedAt: item.UpdatedAt.UnixMilli()}
	if item.Configuration.SMTP != nil {
		value := item.Configuration.SMTP
		projection.Configuration.SMTP = &v1.SMTPPublicConfigurationItem{Host: value.Host, Port: value.Port, Security: value.Security, Username: value.Username, FromAddress: value.FromAddress, FromName: value.FromName}
	}
	if item.Configuration.AliyunSMS != nil {
		projection.Configuration.AliyunSMS = &v1.AliyunSMSPublicConfigurationItem{SignName: item.Configuration.AliyunSMS.SignName}
	}
	if item.Configuration.SMSBao != nil {
		projection.Configuration.SMSBao = &v1.SMSBaoPublicConfigurationItem{SignName: item.Configuration.SMSBao.SignName, ProductID: item.Configuration.SMSBao.ProductID}
	}
	if item.Configuration.TencentSMS != nil {
		value := item.Configuration.TencentSMS
		projection.Configuration.TencentSMS = &v1.TencentSMSPublicConfigurationItem{Region: value.Region, SmsSdkAppID: value.SmsSdkAppID, SignName: value.SignName}
	}
	if item.Configuration.WeChatOfficial != nil {
		projection.Configuration.WeChatOfficial = &v1.WeChatOfficialPublicConfigurationItem{AppID: item.Configuration.WeChatOfficial.AppID,
			CallbackVerified: item.Configuration.WeChatOfficial.CallbackVerified, CallbackVerifiedAt: unixMilli(item.Configuration.WeChatOfficial.CallbackVerifiedAt)}
	}
	if item.Configuration.WeCom != nil {
		projection.Configuration.WeCom = &v1.WeComPublicConfigurationItem{CorpID: item.Configuration.WeCom.CorpID, AgentID: item.Configuration.WeCom.AgentID}
	}
	if item.Configuration.SignedWebhook != nil {
		projection.Configuration.SignedWebhook = &v1.SignedWebhookPublicConfigurationItem{EndpointURL: item.Configuration.SignedWebhook.EndpointURL}
	}
	return projection
}

func eventItem(item notification.Event) *v1.EventItem {
	return &v1.EventItem{Code: item.Code, Topic: item.Topic, ObjectType: item.ObjectType,
		RequiredVariables: append([]string(nil), item.RequiredVariables...), OptionalVariables: append([]string(nil), item.OptionalVariables...),
		SecurityRequired: item.SecurityRequired, Version: item.Version}
}

func templateItem(item notification.Template) *v1.TemplateItem {
	projection := &v1.TemplateItem{TemplateNo: item.TemplateNo, Version: item.Version, EventCode: item.EventCode,
		ChannelCode: item.ChannelCode, Locale: item.Locale, Subject: item.Subject, Body: item.Body,
		Status: item.Status, CreatedBy: int64(item.CreatedBy), CreatedAt: item.CreatedAt.UnixMilli()}
	if item.SMS != nil {
		mapping := make(map[string]string, len(item.SMS.DataMapping))
		for key, value := range item.SMS.DataMapping {
			mapping[key] = value
		}
		projection.SMS = &v1.SMSTemplateMappingItem{ProviderCode: item.SMS.ProviderCode, TemplateID: item.SMS.TemplateID,
			VariableOrder: append([]string(nil), item.SMS.VariableOrder...), DataMapping: mapping}
	}
	if item.WeChatOfficial != nil {
		mapping := make(map[string]string, len(item.WeChatOfficial.DataMapping))
		for key, value := range item.WeChatOfficial.DataMapping {
			mapping[key] = value
		}
		projection.WeChatOfficial = &v1.WeChatOfficialTemplateMappingItem{TemplateID: item.WeChatOfficial.TemplateID, DataMapping: mapping}
	}
	return projection
}

func channelTestItem(item *notification.ChannelTestResult) *v1.ChannelTestItem {
	if item == nil {
		return nil
	}
	completedAt := int64(0)
	if !item.CompletedAt.IsZero() {
		completedAt = item.CompletedAt.UnixMilli()
	}
	return &v1.ChannelTestItem{OperationNo: item.OperationNo, ChannelCode: item.ChannelCode, TargetMask: item.TargetMask,
		ResultClass: item.ResultClass, Reused: item.Reused, CompletedAt: completedAt}
}

func deliveryItem(item notification.Delivery) *v1.DeliveryItem {
	return &v1.DeliveryItem{TaskNo: item.TaskNo, ChainNo: item.ChainNo, PreviousTaskNo: item.PreviousTaskNo,
		EventCode: item.EventCode, Topic: item.Topic, ChannelCode: item.ChannelCode, Locale: item.Locale,
		RecipientMask: item.RecipientMask, BusinessObjectType: item.BusinessObjectType, BusinessObjectNo: item.BusinessObjectNo,
		Status: string(item.Status), ResultClass: item.ResultClass, RetrySequence: item.RetrySequence,
		NextAttemptAt: unixMilli(item.NextAttemptAt), QueryDeadline: unixMilli(item.QueryDeadline),
		QueryDeadlineSource: item.QueryDeadlineSource, ManualResendCount: item.ManualResendCount,
		IsChainTail: item.IsChainTail, Version: item.Version, QueuedAt: item.QueuedAt.UnixMilli(), CompletedAt: unixMilli(item.CompletedAt)}
}

func eligibilityItem(item *notification.ResendEligibility) *v1.ResendEligibilityItem {
	if item == nil {
		return nil
	}
	reasons := make([]string, len(item.Reasons))
	for index := range item.Reasons {
		reasons[index] = string(item.Reasons[index])
	}
	return &v1.ResendEligibilityItem{TaskNo: item.TaskNo, TaskVersion: item.TaskVersion, Allowed: item.Allowed,
		Reasons: reasons, EvaluatedAt: item.EvaluatedAt.UnixMilli(), NextAllowedAt: unixMilli(item.NextAllowedAt),
		RemainingCount: item.RemainingCount, ReplacedByTaskNo: item.ReplacedByTaskNo}
}

func unixMilli(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	result := value.UnixMilli()
	return &result
}
