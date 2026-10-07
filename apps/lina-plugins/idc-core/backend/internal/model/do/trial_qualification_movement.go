// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TrialQualificationMovement is the golang structure of table plugin_idc_core_trial_qualification_movement for DAO operations like Where/Data.
type TrialQualificationMovement struct {
	g.Meta             `orm:"table:plugin_idc_core_trial_qualification_movement, do:true"`
	Id                 any        //
	TenantId           any        //
	MovementNo         any        //
	ScopeNo            any        //
	CustomerNo         any        //
	ProductNo          any        //
	OrderNo            any        //
	MovementTypeCode   any        //
	Quantity           any        //
	HeldBefore         any        //
	HeldAfter          any        //
	ConsumedBefore     any        //
	ConsumedAfter      any        //
	ScopeVersionBefore any        //
	ScopeVersionAfter  any        //
	EvidenceDigest     any        //
	ContentDigest      any        //
	CreatedAt          *time.Time //
}
