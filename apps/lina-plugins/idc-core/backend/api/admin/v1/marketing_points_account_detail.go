package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingPointsAccountDetailReq struct {
	g.Meta     `path:"/admin/marketing/points/accounts/{customerNo}" method:"get" tags:"IDC Marketing Management" summary:"Get points account detail" dc:"Return one non-monetary points account with bounded lots and entries." permission:"idc:admin:marketing:points:query"`
	CustomerNo string `json:"customerNo" v:"required|max-length:128" dc:"Owning customer business number" eg:"customer-001"`
}

type MarketingPointsAccountDetailRes struct {
	Account *MarketingPointsAccountItem `json:"account" dc:"Non-monetary points account balances and counts" eg:"{}"`
	Lots    []*MarketingPointsLotItem   `json:"lots" dc:"At most one hundred recent conserved points lots" eg:"[]"`
	Entries []*MarketingPointsEntryItem `json:"entries" dc:"At most two hundred recent append-only points entries" eg:"[]"`
}
