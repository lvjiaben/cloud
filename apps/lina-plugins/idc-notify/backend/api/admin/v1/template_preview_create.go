package v1

import "github.com/gogf/gf/v2/frame/g"

type TemplatePreviewCreateReq struct {
	g.Meta     `path:"/admin/notifications/templates/{templateNo}/preview" method:"post" tags:"IDC Notification Templates" summary:"Preview notification template" dc:"Renders a notification-template version with a bounded variable map without sending or changing delivery state." permission:"idc:admin:notification:channel:manage"`
	TemplateNo string            `json:"templateNo" in:"path" v:"required|max-length:80" dc:"Opaque notification template number" eg:"ntpl_support_ticket_updated_in_app_zh"`
	Version    int64             `json:"version" v:"min:0" dc:"Optional template version; use 0 to preview the latest version" eg:"2"`
	Variables  map[string]string `json:"variables" dc:"Preview variable values keyed by the event variable name" eg:"{}"`
}
type TemplatePreviewCreateRes struct {
	Subject string `json:"subject" dc:"Rendered notification subject" eg:"Ticket ticket_1001 was updated"`
	Body    string `json:"body" dc:"Rendered notification body" eg:"Your support ticket has a new reply."`
}
