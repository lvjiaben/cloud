// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingAdjustmentRequest is the golang structure of table plugin_idc_core_billing_adjustment_request for DAO operations like Where/Data.
type BillingAdjustmentRequest struct {
	g.Meta                    `orm:"table:plugin_idc_core_billing_adjustment_request, do:true"`
	Id                        any        //
	TenantId                  any        //
	AdjustmentNo              any        //
	CustomerNo                any        //
	InvoiceNo                 *string    //
	AccountNo                 *string    //
	AccountTypeCode           *string    //
	CurrencyCode              any        //
	AdjustmentTypeCode        any        //
	AmountMinor               any        //
	ReasonCode                any        //
	EvidenceDigest            any        //
	IdempotencyDigest         any        //
	RequestDigest             any        //
	ApprovalRequired          any        //
	StatusCode                any        //
	RequestedBy               any        //
	LedgerTransactionNo       *string    //
	LedgerTransactionTypeCode *string    //
	PostedAt                  *time.Time //
	Version                   any        //
	CreatedAt                 *time.Time //
	UpdatedAt                 *time.Time //
	ProofReference            any        // Verified receipt evidence or the exact marketing value-grant business number
	ReceivingAccountReference any        //
	BusinessPurposeCode       any        //
	PolicyNo                  any        //
	PolicyVersion             any        //
	CreditStatementNo         any        //
}
