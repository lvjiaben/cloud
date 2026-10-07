package v1

import "github.com/gogf/gf/v2/frame/g"

type DeliveryResendEligibilityGetReq struct {
	g.Meta `path:"/admin/notifications/deliveries/{taskNo}/resend-eligibility" method:"get" tags:"IDC Notification Deliveries" summary:"Check manual resend eligibility" dc:"Re-evaluates current business-object visibility, channel safety, cooldown, resend limit, recipient preference, and recent-login gates without changing state." permission:"idc:admin:notification:channel:manage"`
	TaskNo string `json:"taskNo" in:"path" v:"required|max-length:80" dc:"Opaque delivery task number to evaluate" eg:"ntask_01jnotify000000000000000001"`
}
type DeliveryResendEligibilityGetRes struct {
	Item *ResendEligibilityItem `json:"item" dc:"Current manual-resend eligibility projection" eg:"{}"`
}
