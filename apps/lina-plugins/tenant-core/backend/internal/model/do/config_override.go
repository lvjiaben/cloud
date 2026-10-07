// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ConfigOverride is the golang structure of table plugin_tenant_core_config_override for DAO operations like Where/Data.
type ConfigOverride struct {
	g.Meta      `orm:"table:plugin_tenant_core_config_override, do:true"`
	Id          any        //
	TenantId    any        //
	ConfigKey   any        //
	ConfigValue any        //
	Enabled     any        //
	CreatedAt   *time.Time //
	UpdatedAt   *time.Time //
	DeletedAt   *time.Time //
}
