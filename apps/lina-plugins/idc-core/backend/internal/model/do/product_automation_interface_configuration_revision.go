// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProductAutomationInterfaceConfigurationRevision is the golang structure of table plugin_idc_core_product_automation_interface_configuration_revision for DAO operations like Where/Data.
type ProductAutomationInterfaceConfigurationRevision struct {
	g.Meta                             `orm:"table:plugin_idc_core_product_automation_interface_configuration_revision, do:true"`
	Id                                 any        //
	TenantId                           any        //
	InterfaceInstanceNo                any        //
	Revision                           any        //
	SchemaVersion                      any        //
	DeclarationSnapshot                any        //
	PublicValueSnapshot                any        //
	DeclarationDigest                  any        //
	ValueDigest                        any        //
	SecretFieldCount                   any        //
	CreatedBy                          any        //
	CreatedAt                          *time.Time //
	ResourceOwnerConfigurationRevision any        // Immutable resource-owner endpoint/account/secret-reference revision; empty for non-resource interfaces / 资源拥有方不可变接口配置修订，非资源接口为空
}
