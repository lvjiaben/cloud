// This file defines bounded staff order queries and detailed order projections.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	accountv1 "lina-plugin-idc-core/backend/api/account/v1"
)

type CommerceOrderListReq struct {
	g.Meta           `path:"/admin/commerce/orders" method:"get" tags:"IDC Commerce Management" summary:"List commerce orders" dc:"Return one bounded numbered or encrypted-keyset page with an accurate filtered total for direct active staff with exact order-list permission." permission:"idc:admin:commerce:order:list"`
	CustomerNo       string `json:"customerNo" v:"max-length:40" dc:"Optional exact owning customer number"`
	StatusCode       string `json:"statusCode" dc:"Optional exact order status"`
	RiskDecisionCode string `json:"riskDecisionCode" dc:"Optional exact risk decision"`
	Cursor           string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page"`
	Page             int    `json:"page" v:"min:0" dc:"One-based page number; omit when using a cursor" eg:"1"`
	Limit            int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum orders returned in this page"`
}

type CommerceOrderListRes struct {
	Orders     []*accountv1.CommerceOrderSummary `json:"orders" dc:"Bounded order summaries"`
	NextCursor string                            `json:"nextCursor" dc:"Encrypted next-page cursor, or empty"`
	Total      int                               `json:"total" dc:"Exact total under the current filters and access scope" eg:"25"`
}

type CommerceOrderDetailReq struct {
	g.Meta  `path:"/admin/commerce/orders/{orderNo}" method:"get" tags:"IDC Commerce Management" summary:"Get commerce order" dc:"Return one bounded aggregate without frontend waterfall queries or dangerous lifecycle shortcuts." permission:"idc:admin:commerce:order:query"`
	OrderNo string `json:"orderNo" in:"path" v:"required|max-length:40" dc:"Stable order business number"`
}

type CommerceOrderDetailRes struct {
	Order        *accountv1.CommerceOrderDetail `json:"order" dc:"Bounded order, invoice, inventory, fulfillment, cancellation and review aggregate"`
	ContractRule *ContractPolicyOrderRule       `json:"contractRule" dc:"Current editable order override and frozen final contract decision"`
}
