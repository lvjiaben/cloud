package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	accountv1 "lina-plugin-idc-core/backend/api/supportaccount/v1"
)

type SupportCategoryOption struct {
	CategoryNo string `json:"categoryNo"`
	NameZhCN   string `json:"nameZhCN"`
	NameEnUS   string `json:"nameEnUS"`
	Priority   string `json:"priority"`
}
type SupportCategoryListReq struct {
	g.Meta  `path:"/public/support/categories" method:"get" tags:"IDC Public Support" summary:"List customer-facing presales categories"`
	Keyword string `json:"keyword"`
	Cursor  string `json:"cursor"`
	Limit   int    `json:"limit" d:"50" v:"min:1|max:100"`
}
type SupportCategoryListRes struct {
	Items      []SupportCategoryOption `json:"items"`
	NextCursor string                  `json:"nextCursor"`
}

type PresalesAccessVerifyReq struct {
	g.Meta      `path:"/public/support/presales/access-verifications" method:"post" tags:"IDC Public Support" summary:"Verify a two-hour presales access link"`
	AccessToken string `json:"accessToken"`
}
type PresalesAccessVerifyRes struct {
	Item      *accountv1.TicketDetail `json:"item"`
	ExpiresAt int64                   `json:"expiresAt"`
}
type PresalesReplyReq struct {
	g.Meta        `path:"/public/support/presales/replies" method:"post" tags:"IDC Public Support" summary:"Reply through a verified presales access link"`
	AccessToken   string   `json:"accessToken"`
	Body          string   `json:"body"`
	TicketVersion int64    `json:"ticketVersion"`
	UploadNos     []string `json:"uploadNos" dc:"Bounded attachment upload numbers reserved and uploaded with the same access token"`
}
type PresalesReplyRes struct {
	Item      *accountv1.TicketDetail `json:"item"`
	ExpiresAt int64                   `json:"expiresAt"`
}
type PresalesCloseReq struct {
	g.Meta        `path:"/public/support/presales/closures" method:"post" tags:"IDC Public Support" summary:"Close a presales ticket through a verified access link"`
	AccessToken   string `json:"accessToken" dc:"Two-hour access token bound to one presales ticket"`
	TicketVersion int64  `json:"ticketVersion" dc:"Version returned by the latest ticket read"`
}
type PresalesCloseRes struct {
	Item      *accountv1.TicketDetail `json:"item" dc:"Customer-safe ticket detail"`
	ExpiresAt int64                   `json:"expiresAt" dc:"Access token expiry as Unix milliseconds"`
}
type PresalesReopenReq struct {
	g.Meta        `path:"/public/support/presales/reopenings" method:"post" tags:"IDC Public Support" summary:"Reopen a presales ticket through a verified access link"`
	AccessToken   string `json:"accessToken" dc:"Two-hour access token bound to one presales ticket"`
	TicketVersion int64  `json:"ticketVersion" dc:"Version returned by the latest ticket read"`
}
type PresalesReopenRes = PresalesCloseRes
type PresalesRatingReq struct {
	g.Meta        `path:"/public/support/presales/ratings" method:"post" tags:"IDC Public Support" summary:"Rate a presales ticket through a verified access link"`
	AccessToken   string `json:"accessToken" dc:"Two-hour access token bound to one presales ticket"`
	TicketVersion int64  `json:"ticketVersion" dc:"Version returned by the latest ticket read"`
	RoundNo       int    `json:"roundNo" dc:"Current business round returned by the ticket"`
	Stars         int    `json:"stars" dc:"Rating from 1 to 5"`
	Feedback      string `json:"feedback" dc:"Optional rating feedback"`
}
type PresalesRatingRes = PresalesCloseRes
type PresalesAttachmentUploadCreateReq struct {
	g.Meta      `path:"/public/support/presales/attachment-uploads" method:"post" tags:"IDC Public Support" summary:"Reserve an anonymous presales attachment" dc:"Reserve one customer-visible attachment against the single presales ticket bound to a live two-hour access token."`
	AccessToken string `json:"accessToken" dc:"Live two-hour access token bound to one presales ticket"`
	FileName    string `json:"fileName" dc:"Original local file name used for actual-type inspection"`
	SizeBytes   int64  `json:"sizeBytes" dc:"Declared file size from 1 byte through 20 MiB"`
}
type PresalesAttachmentUploadCreateRes struct {
	UploadNo   string `json:"uploadNo" dc:"Opaque upload reservation number"`
	NextOffset int64  `json:"nextOffset" dc:"Required next byte offset; zero for the single raw upload"`
	ExpiresAt  int64  `json:"expiresAt" dc:"Reservation expiry as Unix milliseconds"`
	Version    int64  `json:"version" dc:"Optimistic reservation version"`
}
type PresalesAttachmentUploadContentUpdateReq struct {
	g.Meta      `path:"/public/support/presales/attachment-uploads/{uploadNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Public Support" summary:"Upload an anonymous presales attachment" dc:"Upload the exact declared bytes after rechecking that the access token still grants the same presales ticket."`
	UploadNo    string `json:"uploadNo" dc:"Opaque upload reservation number"`
	AccessToken string `json:"accessToken" dc:"Live two-hour access token bound to the reservation ticket"`
}
type PresalesAttachmentUploadContentUpdateRes struct {
	UploadNo  string `json:"uploadNo" dc:"Opaque upload reservation number"`
	FileName  string `json:"fileName" dc:"Inspected file name"`
	MediaType string `json:"mediaType" dc:"Server-detected safe media type"`
	SizeBytes int64  `json:"sizeBytes" dc:"Server-confirmed file size"`
	Version   int64  `json:"version" dc:"Updated optimistic reservation version"`
	Ready     bool   `json:"ready" dc:"Whether the upload may be selected by the next anonymous reply"`
}
type PresalesAttachmentOpenReq struct {
	g.Meta       `path:"/public/support/presales/attachments/{attachmentNo}/content" method:"post" tags:"IDC Public Support" summary:"Download an attachment through a valid presales access link"`
	AttachmentNo string `json:"attachmentNo"`
	AccessToken  string `json:"accessToken"`
}
type PresalesAttachmentOpenRes struct{}
