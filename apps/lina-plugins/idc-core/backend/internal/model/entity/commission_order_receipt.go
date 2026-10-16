// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommissionOrderReceipt is the golang structure for table commission_order_receipt.
type CommissionOrderReceipt struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	OrderNo      string     `json:"orderNo"      orm:"order_no"      description:""`
	SettlementNo string     `json:"settlementNo" orm:"settlement_no" description:""`
	SourceDigest string     `json:"sourceDigest" orm:"source_digest" description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	InvoiceNo    string     `json:"invoiceNo"    orm:"invoice_no"    description:""`
}
