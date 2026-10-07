package v1

import "github.com/gogf/gf/v2/frame/g"

// TicketStatusTransitionOption is one server-authorized target presented by a
// ticket detail response. Its base stage remains the only lifecycle meaning.
type TicketStatusTransitionOption struct {
	StatusNo       string `json:"statusNo" dc:"Custom status number"`
	NameZhCN       string `json:"nameZhCN" dc:"Simplified Chinese display name"`
	NameEnUS       string `json:"nameEnUS" dc:"English display name"`
	BaseStage      string `json:"baseStage" dc:"Mapped stable base stage"`
	Version        int64  `json:"version" dc:"Current custom status configuration version"`
	RequiresReason bool   `json:"requiresReason" dc:"Whether this transition requires an operator reason"`
}

// TicketStatusTransitionReq changes the ticket's custom status and derives the
// authoritative base stage from the active custom-status configuration.
type TicketStatusTransitionReq struct {
	g.Meta         `path:"/admin/support/tickets/{ticketNo}/status-transitions" method:"post" tags:"IDC Support" summary:"Change support ticket status" dc:"Changes an accessible support ticket to one active custom status. The server derives the stable base stage, rechecks the corresponding action permission, and applies optimistic concurrency, lifecycle, SLA, history, notification, and audit rules atomically." permission:"idc:admin:support:ticket:query"`
	TicketNo       string `json:"ticketNo" dc:"Support ticket number"`
	CustomStatusNo string `json:"customStatusNo" dc:"Active custom status number"`
	Reason         string `json:"reason" dc:"Optional transition reason; required for force close and reopen"`
	TicketVersion  int64  `json:"ticketVersion" dc:"Expected current ticket version"`
}

type TicketStatusTransitionRes struct {
	Item *TicketDetail `json:"item" dc:"Updated support ticket detail"`
}
