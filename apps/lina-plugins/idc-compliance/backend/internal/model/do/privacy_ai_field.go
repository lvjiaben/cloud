// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyAiField is the golang structure of table plugin_idc_compliance_privacy_ai_field for DAO operations like Where/Data.
type PrivacyAiField struct {
	g.Meta       `orm:"table:plugin_idc_compliance_privacy_ai_field, do:true"`
	Id           any        //
	TenantId     any        //
	AssessmentNo any        //
	FieldCode    any        //
	CreatedAt    *time.Time //
}
