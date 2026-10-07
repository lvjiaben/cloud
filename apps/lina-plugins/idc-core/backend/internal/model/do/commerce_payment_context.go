// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommercePaymentContext is the golang structure of table plugin_idc_core_commerce_payment_context for DAO operations like Where/Data.
type CommercePaymentContext struct {
	g.Meta             `orm:"table:plugin_idc_core_commerce_payment_context, do:true"`
	Id                 any        //
	TenantId           any        //
	ContextNo          any        //
	OrderNo            any        //
	InvoiceNo          any        //
	CustomerNo         any        //
	CustomerTypeCode   any        //
	StatusCode         any        //
	ReasonCode         any        //
	IdentityStatusCode any        //
	IdentityCaseNo     any        // Opaque compliance-owned case reference from the most recent evaluation, never raw identity material
	IdentityVersion    any        //
	FirstRequestedAt   *time.Time //
	LastEvaluatedAt    *time.Time //
	ReadyAt            *time.Time //
	ExpiresAt          *time.Time //
	Version            any        //
	CreatedAt          *time.Time //
	UpdatedAt          *time.Time //
}
