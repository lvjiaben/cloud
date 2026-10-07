// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerContactPreference is the golang structure of table plugin_idc_core_customer_contact_preference for DAO operations like Where/Data.
type CustomerContactPreference struct {
	g.Meta      `orm:"table:plugin_idc_core_customer_contact_preference, do:true"`
	Id          any        //
	TenantId    any        //
	CustomerNo  any        //
	ContactNo   any        //
	TopicCode   any        //
	ChannelCode any        //
	Enabled     any        //
	Version     any        //
	CreatedAt   *time.Time //
	UpdatedAt   *time.Time //
}
