package v1

import "github.com/gogf/gf/v2/frame/g"

type ProductFulfillmentTaskManualConclusionReq struct {
	g.Meta      `path:"/admin/product-fulfillment/tasks/{intentNo}/manual-conclusions" method:"post" tags:"IDC Product Fulfillment" summary:"Conclude manual product fulfillment" dc:"Append an authorized completed or unable-to-complete manual outcome without overwriting automated task or supplier history." permission:"idc:admin:product-fulfillment:manual-conclude"`
	OutcomeCode string `json:"outcomeCode" v:"required|in:completed,unable_to_complete" dc:"Exact manual outcome"`
	ProductFulfillmentManualActionReq
}

type ProductFulfillmentTaskManualConclusionRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Committed current fulfillment task projection"`
}
