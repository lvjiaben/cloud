// This file defines the administrator contract for one confirmed real channel test.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ChannelTestCreateReq carries one confirmed provider test without persisting its raw target.
type ChannelTestCreateReq struct {
	g.Meta          `path:"/admin/notifications/channels/{channelCode}/tests" method:"post" tags:"IDC Notification Channels" summary:"Send one real channel test" dc:"Performs one real provider delivery to an administrator-entered target after current active session and explicit confirmation. The raw target and variable values are not persisted; ambiguous interrupted operations are never sent again automatically." permission:"idc:admin:notification:channel:manage"`
	ChannelCode     string            `json:"channelCode" in:"path" v:"required|in:email,sms,wechat_official,wechat_work" dc:"External channel to test; WeCom uses only the current operator's active self-service binding" eg:"sms"`
	Target          string            `json:"target" v:"max-length:512" dc:"One-time email, E.164 phone number, or Official Account OpenID; it must be empty for WeCom" eg:"+8613800138000"`
	TemplateNo      string            `json:"templateNo" v:"required|max-length:80" dc:"Active local template number used for this test" eg:"ntpl_service_expiry_sms_zh"`
	TemplateVersion int64             `json:"templateVersion" v:"min:1" dc:"Immutable local template version used for this test" eg:"2"`
	Variables       map[string]string `json:"variables" dc:"Values for the selected event variables; keys are validated against the event catalog and provider mapping" eg:"{}"`
	ExpectedVersion int64             `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"3"`
	IdempotencyKey  string            `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key; reusing it never duplicates the external send" eg:"channel-test-20260815-001"`
	Confirmed       bool              `json:"confirmed" v:"required|in:true" dc:"Explicit acknowledgement that this is a real delivery and SMS may incur provider charges" eg:"true"`
}

// ChannelTestCreateRes returns masked evidence for the completed or uncertain test.
type ChannelTestCreateRes struct {
	Item *ChannelTestItem `json:"item" dc:"Masked test-delivery evidence" eg:"{}"`
}
