package v1

import "github.com/gogf/gf/v2/frame/g"

type TemplateUpdateReq struct {
	g.Meta          `path:"/admin/notifications/templates" method:"put" tags:"IDC Notification Templates" summary:"Create notification template version" dc:"Creates an immutable notification-template version after validating event variables, channel, locale, status, optimistic version, and idempotency key." permission:"idc:admin:notification:channel:manage"`
	TemplateNo      string                              `json:"templateNo" v:"max-length:80" dc:"Existing opaque template number; omit only when creating a new template series" eg:"ntpl_support_ticket_updated_in_app_zh"`
	EventCode       string                              `json:"eventCode" v:"required|max-length:96" dc:"Registered notification event code" eg:"support.ticket_updated"`
	ChannelCode     string                              `json:"channelCode" v:"required|max-length:32" dc:"Notification channel code for this template" eg:"in_app"`
	Locale          string                              `json:"locale" v:"required|in:zh-CN,en-US" dc:"Template locale: zh-CN or en-US" eg:"zh-CN"`
	Subject         string                              `json:"subject" v:"max-length:262144" dc:"Template subject using only variables declared by the selected event; may be empty for channels without a subject" eg:"Ticket {{ticketNo}} was updated"`
	Body            string                              `json:"body" v:"required|max-length:262144" dc:"Template body using only variables declared by the selected event" eg:"Your ticket {{ticketNo}} has a new reply."`
	Status          string                              `json:"status" v:"required|in:draft,active,disabled" dc:"Template status: draft=not used, active=eligible for delivery, disabled=retained but unavailable" eg:"active"`
	ExpectedVersion int64                               `json:"expectedVersion" v:"min:0" dc:"Expected latest template version; use 0 only when creating a new template series" eg:"1"`
	IdempotencyKey  string                              `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key for safely replaying the same template command" eg:"template-update-20260815-001"`
	SMS             *SMSTemplateMappingInput            `json:"sms" dc:"Required provider-specific mapping for SMS; belongs to this immutable local version" eg:"{}"`
	WeChatOfficial  *WeChatOfficialTemplateMappingInput `json:"weChatOfficial" dc:"Required WeChat provider template mapping for Official Account messages; belongs to this immutable local version" eg:"{}"`
}
type TemplateUpdateRes struct {
	Item *TemplateItem `json:"item" dc:"New immutable notification template version" eg:"{}"`
}
