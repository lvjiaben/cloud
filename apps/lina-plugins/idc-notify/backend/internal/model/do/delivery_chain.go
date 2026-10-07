// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryChain is the golang structure of table plugin_idc_notify_delivery_chain for DAO operations like Where/Data.
type DeliveryChain struct {
	g.Meta             `orm:"table:plugin_idc_notify_delivery_chain, do:true"`
	TenantId           any        //
	ChainNo            any        //
	RootTaskNo         any        //
	TailTaskNo         any        //
	ManualResendCount  any        //
	LastManualResendAt any        //
	Version            any        //
	CreatedAt          *time.Time //
	UpdatedAt          *time.Time //
}
