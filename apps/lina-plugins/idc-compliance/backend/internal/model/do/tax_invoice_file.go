// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceFile is the golang structure of table plugin_idc_compliance_tax_invoice_file for DAO operations like Where/Data.
type TaxInvoiceFile struct {
	g.Meta           `orm:"table:plugin_idc_compliance_tax_invoice_file, do:true"`
	Id               any        //
	TenantId         any        //
	FileNo           any        //
	TaxInvoiceNo     any        //
	FileTypeCode     any        //
	PrivateObjectRef any        //
	ContentSha256    any        //
	MediaType        any        //
	ByteSize         any        //
	StatusCode       any        //
	Immutable        any        //
	ArchivedAt       *time.Time //
	RetentionUntil   *time.Time //
	LegalHold        any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
