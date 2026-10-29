package notification

import "time"

const (
	PermissionChannelManage = "idc:admin:notification:channel:manage"
	MaximumPageSize         = 100
	MaximumReasonBytes      = 500
	MaximumTemplateBytes    = 256 * 1024
	MaximumVariables        = 64
)

type ChannelStatus string

const (
	ChannelUnconfigured        ChannelStatus = "unconfigured"
	ChannelConfiguring         ChannelStatus = "configuring"
	ChannelEnabled             ChannelStatus = "enabled"
	ChannelDisabled            ChannelStatus = "disabled"
	ChannelConfigurationFailed ChannelStatus = "configuration_failed"
)

type Channel struct {
	Code                    string
	Kind                    string
	ProviderCode            string
	DisplayName             string
	Status                  ChannelStatus
	BalanceState            string
	Configured              bool
	CredentialMaskedTail    string
	ConfigurationGeneration int64
	PendingQueryCount       int
	Version                 int64
	UpdatedAt               time.Time
	Configuration           ChannelPublicConfiguration
}

type ChannelPublicConfiguration struct {
	SMTP       *SMTPPublicConfiguration
	AliyunSMS  *AliyunSMSPublicConfiguration
	SMSBao     *SMSBaoPublicConfiguration
	TencentSMS *TencentSMSPublicConfiguration
}

type AliyunSMSPublicConfiguration struct {
	SignName string
}

type SMSBaoPublicConfiguration struct {
	SignName  string
	ProductID string
}

type SMTPPublicConfiguration struct {
	Host        string
	Port        int
	Security    string
	Username    string
	FromAddress string
	FromName    string
}

type TencentSMSPublicConfiguration struct {
	Region      string
	SmsSdkAppID string
	SignName    string
}

type ChannelConfigurationInput struct {
	ChannelCode     string
	ProviderCode    string
	NonSecret       map[string]string
	Secrets         map[string][]byte
	ExpectedVersion int64
	IdempotencyKey  string
}

type SMTPConfigurationInput struct {
	Host            string
	Port            int
	Security        string
	Username        string
	Password        []byte
	FromAddress     string
	FromName        string
	ExpectedVersion int64
	IdempotencyKey  string
}

type TencentSMSConfigurationInput struct {
	Region          string
	SmsSdkAppID     string
	SignName        string
	SecretID        []byte
	SecretKey       []byte
	ExpectedVersion int64
	IdempotencyKey  string
}

type AliyunSMSConfigurationInput struct {
	SignName        string
	AccessKeyID     []byte
	AccessKeySecret []byte
	ExpectedVersion int64
	IdempotencyKey  string
}

type SMSBaoConfigurationInput struct {
	SignName        string
	ProductID       string
	Username        []byte
	APIKey          []byte
	ExpectedVersion int64
	IdempotencyKey  string
}

type ChannelTestInput struct {
	ChannelCode     string
	Target          []byte
	TemplateNo      string
	TemplateVersion int64
	Variables       map[string]string
	ExpectedVersion int64
	IdempotencyKey  string
	Confirmed       bool
}

type ChannelTestResult struct {
	OperationNo string
	ChannelCode string
	TargetMask  string
	ResultClass string
	Reused      bool
	CompletedAt time.Time
}

type ChannelLifecycleInput struct {
	ChannelCode     string
	ExpectedVersion int64
	IdempotencyKey  string
	Confirmed       bool
}

type Event struct {
	Code              string
	Topic             string
	ObjectType        string
	RequiredVariables []string
	OptionalVariables []string
	SecurityRequired  bool
	Version           int64
}

type Template struct {
	TemplateNo  string
	Version     int64
	EventCode   string
	ChannelCode string
	Locale      string
	Subject     string
	Body        string
	Status      string
	CreatedBy   int64
	CreatedAt   time.Time
	SMS         *SMSTemplateMapping
}

type SMSTemplateMapping struct {
	ProviderCode  string
	TemplateID    string
	VariableOrder []string
	DataMapping   map[string]string
}

