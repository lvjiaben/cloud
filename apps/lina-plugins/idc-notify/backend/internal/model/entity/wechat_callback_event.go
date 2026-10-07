// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// WechatCallbackEvent is the golang structure for table wechat_callback_event.
type WechatCallbackEvent struct {
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	CallbackNo         string     `json:"callbackNo"         orm:"callback_no"          description:""`
	ProviderMessageRef string     `json:"providerMessageRef" orm:"provider_message_ref" description:""`
	EventDigest        string     `json:"eventDigest"        orm:"event_digest"         description:""`
	ResultClass        string     `json:"resultClass"        orm:"result_class"         description:""`
	AppliedTaskNo      any        `json:"appliedTaskNo"      orm:"applied_task_no"      description:""`
	ReceivedAt         *time.Time `json:"receivedAt"         orm:"received_at"          description:""`
}
