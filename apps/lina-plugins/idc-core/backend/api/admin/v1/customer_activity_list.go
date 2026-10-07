package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerActivityListReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/activities" method:"get" tags:"IDC Customer Management" summary:"List customer activity history" dc:"Return one bounded keyset page of internal notes, follow-ups, or blacklist history." permission:"idc:admin:customer:query"`
	CustomerNo string `json:"customerNo" v:"required"`
	Kind       string `json:"kind" v:"required|in:note,follow_up,blacklist"`
	Cursor     string `json:"cursor"`
	Limit      int    `json:"limit" v:"min:0|max:100"`
}

type CustomerActivityItem struct {
	Number    string `json:"number"`
	Kind      string `json:"kind"`
	Category  string `json:"category"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	RiskLevel string `json:"riskLevel"`
	PlannedAt int64  `json:"plannedAt"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type CustomerActivityListRes struct {
	Items      []*CustomerActivityItem `json:"items"`
	NextCursor string                  `json:"nextCursor"`
}