type TemplateWriteInput struct {
	TemplateNo      string
	EventCode       string
	ChannelCode     string
	Locale          string
	Subject         string
	Body            string
	Status          string
	ExpectedVersion int64
	IdempotencyKey  string
	SMS             *SMSTemplateMapping
}

type TemplatePreviewInput struct {
	TemplateNo string
	Version    int64
	Variables  map[string]string
}

type TemplatePreview struct {
	Subject string
	Body    string
}

type DeliveryStatus string

const (
	DeliveryQueued              DeliveryStatus = "queued"
	DeliverySending             DeliveryStatus = "sending"
	DeliveryRetryWait           DeliveryStatus = "retry_wait"
	DeliverySubmitted           DeliveryStatus = "submitted"
	DeliveryDelivered           DeliveryStatus = "delivered"
	DeliverySucceeded           DeliveryStatus = "succeeded"
	DeliveryFailed              DeliveryStatus = "failed"
	DeliveryResultUnknown       DeliveryStatus = "result_unknown"
	DeliveryManualReview        DeliveryStatus = "manual_review"
	DeliveryManualUnknownClosed DeliveryStatus = "manual_unknown_closed"
	DeliveryCancelled           DeliveryStatus = "cancelled"
	DeliveryTemplateError       DeliveryStatus = "template_error"
	DeliveryPreferenceBlocked   DeliveryStatus = "preference_blocked"
)

type Delivery struct {
	TaskNo              string
	ChainNo             string
	PreviousTaskNo      string
	EventCode           string
	Topic               string
	ChannelCode         string
	Locale              string
	RecipientMask       string
	BusinessObjectType  string
	BusinessObjectNo    string
	Status              DeliveryStatus
	ResultClass         string
	RetrySequence       int
	NextAttemptAt       *time.Time
	QueryDeadline       *time.Time
	QueryDeadlineSource string
	ManualResendCount   int
	IsChainTail         bool
	Version             int64
	QueuedAt            time.Time
	CompletedAt         *time.Time
}

type DeliveryPage struct {
	Items      []Delivery
	NextCursor string
}

type DeliveryListInput struct {
	ChannelCode string
	Status      string
	AfterTaskNo string
	Limit       int
}

type ResendReason string

const (
	ResendReasonTaskChanged           ResendReason = "task_state_changed"
	ResendReasonChannelDisabled       ResendReason = "channel_disabled"
	ResendReasonInsufficientBalance   ResendReason = "insufficient_balance"
	ResendReasonRecipientUnsubscribed ResendReason = "recipient_unsubscribed"
	ResendReasonUnsafe                ResendReason = "unavailable"
	ResendReasonRecentLoginRequired   ResendReason = "recent_login_required"
	ResendReasonCooldown              ResendReason = "cooldown"
	ResendReasonLimitReached          ResendReason = "limit_reached"
)

type ResendEligibility struct {
	TaskNo           string
	TaskVersion      int64
	Allowed          bool
	Reasons          []ResendReason
	EvaluatedAt      time.Time
	NextAllowedAt    *time.Time
	RemainingCount   int
	ReplacedByTaskNo string
}

type ManualResendInput struct {
	TaskNo          string
	ExpectedVersion int64
	OperationNo     string
	Reason          string
	Confirmed       bool
}

type ManualClosureInput struct {
	TaskNo          string
	ExpectedVersion int64
	OperationNo     string
	Reason          string
	Confirmed       bool
}

type EnqueueInput struct {
	EventCode             string
	ChannelCode           string
	Locale                string
	OwnerUserID           int
	CustomerNo            string
	ContactNo             string
	BusinessObjectType    string
	BusinessObjectNo      string
	BusinessObjectVersion int64
	MarketingSnapshotNo   string
	TemplateNo            string
	TemplateVersion       int64
	Subject               string
	Body                  string
	Variables             map[string]string
	IdempotencyKey        string
}
