// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PurchaseServerLogin is the golang structure of table plugin_idc_core_purchase_server_login for DAO operations like Where/Data.
type PurchaseServerLogin struct {
	g.Meta                `orm:"table:plugin_idc_core_purchase_server_login, do:true"`
	Id                    any        //
	TenantId              any        //
	ServerLoginToken      any        //
	CustomerNo            any        //
	UserId                any        //
	ProductVersionNo      any        //
	ConfigurationRevision any        //
	CartItemNo            any        //
	Hostname              any        //
	PasswordSecretRef     any        // Opaque resource-channel-scoped secretcap handle, never plaintext
	ChannelNo             any        //
	CreatedAt             *time.Time //
}
