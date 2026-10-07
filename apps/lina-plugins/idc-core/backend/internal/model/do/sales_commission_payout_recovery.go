// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutRecovery is the golang structure of table plugin_idc_core_sales_commission_payout_recovery for DAO operations like Where/Data.
type SalesCommissionPayoutRecovery struct {
	g.Meta         `orm:"table:plugin_idc_core_sales_commission_payout_recovery, do:true"`
	Id             any        //
	TenantId       any        //
	RecoveryNo     any        //
	PayoutItemNo   any        //
	AttemptNo      any        //
	ActionCode     any        //
	ResultCode     any        //
	ReasonCode     any        //
	EvidenceDigest any        //
	NextAttemptAt  *time.Time //
	RecordedAt     *time.Time //
}
