// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionOfflinePayoutItem is the golang structure for table sales_commission_offline_payout_item.
type SalesCommissionOfflinePayoutItem struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	PayoutNo     string     `json:"payoutNo"     orm:"payout_no"     description:""`
	CommissionNo string     `json:"commissionNo" orm:"commission_no" description:""`
	AmountMinor  int64      `json:"amountMinor"  orm:"amount_minor"  description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
