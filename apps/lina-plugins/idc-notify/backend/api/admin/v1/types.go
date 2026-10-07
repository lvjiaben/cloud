package v1

type ChannelItem struct {
	Code                    string                         `json:"code" dc:"Stable notification channel code" eg:"email"`
	Kind                    string                         `json:"kind" dc:"Adapter kind: in_app, email, sms, wechat_official, wechat_work, signed_webhook, dingtalk, or feishu" eg:"email"`
	ProviderCode            string                         `json:"providerCode" dc:"Installed provider adapter code; empty when no adapter is configured" eg:"provider_code"`
	DisplayName             string                         `json:"displayName" dc:"Merchant-visible channel display name" eg:"Email"`
	Status                  string                         `json:"status" dc:"Channel status: unconfigured, configuring, enabled, or disabled" eg:"enabled"`
	BalanceState            string                         `json:"balanceState" dc:"Normalized provider balance state: unknown, sufficient, or insufficient" eg:"sufficient"`
	Configured              bool                           `json:"configured" dc:"Whether the authoritative secret service currently holds a configured credential" eg:"true"`
	CredentialMaskedTail    string                         `json:"credentialMaskedTail" dc:"Non-secret masked credential tail for administrator recognition" eg:"****1234"`
	ConfigurationGeneration int64                          `json:"configurationGeneration" dc:"Monotonic generation incremented when provider configuration changes" eg:"2"`
	PendingQueryCount       int                            `json:"pendingQueryCount" dc:"Number of submitted or unresolved deliveries still requiring provider-result handling" eg:"0"`
	Version                 int64                          `json:"version" dc:"Channel optimistic configuration version" eg:"3"`
	UpdatedAt               int64                          `json:"updatedAt" dc:"Channel update time as Unix timestamp in milliseconds" eg:"1786752000000"`
	Configuration           ChannelPublicConfigurationItem `json:"configuration" dc:"Typed non-secret provider configuration; credentials are never returned" eg:"{}"`
}

type ChannelPublicConfigurationItem struct {
	SMTP           *SMTPPublicConfigurationItem           `json:"smtp,omitempty" dc:"SMTP public settings when email is configured" eg:"{}"`
	AliyunSMS      *AliyunSMSPublicConfigurationItem      `json:"aliyunSms,omitempty" dc:"Aliyun SMS public settings when selected" eg:"{}"`
	SMSBao         *SMSBaoPublicConfigurationItem         `json:"smsBao,omitempty" dc:"SMSBao public settings when selected" eg:"{}"`
	TencentSMS     *TencentSMSPublicConfigurationItem     `json:"tencentSms,omitempty" dc:"Tencent Cloud SMS public settings when SMS is configured" eg:"{}"`
	WeChatOfficial *WeChatOfficialPublicConfigurationItem `json:"weChatOfficial,omitempty" dc:"WeChat Official Account public settings when configured" eg:"{}"`
	WeCom          *WeComPublicConfigurationItem          `json:"weCom,omitempty" dc:"WeCom public settings when configured" eg:"{}"`
	SignedWebhook  *SignedWebhookPublicConfigurationItem  `json:"signedWebhook,omitempty" dc:"Signed Webhook public settings when configured" eg:"{}"`
}

type AliyunSMSPublicConfigurationItem struct {
	SignName string `json:"signName" dc:"Approved Aliyun SMS signature" eg:"Example IDC"`
}

type SMSBaoPublicConfigurationItem struct {
	SignName  string `json:"signName" dc:"Reported SMS signature without brackets" eg:"Example IDC"`
	ProductID string `json:"productId" dc:"Optional SMSBao dedicated product ID" eg:"10001"`
}

type SMTPPublicConfigurationItem struct {
	Host        string `json:"host" dc:"Configured SMTP host that passed the deployment allowlist" eg:"mail.example.com"`
	Port        int    `json:"port" dc:"Configured TLS-only SMTP port" eg:"465"`
	Security    string `json:"security" dc:"SMTP transport security mode: implicit_tls or starttls" eg:"implicit_tls"`
	Username    string `json:"username" dc:"Configured SMTP authentication username" eg:"mailer@example.com"`
	FromAddress string `json:"fromAddress" dc:"Configured sender email address" eg:"notice@example.com"`
	FromName    string `json:"fromName" dc:"Configured sender display name" eg:"Example IDC"`
}
type TencentSMSPublicConfigurationItem struct {
	Region      string `json:"region" dc:"Tencent Cloud SMS API region" eg:"ap-guangzhou"`
	SmsSdkAppID string `json:"smsSdkAppId" dc:"Tencent Cloud SMS application ID" eg:"1400000000"`
	SignName    string `json:"signName" dc:"Approved Tencent Cloud SMS signature" eg:"Example IDC"`
}
type WeChatOfficialPublicConfigurationItem struct {
	AppID              string `json:"appId" dc:"Configured verified WeChat Official Account AppID" eg:"wx1234567890abcdef"`
	CallbackVerified   bool   `json:"callbackVerified" dc:"Whether the encrypted callback challenge passed for the current configuration generation" eg:"true"`
	CallbackVerifiedAt *int64 `json:"callbackVerifiedAt" dc:"Encrypted callback verification time as Unix timestamp in milliseconds; null before verification" eg:"1786752000000"`
}
type WeComPublicConfigurationItem struct {
	CorpID  string `json:"corpId" dc:"Configured WeCom enterprise ID" eg:"ww1234567890abcdef"`
	AgentID int64  `json:"agentId" dc:"Configured WeCom custom application agent ID" eg:"1000002"`
}

