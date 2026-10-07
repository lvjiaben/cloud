package v1

import "github.com/gogf/gf/v2/frame/g"

type ProductFulfillmentTaskTakeoverReq struct {
	g.Meta `path:"/admin/product-fulfillment/tasks/{intentNo}/takeovers" method:"post" tags:"IDC Product Fulfillment" summary:"Take over product fulfillment" dc:"Stop later automated execution and append the authorized handler and reason without forcing success, failure, or retry." permission:"idc:admin:product-fulfillment:takeover"`
	ProductFulfillmentActionReq
}

type ProductFulfillmentTaskTakeoverRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Committed current fulfillment task projection"`
}
