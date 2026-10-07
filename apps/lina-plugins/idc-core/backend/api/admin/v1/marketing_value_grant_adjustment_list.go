// This file defines the bounded granted-value financial review queue.

package v1

import "github.com/gogf/gf/v2/frame/g"

// MarketingValueGrantAdjustmentListReq lists reviewed value actions by status.
type MarketingValueGrantAdjustmentListReq struct {
	Page   int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta `path:"/admin/marketing/grant-adjustments" method:"get" tags:"IDC Marketing" summary:"List granted-value financial actions" dc:"Return a bounded staff review queue. Listing requires the same exact review permission and never changes financial state." permission:"idc:admin:marketing:grant:review"`
	Cursor string `json:"cursor" v:"length:0,128" dc:"Opaque adjustment cursor from the previous page"`
	Limit  int    `json:"limit" v:"min:0|max:100" d:"25" dc:"Maximum rows, from 1 through 100"`
	Status string `json:"status" v:"in:pending_review,rejected,posted" d:"pending_review" dc:"Exact review status"`
}

// MarketingValueGrantAdjustmentListRes returns a stable keyset page.
type MarketingValueGrantAdjustmentListRes struct {
	Total       int                                       `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Adjustments []*MarketingValueGrantFinancialAdjustment `json:"adjustments" dc:"Bounded reviewed financial actions"`
	NextCursor  string                                    `json:"nextCursor" dc:"Opaque cursor for the next page, or empty"`
}
