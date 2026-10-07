// This file defines the customer credit overview HTTP contract.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CreditOverviewGetReq requests the current customer's bounded credit overview.
type CreditOverviewGetReq struct {
	g.Meta `path:"/account/billing/credit/overview" method:"get" tags:"IDC Customer Credit" summary:"Get my credit overview" dc:"Return current-customer credit accounts and bounded application and statement counts."`
}

// CreditOverviewGetRes returns the current customer's bounded credit overview.
type CreditOverviewGetRes struct {
	Overview *CreditOverview `json:"overview" dc:"Current-customer credit overview" eg:"{}"`
}
