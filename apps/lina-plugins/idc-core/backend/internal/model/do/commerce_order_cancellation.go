// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderCancellation is the golang structure of table plugin_idc_core_commerce_order_cancellation for DAO operations like Where/Data.
type CommerceOrderCancellation struct {
	g.Meta                    `orm:"table:plugin_idc_core_commerce_order_cancellation, do:true"`
	Id                        any        //
	TenantId                  any        //
	CancellationNo            any        //
	OrderNo                   any        //
	CustomerNo                any        //
	SourceCode                any        //
	ReasonCode                any        //
	RequestDigest             any        //
	StatusCode                any        //
	PaymentClosureOutcomeCode *string    //
	PaymentClosureReference   *string    //
	PaymentClosureDigest      *string    //
	RequestedBy               any        //
	RequestedAt               *time.Time //
	CompletedAt               *time.Time //
	Version                   any        //
	CreatedAt                 *time.Time //
	UpdatedAt                 *time.Time //
}
