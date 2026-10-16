// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionRecoveryOffset is the golang structure for table sales_commission_recovery_offset.
type SalesCommissionRecoveryOffset struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	OffsetNo     string     `json:"offsetNo"     orm:"offset_no"     description:""`
	RecoveryNo   string     `json:"recoveryNo"   orm:"recovery_no"   description:""`
	CommissionNo string     `json:"commissionNo" orm:"commission_no" description:""`
	AccountNo    string     `json:"accountNo"    orm:"account_no"    description:""`
	AmountMinor  int64      `json:"amountMinor"  orm:"amount_minor"  description:""`
	EntryNo      string     `json:"entryNo"      orm:"entry_no"      description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
