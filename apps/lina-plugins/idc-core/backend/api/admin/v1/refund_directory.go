// This file defines bounded staff refund directory and immutable detail endpoints.

package v1

import "github.com/gogf/gf/v2/frame/g"

type RefundManagementListReq struct {
	Page       int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta     `path:"/admin/billing/refunds" method:"get" tags:"IDC Refund Management" summary:"List refund requests" dc:"Return one bounded newest-first staff refund directory after exact permission and data-scope checks." permission:"idc:admin:billing:refund:list"`
	CustomerNo string `json:"customerNo" in:"query" v:"max-length:40" dc:"Optional exact customer number filter; empty returns all visible customers" eg:"CUS_01J2ABCDEF"`
	StatusCode string `json:"statusCode" in:"query" v:"max-length:32" dc:"Optional exact refund status filter; empty returns all statuses" eg:"pending_review"`
	Cursor     string `json:"cursor" in:"query" v:"max-length:40" dc:"Optional next-page cursor returned by the previous response" eg:"rfd_01J2ABCDEF"`
	Limit      int    `json:"limit" in:"query" v:"min:0|max:100" dc:"Page size from 1 to 100; zero uses 20" eg:"20"`
}

type RefundManagementListRes struct {
	Total      int                         `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Refunds    []*RefundManagementMutation `json:"refunds" dc:"Newest-first visible refund summaries" eg:"[]"`
	NextCursor string                      `json:"nextCursor" dc:"Cursor for the next page, or empty when complete" eg:"rfd_01J2ABCDEF"`
}

type RefundManagementDetailReq struct {
	g.Meta   `path:"/admin/billing/refunds/{refundNo}" method:"get" tags:"IDC Refund Management" summary:"Get refund detail" dc:"Return one exact refund plus immutable selected-item and entitlement-impact snapshots after exact permission and data-scope checks." permission:"idc:admin:billing:refund:query"`
	RefundNo string `json:"refundNo" in:"path" v:"required|max-length:40" dc:"Stable refund number" eg:"rfd_01J2ABCDEF"`
}

type RefundManagementDetailRes struct {
	Detail *RefundManagementDetail `json:"detail" dc:"Exact refund and immutable review evidence" eg:"{}"`
}

type RefundManagementDetail struct {
	Refund *RefundManagementMutation `json:"refund" dc:"Authoritative refund summary" eg:"{}"`
	Items  []*RefundManagementItem   `json:"items" dc:"One to one hundred immutable selected-item snapshots" eg:"[]"`
}

type RefundManagementItem struct {
	RefundItemNo          string                    `json:"refundItemNo" dc:"Stable refund item number" eg:"rfi_01J2ABCDEF"`
	InvoiceItemNo         string                    `json:"invoiceItemNo" dc:"Original invoice item number" eg:"BII_01J2ABCDEF"`
	OrderItemNo           string                    `json:"orderItemNo" dc:"Original order item number" eg:"ORI_01J2ABCDEF"`
	Quantity              int                       `json:"quantity" dc:"Frozen refund quantity" eg:"1"`
	AmountMinor           string                    `json:"amountMinor" dc:"Requested refund amount in currency smallest units" eg:"1299"`
	RefundableAmountMinor string                    `json:"refundableAmountMinor" dc:"Refundable upper bound frozen at submission in currency smallest units" eg:"2599"`
	DescriptionZhCN       string                    `json:"descriptionZhCN" dc:"Frozen Chinese invoice item description" eg:"Cloud server"`
	DescriptionEnUS       string                    `json:"descriptionEnUS" dc:"Frozen English invoice item description" eg:"Cloud server"`
	Impacts               []*RefundManagementImpact `json:"impacts" dc:"Bounded immutable entitlement-impact snapshots for this selection" eg:"[]"`
}

type RefundManagementImpact struct {
	EntitlementNo       string `json:"entitlementNo" dc:"Affected long-lived entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
	EntitlementVersion  int64  `json:"entitlementVersion" dc:"Entitlement version frozen at customer confirmation" eg:"3"`
	StatusCode          string `json:"statusCode" dc:"Entitlement status frozen at customer confirmation" eg:"active"`
	RequiresTermination bool   `json:"requiresTermination" dc:"Whether refund execution must terminate this entitlement" eg:"true"`
}
