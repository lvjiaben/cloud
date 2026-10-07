// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateFieldSet is the golang structure of table plugin_idc_compliance_contract_template_field_set for DAO operations like Where/Data.
type ContractTemplateFieldSet struct {
	g.Meta                   `orm:"table:plugin_idc_compliance_contract_template_field_set, do:true"`
	Id                       any        //
	TenantId                 any        //
	FieldSetNo               any        //
	TemplateNo               any        //
	VersionNo                any        //
	ContentRevisionNo        any        //
	CommandIdempotencyDigest any        //
	CommandFingerprint       any        //
	CreatedByUserId          any        //
	CreatedAt                *time.Time //
}
