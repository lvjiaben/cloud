package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingValueGrantDetailReq struct {
	g.Meta  `path:"/admin/marketing/grants/{grantNo}" method:"get" tags:"IDC Marketing Management" summary:"Get granted value detail" dc:"Return one recharge bonus or cashback grant with bounded append-only entries." permission:"idc:admin:marketing:grant:query"`
	GrantNo string `json:"grantNo" v:"required|max-length:128" dc:"Stable granted-value business number" eg:"grant-001"`
}

type MarketingValueGrantDetailRes struct {
	Grant   *MarketingValueGrantItem   `json:"grant" dc:"Granted-value identity, explicit currency, conserved buckets, and lifecycle" eg:"{}"`
	Entries []*MarketingValueEntryItem `json:"entries" dc:"At most two hundred recent append-only value entries" eg:"[]"`
}
