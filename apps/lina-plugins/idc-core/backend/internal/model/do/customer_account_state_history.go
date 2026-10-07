// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerAccountStateHistory is the golang structure of table plugin_idc_core_customer_account_state_history for DAO operations like Where/Data.
type CustomerAccountStateHistory struct {
	g.Meta               `orm:"table:plugin_idc_core_customer_account_state_history, do:true"`
	Id                   any        //
	TenantId             any        //
	TransitionNo         any        //
	CustomerNo           any        //
	FromStatusCode       any        //
	ToStatusCode         any        //
	ReasonCode           any        //
	SourceCode           any        //
	SourceNo             any        //
	ActorUserId          any        //
	SubjectVersion       any        //
	IdempotencyKeyDigest any        //
	RequestFingerprint   any        //
	OccurredAt           *time.Time //
	CreatedAt            *time.Time //
}