type SignedWebhookPublicConfigurationItem struct {
	EndpointURL string `json:"endpointUrl" dc:"Configured public HTTPS Webhook endpoint" eg:"https://hooks.example.com/lina/events"`
}

type ChannelTestItem struct {
	OperationNo string `json:"operationNo" dc:"Opaque test operation number" eg:"ntest_01jnotify00000000000000001"`
	ChannelCode string `json:"channelCode" dc:"Tested channel code" eg:"sms"`
	TargetMask  string `json:"targetMask" dc:"Masked one-time destination; raw target is never persisted" eg:"+86***8000"`
	ResultClass string `json:"resultClass" dc:"Normalized real provider result" eg:"accepted"`
	Reused      bool   `json:"reused" dc:"Whether this response came from an earlier operation with the same idempotency key" eg:"false"`
	CompletedAt int64  `json:"completedAt" dc:"Completion time as Unix timestamp in milliseconds; zero means an interrupted result remains unknown" eg:"1786752000000"`
}

type EventItem struct {
	Code              string   `json:"code" dc:"Stable notification event code" eg:"support.ticket_updated"`
	Topic             string   `json:"topic" dc:"Event topic: account, billing, service, security, support, or marketing" eg:"support"`
	ObjectType        string   `json:"objectType" dc:"Authoritative business-object type referenced by this event" eg:"support_ticket"`
	RequiredVariables []string `json:"requiredVariables" dc:"Template variables required for this event" eg:"[]"`
	OptionalVariables []string `json:"optionalVariables" dc:"Template variables optionally available for this event" eg:"[]"`
	SecurityRequired  bool     `json:"securityRequired" dc:"Whether this event is a mandatory security notification that customer preferences cannot disable" eg:"false"`
	Version           int64    `json:"version" dc:"Registered event-definition version" eg:"1"`
}

type TemplateItem struct {
	TemplateNo     string                             `json:"templateNo" dc:"Opaque notification template number" eg:"ntpl_support_ticket_updated_in_app_zh"`
	Version        int64                              `json:"version" dc:"Immutable template version" eg:"2"`
	EventCode      string                             `json:"eventCode" dc:"Registered notification event code" eg:"support.ticket_updated"`
	ChannelCode    string                             `json:"channelCode" dc:"Notification channel code for this template" eg:"in_app"`
	Locale         string                             `json:"locale" dc:"Template locale: zh-CN or en-US" eg:"zh-CN"`
	Subject        string                             `json:"subject" dc:"Template subject exactly as stored" eg:"Ticket {{ticketNo}} was updated"`
	Body           string                             `json:"body" dc:"Template body exactly as stored" eg:"Your ticket {{ticketNo}} has a new reply."`
	Status         string                             `json:"status" dc:"Template status: draft=not used, active=eligible for delivery, disabled=retained but unavailable" eg:"active"`
	CreatedBy      int64                              `json:"createdBy" dc:"Host administrator user ID that created this version" eg:"1"`
	CreatedAt      int64                              `json:"createdAt" dc:"Template-version creation time as Unix timestamp in milliseconds" eg:"1786752000000"`
	SMS            *SMSTemplateMappingItem            `json:"sms,omitempty" dc:"Frozen mapping for the selected SMS provider" eg:"{}"`
	WeChatOfficial *WeChatOfficialTemplateMappingItem `json:"weChatOfficial,omitempty" dc:"Frozen WeChat Official Account data mapping" eg:"{}"`
}

type SMSTemplateMappingInput struct {
	ProviderCode  string            `json:"providerCode" v:"required|max-length:64" dc:"SMS provider code frozen with this template version" eg:"tencent_cloud_sms_v20210111"`
	TemplateID    string            `json:"templateId" v:"max-length:128" dc:"Approved provider template ID or code; empty only for SMSBao content matching" eg:"SMS_123456"`
	VariableOrder []string          `json:"variableOrder" dc:"Event variables in Tencent Cloud parameter order" eg:"[]"`
	DataMapping   map[string]string `json:"dataMapping" dc:"Aliyun template parameter name to event variable mapping" eg:"{}"`
}
type WeChatOfficialTemplateMappingInput struct {
	TemplateID  string            `json:"templateId" v:"required|max-length:128" dc:"WeChat Official Account template ID" eg:"template-id"`
	DataMapping map[string]string `json:"dataMapping" dc:"Provider data key to event variable name mapping" eg:"{}"`
}
type SMSTemplateMappingItem = SMSTemplateMappingInput
type WeChatOfficialTemplateMappingItem = WeChatOfficialTemplateMappingInput

