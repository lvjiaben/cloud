// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateField is the golang structure of table plugin_idc_compliance_contract_template_field for DAO operations like Where/Data.
type ContractTemplateField struct {
	g.Meta        `orm:"table:plugin_idc_compliance_contract_template_field, do:true"`
	Id            any        //
	TenantId      any        //
	FieldNo       any        //
	FieldSetNo    any        //
	TemplateNo    any        //
	VersionNo     any        //
	SequenceNo    any        //
	FieldTypeCode any        //
	PartyCode     any        //
	VariableCode  *string    //
	PageNo        any        //
	PositionXPpm  any        //
	PositionYPpm  any        //
	WidthPpm      any        //
	HeightPpm     any        //
	CreatedAt     *time.Time //
}
