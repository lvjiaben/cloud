// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditExposure is the golang structure for table credit_exposure.
type CreditExposure struct {
	Id                       int64      `json:"id"                       orm:"id"                          description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                   description:""`
	ExposureNo               string     `json:"exposureNo"               orm:"exposure_no"                 description:""`
	CreditAccountNo          string     `json:"creditAccountNo"          orm:"credit_account_no"           description:""`
	CustomerNo               string     `json:"customerNo"               orm:"customer_no"                 description:""`
	CurrencyCode             string     `json:"currencyCode"             orm:"currency_code"               description:""`
	SourceTypeCode           string     `json:"sourceTypeCode"           orm:"source_type_code"            description:""`
	SourceReference          string     `json:"sourceReference"          orm:"source_reference"            description:""`
	OrderNo                  *string    `json:"orderNo"                  orm:"order_no"                    description:""`
	InvoiceNo                *string    `json:"invoiceNo"                orm:"invoice_no"                  description:""`
	OriginalPrincipalMinor   int64      `json:"originalPrincipalMinor"   orm:"original_principal_minor"    description:""`
	UnbilledPrincipalMinor   int64      `json:"unbilledPrincipalMinor"   orm:"unbilled_principal_minor"    description:""`
	BilledPrincipalMinor     int64      `json:"billedPrincipalMinor"     orm:"billed_principal_minor"      description:""`
	RepaidPrincipalMinor     int64      `json:"repaidPrincipalMinor"     orm:"repaid_principal_minor"      description:""`
	ReleasedPrincipalMinor   int64      `json:"releasedPrincipalMinor"   orm:"released_principal_minor"    description:""`
	WrittenOffPrincipalMinor int64      `json:"writtenOffPrincipalMinor" orm:"written_off_principal_minor" description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                 description:""`
	SourceSnapshotDigest     string     `json:"sourceSnapshotDigest"     orm:"source_snapshot_digest"      description:""`
	ContentDigest            string     `json:"contentDigest"            orm:"content_digest"              description:""`
	Version                  int64      `json:"version"                  orm:"version"                     description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                  description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                  description:""`
}
