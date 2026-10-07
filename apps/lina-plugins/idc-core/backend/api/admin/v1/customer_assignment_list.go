package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerAssignmentListReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/segments" method:"get" tags:"IDC Customer Management" summary:"List customer group or tag assignments" dc:"Return one bounded keyset page of the selected customer's active group or tag assignments." permission:"idc:admin:customer:query"`
	CustomerNo string `json:"customerNo" v:"required" dc:"Stable external customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	Kind       string `json:"kind" v:"required|in:group,tag" dc:"Assignment kind: group or tag" eg:"tag"`
	Locale     string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Localized ordering: zh-CN or en-US" eg:"zh-CN"`
	Cursor     string `json:"cursor" dc:"Opaque next-page cursor"`
	Limit      int    `json:"limit" v:"min:0|max:100" dc:"Page size; zero selects the bounded default" eg:"25"`
}

type CustomerAssignmentListRes struct {
	Items      []*CustomerSegmentItem `json:"items" dc:"Active customer group or tag assignments" eg:"[]"`
	NextCursor string                 `json:"nextCursor" dc:"Opaque cursor for the next page, or empty"`
}
