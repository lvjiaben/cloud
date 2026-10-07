// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentDownloadTicket is the golang structure of table plugin_idc_core_content_download_ticket for DAO operations like Where/Data.
type ContentDownloadTicket struct {
	g.Meta                 `orm:"table:plugin_idc_core_content_download_ticket, do:true"`
	Id                     any        //
	TenantId               any        //
	TicketNo               any        //
	TokenDigest            any        //
	ContentNo              any        //
	FileNo                 any        //
	ContentValidityVersion any        //
	VisibilityCode         any        //
	UserId                 any        //
	SessionReferenceDigest any        //
	AuthorizationRevision  any        //
	IssuedAt               *time.Time //
	ExpiresAt              *time.Time //
	RevokedAt              *time.Time //
}
