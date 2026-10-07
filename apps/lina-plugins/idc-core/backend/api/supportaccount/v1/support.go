// This file declares customer-owned support APIs, including bounded numbered or cursor ticket queries.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CategoryLabel struct {
	CategoryNo string `json:"categoryNo"`
	NameZhCN   string `json:"nameZhCN"`
	NameEnUS   string `json:"nameEnUS"`
}
type CategoryOption struct {
	CategoryLabel
	Priority string `json:"priority"`
}
type CategoryListReq struct {
	g.Meta          `path:"/account/support/categories" method:"get" tags:"IDC Support Account" summary:"List customer-facing support categories"`
	Keyword, Cursor string
	Limit           int `d:"50" v:"min:1|max:100"`
}
type CategoryListRes struct {
	Items      []CategoryOption `json:"items"`
	NextCursor string           `json:"nextCursor"`
}
type TicketItem struct {
	TicketNo           string                    `json:"ticketNo"`
	Category           *CategoryLabel            `json:"category,omitempty"`
	Subject            string                    `json:"subject"`
	Status             string                    `json:"status"`
	Priority           string                    `json:"priority"`
	CurrentRound       int                       `json:"currentRound"`
	Version            int64                     `json:"version"`
	ResolvedAt         int64                     `json:"resolvedAt,omitempty"`
	ClosedAt           int64                     `json:"closedAt,omitempty"`
	LastActivityAt     int64                     `json:"lastActivityAt"`
	CreatedAt          int64                     `json:"createdAt"`
	UpdatedAt          int64                     `json:"updatedAt"`
	RelatedEntitlement *TicketRelatedEntitlement `json:"relatedEntitlement,omitempty" dc:"Immutable entitlement link snapshot and current safe projection" eg:"sar_0123456789abcdef0123456789abcdef"`
}
type Attachment struct {
	AttachmentNo string `json:"attachmentNo"`
	FileName     string `json:"fileName"`
	MediaType    string `json:"mediaType"`
	SizeBytes    int64  `json:"sizeBytes"`
	CreatedAt    int64  `json:"createdAt"`
}
type Message struct {
	MessageNo   string       `json:"messageNo"`
	Kind        string       `json:"kind"`
	Author      string       `json:"author"`
	Body        string       `json:"body"`
	Automated   bool         `json:"automated"`
	CreatedAt   int64        `json:"createdAt"`
	Attachments []Attachment `json:"attachments"`
}
type Rating struct {
	Stars       int    `json:"stars"`
	RoundNo     int    `json:"roundNo"`
	Feedback    string `json:"feedback,omitempty"`
	SubmittedAt int64  `json:"submittedAt"`
}
type TicketDetail struct {
	Ticket         *TicketItem              `json:"ticket"`
	Messages       []Message                `json:"messages"`
	AllowedActions []string                 `json:"allowedActions"`
	ReopenDeadline int64                    `json:"reopenDeadline,omitempty"`
	Rating         *Rating                  `json:"rating,omitempty"`
	CustomFields   []TicketCustomFieldValue `json:"customFields" dc:"Immutable customer-visible custom field snapshots" eg:"SCF_service_area"`
}
type TicketListReq struct {
	g.Meta `path:"/account/support/tickets" method:"get" tags:"IDC Support Account" summary:"List current customer tickets" dc:"Return a bounded page and matching total of customer-visible tickets owned by the current customer. Numbered pages and cursors cannot be combined."`
	Stage  string `json:"stage" dc:"Optional customer stage: new, in_progress, waiting_customer, resolved or closed; omitted includes all stages." eg:"new"`
	Query  string `json:"query" dc:"Optional exact ticket number or literal subject prefix; omitted disables keyword filtering." eg:"Network"`
	Cursor string `json:"cursor" dc:"Optional opaque continuation cursor. Do not combine with page." eg:""`
	Page   int    `json:"page" v:"min:0" dc:"One-based page number; zero or omitted uses cursor mode." eg:"1"`
	Limit  int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Page size from one to one hundred; defaults to twenty-five." eg:"10"`
}
type TicketListRes struct {
	Tickets    []*TicketItem `json:"tickets"`
	NextCursor string        `json:"nextCursor"`
	Total      int64         `json:"total" dc:"Exact count matching the same customer ownership and filters before pagination." eg:"23"`
}
type TicketDetailReq struct {
	g.Meta   `path:"/account/support/tickets/{ticketNo}" method:"get" tags:"IDC Support Account" summary:"Get current customer ticket"`
	TicketNo string `json:"ticketNo"`
}
type TicketDetailRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketCreateReq struct {
	g.Meta               `path:"/account/support/tickets" method:"post" tags:"IDC Support Account" summary:"Create customer support ticket"`
	CategoryNo           string                        `json:"categoryNo"`
	Subject              string                        `json:"subject"`
	Body                 string                        `json:"body"`
	UploadNos            []string                      `json:"uploadNos"`
	AssociationReference string                        `json:"associationReference" dc:"Optional opaque support association reference for a non-terminal long-term entitlement owned by the current customer" eg:"sar_0123456789abcdef0123456789abcdef"`
	CustomFields         []TicketCustomFieldSubmission `json:"customFields" dc:"Validated customer-visible custom field submissions" eg:"SCF_service_area"`
}
type TicketCreateRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketReplyReq struct {
	g.Meta        `path:"/account/support/tickets/{ticketNo}/replies" method:"post" tags:"IDC Support Account" summary:"Reply to current customer ticket"`
	TicketNo      string   `json:"ticketNo"`
	Body          string   `json:"body"`
	UploadNos     []string `json:"uploadNos"`
	TicketVersion int64    `json:"ticketVersion"`
}
type TicketReplyRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketTransitionReq struct {
	TicketNo      string `json:"ticketNo"`
	TicketVersion int64  `json:"ticketVersion"`
	Confirmed     bool   `json:"confirmed"`
}
type TicketCloseReq struct {
	g.Meta `path:"/account/support/tickets/{ticketNo}/closures" method:"post" tags:"IDC Support Account" summary:"Close current customer ticket"`
	TicketTransitionReq
}
type TicketCloseRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketReopenReq struct {
	g.Meta `path:"/account/support/tickets/{ticketNo}/reopenings" method:"post" tags:"IDC Support Account" summary:"Reopen current customer ticket"`
	TicketTransitionReq
}
type TicketReopenRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketRatingReq struct {
	g.Meta        `path:"/account/support/tickets/{ticketNo}/ratings" method:"post" tags:"IDC Support Account" summary:"Rate current ticket round"`
	TicketNo      string `json:"ticketNo"`
	Stars         int    `json:"stars"`
	RoundNo       int    `json:"roundNo"`
	Feedback      string `json:"feedback"`
	TicketVersion int64  `json:"ticketVersion"`
}
type TicketRatingRes struct {
	Item *TicketDetail `json:"item"`
}
type AttachmentUploadCreateReq struct {
	g.Meta    `path:"/account/support/tickets/{ticketNo}/attachment-uploads" method:"post" tags:"IDC Support Account" summary:"Reserve customer ticket attachment"`
	TicketNo  string `json:"ticketNo"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
}
type AttachmentUploadCreateRes struct {
	UploadNo   string `json:"uploadNo"`
	NextOffset int64  `json:"nextOffset"`
	ExpiresAt  int64  `json:"expiresAt"`
	Version    int64  `json:"version"`
}
type InitialAttachmentUploadCreateReq struct {
	g.Meta    `path:"/account/support/initial-attachment-uploads" method:"post" tags:"IDC Support Account" summary:"Reserve an attachment for a new customer ticket"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
}
type InitialAttachmentUploadCreateRes = AttachmentUploadCreateRes
type AttachmentUploadContentUpdateReq struct {
	g.Meta   `path:"/account/support/ticket-attachment-uploads/{uploadNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Support Account" summary:"Upload reserved customer ticket attachment"`
	UploadNo string `json:"uploadNo"`
}
type InitialAttachmentUploadContentUpdateReq struct {
	g.Meta   `path:"/account/support/initial-attachment-uploads/{uploadNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Support Account" summary:"Upload an attachment reserved for a new customer ticket"`
	UploadNo string `json:"uploadNo"`
}
type InitialAttachmentUploadContentUpdateRes = AttachmentUploadContentUpdateRes
type AttachmentUploadContentUpdateRes struct {
	UploadNo  string `json:"uploadNo"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
	Version   int64  `json:"version"`
	Ready     bool   `json:"ready"`
}
type AttachmentOpenReq struct {
	g.Meta       `path:"/account/support/attachments/{attachmentNo}/content" method:"get" tags:"IDC Support Account" summary:"Download a visible customer ticket attachment"`
	AttachmentNo string `json:"attachmentNo"`
}
type AttachmentOpenRes struct{}
