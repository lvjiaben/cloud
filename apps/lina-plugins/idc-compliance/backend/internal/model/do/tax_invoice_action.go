// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceAction is the golang structure of table plugin_idc_compliance_tax_invoice_action for DAO operations like Where/Data.
type TaxInvoiceAction struct {
	g.Meta               `orm:"table:plugin_idc_compliance_tax_invoice_action, do:true"`
	Id                   any        //
	TenantId             any        //
	ActionNo             any        //
	TaxInvoiceNo         any        //
	ActionCode           any        //
	IdempotencyDigest    any        //
	RequestFingerprint   any        //
	ActorUserId          any        //
	BeforeStatusCode     any        //
	AfterStatusCode      any        //
	BeforeVersion        any        //
	AfterVersion         any        //
	InvoiceTypeCode      any        //
	ProviderCode         any        //
	ReasonCode           any        //
	InternalNote         any        //
	CapabilityReasonCode any        //
	CreatedAt            *time.Time //
}
