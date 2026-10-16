// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionOfflinePayout is the golang structure for table sales_commission_offline_payout.
type SalesCommissionOfflinePayout struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	PayoutNo          string     `json:"payoutNo"          orm:"payout_no"          description:""`
	AccountNo         string     `json:"accountNo"         orm:"account_no"         description:""`
	CurrencyCode      string     `json:"currencyCode"      orm:"currency_code"      description:""`
	AmountMinor       int64      `json:"amountMinor"       orm:"amount_minor"       description:""`
	EvidenceReference string     `json:"evidenceReference" orm:"evidence_reference" description:""`
	PaidAt            *time.Time `json:"paidAt"            orm:"paid_at"            description:""`
	RecordedBy        int        `json:"recordedBy"        orm:"recorded_by"        description:""`
	RequestDigest     string     `json:"requestDigest"     orm:"request_digest"     description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
}
