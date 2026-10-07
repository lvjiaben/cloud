// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoice is the golang structure of table plugin_idc_compliance_tax_invoice for DAO operations like Where/Data.
type TaxInvoice struct {
	g.Meta                       `orm:"table:plugin_idc_compliance_tax_invoice, do:true"`
	Id                           any        //
	TenantId                     any        //
	TaxInvoiceNo                 any        //
	RequestSha256                any        //
	CustomerNo                   any        //
	PrincipalNo                  any        //
	SellerSnapshotRef            any        // Private immutable seller snapshot for the installation merchant, never the software platform
	SellerSnapshotSha256         any        //
	BuyerSnapshotRef             any        //
	BuyerSnapshotSha256          any        //
	InvoiceTypeCode              any        // Customer requests use pending_confirmation; an authorized reviewer selects a supported type later
	StatusCode                   any        //
	TaxCurrencyCode              any        //
	GrossAmountMinor             any        //
	NetAmountMinor               any        //
	TaxAmountMinor               any        //
	BillingCurrencyCode          any        //
	ExchangeRateNumerator        *int64     //
	ExchangeRateDenominator      *int64     //
	ExchangeRateDate             *time.Time //
	ExchangeRateSourceCode       *string    //
	ProviderCode                 *string    //
	ExternalInvoiceRef           *string    //
	MaskedInvoiceNumber          any        //
	InvoiceNumberLookupDigest    *string    // Keyed lookup digest for exact invoice-number reconciliation; raw values and unsalted public hashes are forbidden
	SubmittedAt                  *time.Time //
	IssuedAt                     *time.Time //
	DeliveredAt                  *time.Time //
	CanceledAt                   *time.Time //
	Version                      any        //
	CreatedAt                    *time.Time //
	UpdatedAt                    *time.Time //
	DecisionReasonCode           any        //
	DecisionInternalNote         any        // Optional administrator-only note; customer projections must never expose it
	ReviewedByUserId             *int       //
	ReviewedAt                   *time.Time //
	ProviderNotIssuedConfirmedAt *time.Time // Authoritative positive evidence required before safely canceling a failed provider attempt
}
