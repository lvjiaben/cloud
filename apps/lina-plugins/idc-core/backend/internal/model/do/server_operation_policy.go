// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// ServerOperationPolicy is the golang structure of table plugin_idc_core_server_operation_policy for DAO operations like Where/Data.
type ServerOperationPolicy struct {
	g.Meta                `orm:"table:plugin_idc_core_server_operation_policy, do:true"`
	TenantId              any //
	RequireHighRiskReauth any //
	Version               any //
	CreatedAt             any //
	UpdatedAt             any //
	AllowCustomerTransfer any //
}
