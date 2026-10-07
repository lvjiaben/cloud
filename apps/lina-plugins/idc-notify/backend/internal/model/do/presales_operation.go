// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PresalesOperation is the golang structure of table plugin_idc_notify_presales_operation for DAO operations like Where/Data.
type PresalesOperation struct {
	g.Meta                  `orm:"table:plugin_idc_notify_presales_operation, do:true"`
	TenantId                any        //
	OperationDigest         any        //
	OperationCode           any        //
	VerificationNo          any        //
	DestinationKind         any        //
	DestinationDigest       any        //
	BrowserDigest           any        //
	TicketDigest            any        //
	RequestDigest           any        //
	StatusCode              any        //
	TicketNo                any        //
	AccessExpiresAt         *time.Time //
	LinkDeliveryState       any        //
	ProviderBoundaryCrossed any        //
	CompletedAt             *time.Time //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
