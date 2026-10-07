package v1

import "github.com/gogf/gf/v2/frame/g"

type ProductFulfillmentTaskReviewApprovalReq struct {
	g.Meta `path:"/admin/product-fulfillment/tasks/{intentNo}/review-approvals" method:"post" tags:"IDC Product Fulfillment" summary:"Approve and start product fulfillment" dc:"After settlement, append an independently authorized approval and asynchronously start the frozen plugin workflow." permission:"idc:admin:product-fulfillment:review-approve"`
	ProductFulfillmentActionReq
}

type ProductFulfillmentTaskReviewApprovalRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Committed current fulfillment task projection"`
}
