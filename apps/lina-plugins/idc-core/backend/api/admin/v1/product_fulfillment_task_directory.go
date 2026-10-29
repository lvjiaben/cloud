package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

type ProductFulfillmentTaskListReq struct {
	Page                int `json:"page" v:"min:0" dc:"Optional one-based page number; cannot be combined with cursor" eg:"1"`
	g.Meta              `path:"/admin/product-fulfillment/tasks" method:"get" tags:"IDC Product Fulfillment" summary:"List product fulfillment tasks" dc:"Return one bounded encrypted-keyset page after exact permission and data-scope enforcement." permission:"idc:admin:product-fulfillment:task:list"`
	OrderNo             string `json:"orderNo" v:"max-length:40" dc:"Optional exact order number"`
	CustomerNo          string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number"`
	FulfillmentModeCode string `json:"fulfillmentModeCode" v:"in:,manual,approval_then_plugin,payment_confirmed_plugin" dc:"Optional exact frozen fulfillment mode"`
	StatusCode          string `json:"statusCode" v:"in:,awaiting_review,processing,manual_processing,completed,not_completed,cancelled" dc:"Optional exact safe workflow status available after a fulfillment intent is created"`
	Cursor              string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page"`
	Limit               int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum tasks returned"`
}

type ProductFulfillmentTaskListRes struct {
	Total      int                              `json:"total" dc:"Total records matching the current filters and authorized data scope" eg:"0"`
	Tasks      []*ProductFulfillmentTaskSummary `json:"tasks" dc:"Bounded product fulfillment task summaries"`
	NextCursor string                           `json:"nextCursor" dc:"Encrypted next-page cursor, or empty"`
}

type ProductFulfillmentTaskDetailReq struct {
	IncludeRecoveryConfiguration bool   `json:"includeRecoveryConfiguration" in:"query" dc:"Explicitly include operational correction choices; requires independent recovery permission and may perform bounded read-only upstream queries" eg:"false"`
	RecoveryInterfaceNo          string `json:"recoveryInterfaceNo" in:"query" v:"max-length:64" dc:"Optional compatible upstream selected in the correction editor" eg:"pfi_0123456789abcdef0123456789abcdef"`
	RecoveryRegion               string `json:"recoveryRegion" in:"query" v:"max-length:128" dc:"Optional selected native region used to filter nodes" eg:"1"`
	RecoveryLine                 string `json:"recoveryLine" in:"query" v:"max-length:128" dc:"Optional selected native node" eg:"5"`
	RecoveryOperatingSystem      string `json:"recoveryOperatingSystem" in:"query" v:"max-length:128" dc:"Optional selected native image" eg:"97"`
	g.Meta                       `path:"/admin/product-fulfillment/tasks/{intentNo}" method:"get" tags:"IDC Product Fulfillment" summary:"Get product fulfillment task" dc:"Return one bounded safe task aggregate after exact permission and data-scope enforcement." permission:"idc:admin:product-fulfillment:task:query"`
	IntentNo                     string `json:"intentNo" in:"path" v:"required|max-length:40" dc:"Stable fulfillment intent number"`
}

type ProductFulfillmentTaskDetailRes struct {
	RecoveryConfiguration *provisionresourcecap.RecoveryEditor `json:"recoveryConfiguration,omitempty" dc:"Explicitly requested correction editor with immutable paid specifications and bounded compatible upstream choices" eg:"{}"`
	Task                  *ProductFulfillmentTaskDetail        `json:"task" dc:"Current safe product fulfillment task aggregate"`
}
