// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// WechatCallbackEvent is the golang structure of table plugin_idc_notify_wechat_callback_event for DAO operations like Where/Data.
type WechatCallbackEvent struct {
	g.Meta             `orm:"table:plugin_idc_notify_wechat_callback_event, do:true"`
	TenantId           any        //
	CallbackNo         any        //
	ProviderMessageRef any        //
	EventDigest        any        //
	ResultClass        any        //
	AppliedTaskNo      any        //
	ReceivedAt         *time.Time //
}
