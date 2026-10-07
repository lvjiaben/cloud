// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionPayoutRecovery is the golang structure for table sales_commission_payout_recovery.
type SalesCommissionPayoutRecovery struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	RecoveryNo     string     `json:"recoveryNo"     orm:"recovery_no"     description:""`
	PayoutItemNo   string     `json:"payoutItemNo"   orm:"payout_item_no"  description:""`
	AttemptNo      int        `json:"attemptNo"      orm:"attempt_no"      description:""`
	ActionCode     string     `json:"actionCode"     orm:"action_code"     description:""`
	ResultCode     string     `json:"resultCode"     orm:"result_code"     description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"     description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest" description:""`
	NextAttemptAt  *time.Time `json:"nextAttemptAt"  orm:"next_attempt_at" description:""`
	RecordedAt     *time.Time `json:"recordedAt"     orm:"recorded_at"     description:""`
}
