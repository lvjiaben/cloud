package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerAssistanceSessionStartReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/assistance-sessions" method:"post" tags:"IDC Customer Management" summary:"Start a read-only customer assistance session" dc:"Create one explicitly confirmed fifteen-minute read-only assistance session for an active staff member." permission:"idc:admin:customer:assist"`
	CustomerNo     string `json:"customerNo" in:"path" v:"required|length:36,36" dc:"Stable external customer number"`
	Reason         string `json:"reason" v:"required|min-length:10|max-length:512" dc:"Required staff explanation for this assistance session"`
	TicketNo       string `json:"ticketNo" v:"max-length:64" dc:"Optional related support ticket number"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:12|max-length:128" dc:"Replay-safe request key"`
}

type CustomerAssistanceSessionStartRes struct {
	SessionNo  string `json:"sessionNo"`
	CustomerNo string `json:"customerNo"`
	Status     string `json:"status"`
	LaunchURL  string `json:"launchUrl" dc:"Absolute customer application URL containing the one-time launch token"`
	StartedAt  int64  `json:"startedAt" dc:"Database start time in Unix milliseconds"`
	ExpiresAt  int64  `json:"expiresAt" dc:"Database expiry time in Unix milliseconds"`
}

type CustomerAssistanceSessionEndReq struct {
	g.Meta    `path:"/admin/customer-assistance/{sessionNo}/end" method:"post" tags:"IDC Customer Management" summary:"End a customer assistance session" dc:"End the calling staff member's own active assistance session." permission:"idc:admin:customer:assist"`
	SessionNo string `json:"sessionNo" in:"path" v:"required|length:36,36" dc:"Stable assistance session number"`
}

type CustomerAssistanceSessionEndRes struct {
	SessionNo  string `json:"sessionNo"`
	CustomerNo string `json:"customerNo"`
	Status     string `json:"status"`
	EndedAt    int64  `json:"endedAt" dc:"Database end time in Unix milliseconds"`
}
