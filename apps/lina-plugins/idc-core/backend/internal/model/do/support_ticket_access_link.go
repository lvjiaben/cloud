// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketAccessLink is the golang structure of table plugin_idc_core_support_ticket_access_link for DAO operations like Where/Data.
type SupportTicketAccessLink struct {
	g.Meta            `orm:"table:plugin_idc_core_support_ticket_access_link, do:true"`
	Id                any        //
	TenantId          any        //
	LinkNo            any        //
	TicketNo          any        //
	TokenDigest       any        //
	DestinationDigest any        //
	DestinationKind   any        //
	OperationDigest   any        //
	RequestDigest     any        //
	TokenSecretRef    any        //
	ExpiresAt         *time.Time //
	RevokedAt         *time.Time //
	CreatedAt         *time.Time //
}
