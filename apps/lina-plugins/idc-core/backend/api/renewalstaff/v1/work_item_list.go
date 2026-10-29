// This file exposes a bounded queue of authorized accessory-renewal work items.

package v1

import "github.com/gogf/gf/v2/frame/g"

// WorkItemListReq applies the same precise permission as notification sync.
type WorkItemListReq struct {
	g.Meta   `path:"/admin/server-accessory-renewal-work-items" method:"get" tags:"IDC Accessory Renewals" summary:"List accessory renewal work items" dc:"Read a bounded current-staff queue after merchant data-scope and renewal permission checks." permission:"idc:admin:resource:accessory-renewal:handle"`
	Page     int `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number starting at one" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"min:1|max:50" dc:"Page size between one and fifty" eg:"20"`
}

// WorkItem carries only safe local references needed for staff attention.
type WorkItem struct {
	WorkItemNo string `json:"workItemNo" dc:"Stable renewal work item number" eg:"arw_001"`
	InvoiceNo  string `json:"invoiceNo" dc:"Related unpaid invoice number" eg:"INV_001"`
	CustomerNo string `json:"customerNo" dc:"Customer owning the accessory" eg:"CUS_001"`
	SubjectNo  string `json:"subjectNo" dc:"Original service or native entitlement number" eg:"ENT_001"`
	ReasonCode string `json:"reasonCode" dc:"Reason: balance_payment_failed or deletion_invoice_pending" eg:"balance_payment_failed"`
	CreatedAt  int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1792742400000"`
}

// WorkItemListRes returns the exact merchant-filtered total and current page.
type WorkItemListRes struct {
	Items []WorkItem `json:"items" dc:"Current page of open renewal work items" eg:"[]"`
	Total int        `json:"total" dc:"Exact count under the same current staff data scope" eg:"1"`
}
