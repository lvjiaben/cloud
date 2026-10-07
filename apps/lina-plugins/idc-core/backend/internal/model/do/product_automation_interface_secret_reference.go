// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProductAutomationInterfaceSecretReference is the golang structure of table plugin_idc_core_product_automation_interface_secret_reference for DAO operations like Where/Data.
type ProductAutomationInterfaceSecretReference struct {
	g.Meta                  `orm:"table:plugin_idc_core_product_automation_interface_secret_reference, do:true"`
	Id                      any        //
	TenantId                any        //
	ConfigurationRevisionId any        //
	FieldKey                any        //
	SecretRef               any        //
	Configured              any        //
	MaskedTail              any        //
	SecretUpdatedAt         *time.Time //
	CreatedAt               *time.Time //
}
