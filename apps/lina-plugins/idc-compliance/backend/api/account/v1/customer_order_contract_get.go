// This file defines the customer-owned order contract detail API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerOrderContractGetReq returns one customer-owned order contract.
type CustomerOrderContractGetReq struct {
	g.Meta  `path:"/account/compliance/orders/{orderNo}/contract" method:"get" tags:"IDC Compliance Account" summary:"Get my order contract" dc:"Return the one contract for an order only when both belong to the authenticated customer. Non-owned and missing records have the same response."`
	OrderNo string `json:"orderNo" v:"required|max-length:64" dc:"Customer-owned order number" eg:"ord_0123456789abcdef0123456789abcdef"`
}

// CustomerOrderContractGetRes returns the order's customer-safe contract.
type CustomerOrderContractGetRes struct {
	Item CustomerContractItem `json:"item" dc:"Customer-safe contract associated with the requested order" eg:"{}"`
}
