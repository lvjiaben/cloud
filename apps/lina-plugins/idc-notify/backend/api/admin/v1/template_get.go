package v1

import "github.com/gogf/gf/v2/frame/g"

type TemplateGetReq struct {
	g.Meta     `path:"/admin/notifications/templates/{templateNo}" method:"get" tags:"IDC Notification Templates" summary:"Get notification template version" dc:"Returns one authorized immutable notification-template version, or the latest version when no version is supplied." permission:"idc:admin:notification:channel:manage"`
	TemplateNo string `json:"templateNo" in:"path" v:"required|max-length:80" dc:"Opaque notification template number" eg:"ntpl_support_ticket_updated_in_app_zh"`
	Version    int64  `json:"version" in:"query" v:"min:0" dc:"Optional template version; use 0 or omit to read the latest version" eg:"2"`
}
type TemplateGetRes struct {
	Item *TemplateItem `json:"item" dc:"Notification template version" eg:"{}"`
}
