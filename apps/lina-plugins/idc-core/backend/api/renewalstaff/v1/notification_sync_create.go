// This file creates a durable renewal summary intent for the actual current staff.

package v1

import "github.com/gogf/gf/v2/frame/g"

// NotificationSyncCreateReq never accepts a recipient, permission or customer ID.
type NotificationSyncCreateReq struct {
	g.Meta `path:"/admin/server-accessory-renewal-notification-syncs" method:"post" tags:"IDC Accessory Renewals" summary:"Sync current staff renewal notification" dc:"Create an idempotent notification intent for current open renewal work. The recipient is derived from the authenticated staff identity." permission:"idc:admin:resource:accessory-renewal:handle"`
	Locale string `json:"locale" d:"en-US" v:"in:zh-CN,en-US" dc:"Notification locale: zh-CN or en-US" eg:"zh-CN"`
}

// NotificationSyncCreateRes distinguishes persisted intent from final delivery.
type NotificationSyncCreateRes struct {
	Total   int    `json:"total" dc:"Exact current count of accessible open work items" eg:"1"`
	EventNo string `json:"eventNo" dc:"Durable event intent number; empty when no work remains, not delivery confirmation" eg:"evt_001"`
}
