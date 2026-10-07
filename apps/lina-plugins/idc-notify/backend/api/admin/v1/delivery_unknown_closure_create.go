package v1

import "github.com/gogf/gf/v2/frame/g"

type DeliveryUnknownClosureCreateReq struct {
	g.Meta          `path:"/admin/notifications/deliveries/{taskNo}/unknown-closures" method:"post" tags:"IDC Notification Deliveries" summary:"Close unresolved delivery as still unknown" dc:"Closes a manual-review delivery as unresolved only after current object visibility, recent-login, explicit confirmation, and optimistic-version checks." permission:"idc:admin:notification:channel:manage"`
	TaskNo          string `json:"taskNo" in:"path" v:"required|max-length:80" dc:"Opaque manual-review delivery task number" eg:"ntask_01jnotify000000000000000001"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected delivery version for optimistic concurrency" eg:"4"`
	OperationNo     string `json:"operationNo" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency operation number for this closure" eg:"unknown-close-20260815-001"`
	Reason          string `json:"reason" v:"required|max-length:500" dc:"Administrator reason recorded in the manual-closure audit evidence" eg:"Provider inquiry deadline passed without a definitive result"`
	Confirmed       bool   `json:"confirmed" v:"required" dc:"Explicit confirmation that the administrator intends to close this delivery as unresolved" eg:"true"`
}
type DeliveryUnknownClosureCreateRes struct {
	Item *DeliveryItem `json:"item" dc:"Delivery projection after manual unresolved closure" eg:"{}"`
}
