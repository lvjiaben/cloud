// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingAdjustmentApproval is the golang structure of table plugin_idc_core_billing_adjustment_approval for DAO operations like Where/Data.
type BillingAdjustmentApproval struct {
	g.Meta              `orm:"table:plugin_idc_core_billing_adjustment_approval, do:true"`
	Id                  any        //
	TenantId            any        //
	ApprovalNo          any        //
	AdjustmentNo        any        //
	RequesterIdSnapshot any        //
	ReviewerId          any        //
	DecisionCode        any        //
	CommentDigest       any        //
	EvidenceDigest      any        //
	DecidedAt           *time.Time //
	CreatedAt           *time.Time //
}
