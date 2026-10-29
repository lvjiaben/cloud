// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProductFulfillmentCommand is the golang structure of table plugin_idc_core_product_fulfillment_command for DAO operations like Where/Data.
type ProductFulfillmentCommand struct {
	g.Meta                `orm:"table:plugin_idc_core_product_fulfillment_command, do:true"`
	TenantId              any        //
	CommandNo             any        //
	IntentNo              any        //
	ActionCode            any        //
	IdempotencyKey        any        //
	RequestDigest         any        //
	ReasonCode            any        //
	ReasonLabelZhCn       any        //
	ReasonLabelEnUs       any        //
	SafeNote              any        //
	ActorUserId           any        //
	ResultingVersion      any        //
	CreatedAt             *time.Time //
	RecoveryTaskVersion   any        //
	RecoveryQueryCount    any        //
	RecoverySubmitCount   any        //
	RecoveryConfiguration any        //
}
