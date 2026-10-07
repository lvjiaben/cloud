// This file defines the contract for removing the current staff member's own WeCom binding.

package v1

import "github.com/gogf/gf/v2/frame/g"

// WeComBindingRemovalCreateReq carries the concurrency and confirmation evidence for removal.
type WeComBindingRemovalCreateReq struct {
	g.Meta          `path:"/staff/notifications/wechat-work/binding-removals" method:"post" tags:"IDC Staff WeCom Binding" summary:"Remove my WeCom binding" dc:"Starts recoverable revocation of only the current active direct staff member's own binding after recent full login and explicit confirmation."`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected binding version" eg:"2"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"wecom-unbind-20260815-001"`
	Confirmed       bool   `json:"confirmed" v:"required" dc:"Explicit confirmation" eg:"true"`
}

// WeComBindingRemovalCreateRes returns the safe binding state after removal starts.
type WeComBindingRemovalCreateRes struct {
	Item *WeComBindingItem `json:"item" dc:"Binding status after removal starts" eg:"{}"`
}
