// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TrialQualificationScope is the golang structure of table plugin_idc_core_trial_qualification_scope for DAO operations like Where/Data.
type TrialQualificationScope struct {
	g.Meta           `orm:"table:plugin_idc_core_trial_qualification_scope, do:true"`
	Id               any        //
	TenantId         any        //
	ScopeNo          any        //
	CustomerNo       any        //
	ProductNo        any        //
	HeldQuantity     any        //
	ConsumedQuantity any        //
	Version          any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
