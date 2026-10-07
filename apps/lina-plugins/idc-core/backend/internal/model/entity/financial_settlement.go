// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialSettlement is the golang structure for table financial_settlement.
type FinancialSettlement struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	SettlementNo           string     `json:"settlementNo"           orm:"settlement_no"            description:""`
	OrderNo                string     `json:"orderNo"                orm:"order_no"                 description:""`
	InvoiceNo              string     `json:"invoiceNo"              orm:"invoice_no"               description:""`
	CustomerNo             string     `json:"customerNo"             orm:"customer_no"              description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"            description:""`
	BasisCode              string     `json:"basisCode"              orm:"basis_code"               description:"Mutually exclusive funded, credit, zero-amount, full-discount, or trial basis"`
	TotalAmountMinor       int64      `json:"totalAmountMinor"       orm:"total_amount_minor"       description:""`
	PaidAmountMinor        int64      `json:"paidAmountMinor"        orm:"paid_amount_minor"        description:""`
	CreditAmountMinor      int64      `json:"creditAmountMinor"      orm:"credit_amount_minor"      description:""`
	SourceCount            int        `json:"sourceCount"            orm:"source_count"             description:""`
	IdentityEvidenceDigest string     `json:"identityEvidenceDigest" orm:"identity_evidence_digest" description:"Digest of the re-read customer identity authority without personal material"`
	SourceSnapshotDigest   string     `json:"sourceSnapshotDigest"   orm:"source_snapshot_digest"   description:""`
	ContentDigest          string     `json:"contentDigest"          orm:"content_digest"           description:""`
	SettledAt              *time.Time `json:"settledAt"              orm:"settled_at"               description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
}
