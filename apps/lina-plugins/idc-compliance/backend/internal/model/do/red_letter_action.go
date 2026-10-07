// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterAction is the golang structure of table plugin_idc_compliance_red_letter_action for DAO operations like Where/Data.
type RedLetterAction struct {
	g.Meta             `orm:"table:plugin_idc_compliance_red_letter_action, do:true"`
	Id                 any        //
	TenantId           any        //
	ActionNo           any        //
	TaskNo             any        //
	ActionCode         any        //
	IdempotencyDigest  any        //
	RequestFingerprint any        //
	ActorTypeCode      any        //
	ActorUserId        any        //
	BeforeStatusCode   any        //
	AfterStatusCode    any        //
	BeforeVersion      any        //
	AfterVersion       any        //
	EvidenceNo         any        //
	CreatedAt          *time.Time //
}
