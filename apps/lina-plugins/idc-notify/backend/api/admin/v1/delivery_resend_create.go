package v1

import "github.com/gogf/gf/v2/frame/g"

type DeliveryResendCreateReq struct {
	g.Meta          `path:"/admin/notifications/deliveries/{taskNo}/resends" method:"post" tags:"IDC Notification Deliveries" summary:"Create manual resend task" dc:"Creates one linear manual-resend successor only after current object visibility, channel safety, cooldown, limit, and current-session checks pass." permission:"idc:admin:notification:channel:manage"`
	TaskNo          string `json:"taskNo" in:"path" v:"required|max-length:80" dc:"Opaque failed delivery task number at the current resend-chain tail" eg:"ntask_01jnotify000000000000000001"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected source delivery version for optimistic concurrency" eg:"4"`
	OperationNo     string `json:"operationNo" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency operation number for this manual resend" eg:"resend-operation-0001"`
	Reason          string `json:"reason" v:"required|max-length:500" dc:"Administrator reason recorded in the manual-resend audit evidence" eg:"Provider confirmed that the recipient did not receive the message"`
	Confirmed       bool   `json:"confirmed" v:"required" dc:"Explicit confirmation that the administrator intends to resend" eg:"true"`
}
type DeliveryResendCreateRes struct {
	Item *DeliveryItem `json:"item" dc:"New queued successor delivery projection" eg:"{}"`
}
