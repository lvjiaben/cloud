// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerInvoiceProfile is the golang structure of table plugin_idc_core_customer_invoice_profile for DAO operations like Where/Data.
type CustomerInvoiceProfile struct {
	g.Meta               `orm:"table:plugin_idc_core_customer_invoice_profile, do:true"`
	Id                   any        //
	TenantId             any        //
	ProfileNo            any        //
	CustomerNo           any        //
	TitleTypeCode        any        //
	InvoiceTitle         any        //
	TaxRegistrationNo    any        //
	RegisteredAddress    any        //
	RegisteredPhone      any        //
	BankName             any        //
	BankAccountMasked    any        //
	BankAccountSecretRef any        // Opaque core secret-envelope reference; never returned by ordinary APIs or logs
	IsDefault            any        //
	StatusCode           any        //
	Version              any        //
	CreatedBy            any        //
	UpdatedBy            any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
