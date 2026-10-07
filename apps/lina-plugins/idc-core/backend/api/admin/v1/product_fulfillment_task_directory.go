package v1

import "github.com/gogf/gf/v2/frame/g"

type ProductFulfillmentTaskListReq struct {
	Page                int `json:"page" v:"min:0" dc:"Optional one-based page number; cannot be combined with cursor" eg:"1"`
	g.Meta              `path:"/admin/product-fulfillment/tasks" method:"get" tags:"IDC Product Fulfillment" summary:"List product fulfillment tasks" dc:"Return one bounded encrypted-keyset page after exact permission and data-scope enforcement." permission:"idc:admin:product-fulfillment:task:list"`
	OrderNo             string `json:"orderNo" v:"max-length:40" dc:"Optional exact order number"`
	CustomerNo          string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number"`
	FulfillmentModeCode string `json:"fulfillmentModeCode" v:"in:,manual,approval_then_plugin,payment_confirmed_plugin" dc:"Optional exact frozen fulfillment mode"`
	StatusCode          string `json:"statusCode" v:"in:,awaiting_review,processing,completed,not_completed,cancelled" dc:"Optional exact safe workflow status available after a fulfillment intent is created"`
	Cursor              string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page"`
	Limit               int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum tasks returned"`
}

type ProductFulfillmentTaskListRes struct {
	Total      int                              `json:"total" dc:"Total records matching the current filters and authorized data scope" eg:"0"`
	Tasks      []*ProductFulfillmentTaskSummary `json:"tasks" dc:"Bounded product fulfillment task summaries"`
	NextCursor string                           `json:"nextCursor" dc:"Encrypted next-page cursor, or empty"`
}

type ProductFulfillmentTaskDetailReq struct {
	g.Meta   `path:"/admin/product-fulfillment/tasks/{intentNo}" method:"get" tags:"IDC Product Fulfillment" summary:"Get product fulfillment task" dc:"Return one bounded safe task aggregate after exact permission and data-scope enforcement." permission:"idc:admin:product-fulfillment:task:query"`
	IntentNo string `json:"intentNo" in:"path" v:"required|max-length:40" dc:"Stable fulfillment intent number"`
}

type ProductFulfillmentTaskDetailRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Current safe product fulfillment task aggregate"`
}
