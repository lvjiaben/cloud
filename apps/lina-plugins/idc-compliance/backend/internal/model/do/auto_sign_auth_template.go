// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// AutoSignAuthTemplate is the golang structure of table plugin_idc_compliance_auto_sign_auth_template for DAO operations like Where/Data.
type AutoSignAuthTemplate struct {
	g.Meta          `orm:"table:plugin_idc_compliance_auto_sign_auth_template, do:true"`
	Id              any        //
	TenantId        any        //
	AuthorizationNo any        //
	TemplateNo      any        //
	VersionNo       any        //
	CreatedAt       *time.Time //
}
