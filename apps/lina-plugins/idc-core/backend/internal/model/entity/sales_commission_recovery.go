// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionRecovery is the golang structure for table sales_commission_recovery.
type SalesCommissionRecovery struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	RecoveryNo   string     `json:"recoveryNo"   orm:"recovery_no"   description:""`
	AccountNo    string     `json:"accountNo"    orm:"account_no"    description:""`
	CommissionNo string     `json:"commissionNo" orm:"commission_no" description:""`
	RefundNo     string     `json:"refundNo"     orm:"refund_no"     description:""`
	ReversalNo   string     `json:"reversalNo"   orm:"reversal_no"   description:""`
	AmountMinor  int64      `json:"amountMinor"  orm:"amount_minor"  description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