type DeliveryItem struct {
	TaskNo              string `json:"taskNo" dc:"Opaque notification delivery task number" eg:"ntask_01jnotify000000000000000001"`
	ChainNo             string `json:"chainNo" dc:"Opaque delivery-chain number shared by the original task and manual-resend successors" eg:"nchain_01jnotify0000000000000001"`
	PreviousTaskNo      string `json:"previousTaskNo" dc:"Previous delivery task in the linear resend chain; empty for the root task" eg:"ntask_01jnotify000000000000000000"`
	EventCode           string `json:"eventCode" dc:"Stable notification event code" eg:"support.ticket_updated"`
	Topic               string `json:"topic" dc:"Notification topic: account, billing, service, security, support, or marketing" eg:"support"`
	ChannelCode         string `json:"channelCode" dc:"Notification channel code" eg:"in_app"`
	Locale              string `json:"locale" dc:"Locale frozen when the delivery task was created" eg:"zh-CN"`
	RecipientMask       string `json:"recipientMask" dc:"Masked recipient evidence that never exposes the raw destination" eg:"c***@example.com"`
	BusinessObjectType  string `json:"businessObjectType" dc:"Authoritative business-object type referenced by the notification" eg:"support_ticket"`
	BusinessObjectNo    string `json:"businessObjectNo" dc:"Opaque authoritative business-object number" eg:"ticket_1001"`
	Status              string `json:"status" dc:"Delivery status: queued, sending, retry_wait, submitted, delivered, failed, cancelled, preference_blocked, result_unknown, manual_review, or manual_unknown_closed" eg:"failed"`
	ResultClass         string `json:"resultClass" dc:"Normalized delivery result without raw provider details" eg:"definitive_not_received_permanent"`
	RetrySequence       int    `json:"retrySequence" dc:"Number of automatic retry attempts already consumed, from 0 to 3" eg:"3"`
	NextAttemptAt       *int64 `json:"nextAttemptAt" dc:"Next automatic attempt time as Unix timestamp in milliseconds; null when no retry is scheduled" eg:"1786752300000"`
	QueryDeadline       *int64 `json:"queryDeadline" dc:"Provider-result query deadline as Unix timestamp in milliseconds; null when no provider query is pending" eg:"1786755600000"`
	QueryDeadlineSource string `json:"queryDeadlineSource" dc:"Source of the query deadline, such as provider or local_policy; empty when no query is pending" eg:"provider"`
	ManualResendCount   int    `json:"manualResendCount" dc:"Manual resends already created in this chain, from 0 to 3" eg:"1"`
	IsChainTail         bool   `json:"isChainTail" dc:"Whether this task is the current linear delivery-chain tail" eg:"true"`
	Version             int64  `json:"version" dc:"Delivery optimistic version" eg:"4"`
	QueuedAt            int64  `json:"queuedAt" dc:"Delivery queue time as Unix timestamp in milliseconds" eg:"1786752000000"`
	CompletedAt         *int64 `json:"completedAt" dc:"Delivery completion time as Unix timestamp in milliseconds; null while unfinished" eg:"1786752005000"`
}

type ResendEligibilityItem struct {
	TaskNo           string   `json:"taskNo" dc:"Opaque delivery task number evaluated for manual resend" eg:"ntask_01jnotify000000000000000001"`
	TaskVersion      int64    `json:"taskVersion" dc:"Current delivery version observed during eligibility evaluation" eg:"4"`
	Allowed          bool     `json:"allowed" dc:"Whether a manual resend is currently allowed after every safety gate" eg:"false"`
	Reasons          []string `json:"reasons" dc:"Blocking reason codes; empty only when a manual resend is allowed" eg:"[]"`
	EvaluatedAt      int64    `json:"evaluatedAt" dc:"Eligibility evaluation time as Unix timestamp in milliseconds" eg:"1786752000000"`
	NextAllowedAt    *int64   `json:"nextAllowedAt" dc:"Earliest resend time as Unix timestamp in milliseconds when blocked by cooldown; null otherwise" eg:"1786752600000"`
	RemainingCount   int      `json:"remainingCount" dc:"Manual resends remaining in the chain, from 0 to 3" eg:"2"`
	ReplacedByTaskNo string   `json:"replacedByTaskNo" dc:"Current chain-tail task number when the requested task has already been replaced; empty otherwise" eg:"ntask_01jnotify000000000000000002"`
}
