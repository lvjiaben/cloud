package v1

import "github.com/gogf/gf/v2/frame/g"

type ProductFulfillmentTaskReviewRejectionReq struct {
	g.Meta `path:"/admin/product-fulfillment/tasks/{intentNo}/review-rejections" method:"post" tags:"IDC Product Fulfillment" summary:"Reject product fulfillment review" dc:"Append an independently authorized rejection and move only this intent to manual handling without cancelling or refunding the order." permission:"idc:admin:product-fulfillment:review-reject"`
	ProductFulfillmentActionReq
}

type ProductFulfillmentTaskReviewRejectionRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Committed current fulfillment task projection"`
}
