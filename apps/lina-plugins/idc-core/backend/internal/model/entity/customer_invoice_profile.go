// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerInvoiceProfile is the golang structure for table customer_invoice_profile.
type CustomerInvoiceProfile struct {
	Id                   int64      `json:"id"                   orm:"id"                      description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"               description:""`
	ProfileNo            string     `json:"profileNo"            orm:"profile_no"              description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"             description:""`
	TitleTypeCode        string     `json:"titleTypeCode"        orm:"title_type_code"         description:""`
	InvoiceTitle         string     `json:"invoiceTitle"         orm:"invoice_title"           description:""`
	TaxRegistrationNo    string     `json:"taxRegistrationNo"    orm:"tax_registration_no"     description:""`
	RegisteredAddress    string     `json:"registeredAddress"    orm:"registered_address"      description:""`
	RegisteredPhone      string     `json:"registeredPhone"      orm:"registered_phone"        description:""`
	BankName             string     `json:"bankName"             orm:"bank_name"               description:""`
	BankAccountMasked    string     `json:"bankAccountMasked"    orm:"bank_account_masked"     description:""`
	BankAccountSecretRef string     `json:"bankAccountSecretRef" orm:"bank_account_secret_ref" description:"Opaque core secret-envelope reference; never returned by ordinary APIs or logs"`
	IsDefault            bool       `json:"isDefault"            orm:"is_default"              description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"             description:""`
	Version              int64      `json:"version"              orm:"version"                 description:""`
	CreatedBy            int        `json:"createdBy"            orm:"created_by"              description:""`
	UpdatedBy            int        `json:"updatedBy"            orm:"updated_by"              description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"              description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"              description:""`
}
