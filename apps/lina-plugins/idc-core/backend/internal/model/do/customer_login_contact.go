// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerLoginContact is the golang structure of table plugin_idc_core_customer_login_contact for DAO operations like Where/Data.
type CustomerLoginContact struct {
	g.Meta             `orm:"table:plugin_idc_core_customer_login_contact, do:true"`
	TenantId           any //
	UserId             any //
	EmailDigest        any //
	PhoneDigest        any //
	IdentityCiphertext any //
	Revision           any //
	CreatedAt          any //
	UpdatedAt          any //
}
