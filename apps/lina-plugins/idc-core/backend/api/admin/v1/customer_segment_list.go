package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerSegmentListReq struct {
	g.Meta     `path:"/admin/customer-segments" method:"get" tags:"IDC Customer Management" summary:"List customer segments" dc:"Return one bounded keyset page of active customer groups or tags for management filters." permission:"idc:admin:customer:list"`
	Kind       string `json:"kind" v:"required|in:group,tag" dc:"Segment catalog kind: group or tag" eg:"tag"`
	Locale     string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Localized indexed name used for filtering and ordering" eg:"zh-CN"`
	NamePrefix string `json:"namePrefix" dc:"Optional literal localized name prefix" eg:"Priority"`
	Cursor     string `json:"cursor" v:"max-length:1024" dc:"Opaque cursor from the previous response; empty for the first page" eg:""`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum active segment definitions returned in this page" eg:"25"`
}

type CustomerSegmentListRes struct {
	Items      []*CustomerSegmentItem `json:"items" dc:"Active customer group or tag definitions" eg:"[]"`
	NextCursor string                 `json:"nextCursor" dc:"Opaque cursor for the next page, or empty when complete" eg:""`
}
