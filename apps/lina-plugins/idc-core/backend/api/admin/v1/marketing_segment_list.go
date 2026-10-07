// This file defines the bounded marketing segment management queue.

package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingSegmentListReq struct {
	Page       int `json:"page" v:"min:0" dc:"Optional one-based page number; mutually exclusive with cursor" eg:"1"`
	g.Meta     `path:"/admin/marketing/segments" method:"get" tags:"IDC Marketing Management" summary:"List marketing segments" dc:"Return one bounded keyset page of segment summaries and operational counts." permission:"idc:admin:marketing:segment:list"`
	NamePrefix string `json:"namePrefix" dc:"Optional literal segment code, Simplified Chinese name, or English name prefix; percent and underscore are not wildcards" eg:"loyal"`
	Mode       string `json:"mode" dc:"Optional segment mode: static or dynamic; empty includes both" eg:"dynamic"`
	Status     string `json:"status" dc:"Optional segment status: draft, active, paused, or archived; empty includes all" eg:"active"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Opaque cursor from the previous response; empty requests the first page" eg:""`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum segments returned in this page" eg:"25"`
}

type MarketingSegmentListRes struct {
	Total      int                            `json:"total" dc:"Exact total under the same authorization and filters" eg:"25"`
	Segments   []*MarketingSegmentSummaryItem `json:"segments" dc:"Bounded segment workbench summaries" eg:"[]"`
	NextCursor string                         `json:"nextCursor" dc:"Opaque cursor for the next page, or empty when complete" eg:""`
}
