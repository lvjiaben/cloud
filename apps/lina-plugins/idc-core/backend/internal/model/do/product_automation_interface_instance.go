// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProductAutomationInterfaceInstance is the golang structure of table plugin_idc_core_product_automation_interface_instance for DAO operations like Where/Data.
type ProductAutomationInterfaceInstance struct {
	g.Meta               `orm:"table:plugin_idc_core_product_automation_interface_instance, do:true"`
	Id                   any        //
	TenantId             any        //
	InterfaceInstanceNo  any        //
	DisplayName          any        //
	PluginId             any        //
	ExtensionPointCode   any        //
	PluginReleaseVersion any        //
	ContractVersion      any        //
	AuthorizedMethods    []string   //
	ReleaseDigest        any        //
	DeclarationDigest    any        //
	NewBusinessState     any        //
	EverReferenced       any        //
	Version              any        //
	CreatedBy            any        //
	CreatedAt            *time.Time //
	ChangedBy            any        //
	ChangedAt            *time.Time //
	InterfaceOriginCode  any        //
	ResourceOwnerId      any        //
	ResourceChannelRef   any        //
}
