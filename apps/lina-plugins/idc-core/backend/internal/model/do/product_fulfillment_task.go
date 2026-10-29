// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProductFulfillmentTask is the golang structure of table plugin_idc_core_product_fulfillment_task for DAO operations like Where/Data.
type ProductFulfillmentTask struct {
	g.Meta                 `orm:"table:plugin_idc_core_product_fulfillment_task, do:true"`
	TenantId               any        //
	TaskNo                 any        //
	IntentNo               any        //
	StatusCode             any        //
	PhaseCode              any        //
	AttemptCount           any        //
	QueryAttemptCount      any        //
	SubmitAttemptCount     any        //
	MaximumAttempts        any        //
	RetryBaseSeconds       any        //
	NextAttemptAt          *time.Time //
	LeaseToken             any        //
	LeaseExpiresAt         *time.Time //
	ExternalReference      any        //
	LastResultCode         any        //
	TakeoverRequested      any        //
	CancelRequested        any        //
	Version                any        //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
	UpstreamDeliveries     any        //
	RecoveryCount          any        //
	RecoveryQueryBaseline  any        //
	RecoverySubmitBaseline any        //
}
