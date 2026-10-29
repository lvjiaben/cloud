package v1

import "github.com/gogf/gf/v2/frame/g"

// ContactIndexRefreshReq refreshes one visible host-user page into the derived
// customer-only contact directory; it never stores administrator credentials.
type ContactIndexRefreshReq struct {
	g.Meta               `path:"/admin/account-login/contact-index-refreshes" method:"post" tags:"IDC Customer Login" summary:"Refresh one customer login contact page" dc:"Read one public, permission-scoped host user page using the current administrator and update encrypted customer contact indexes. Repeat bounded pages until the visible total is reached." permission:"idc:admin:customer:update"`
	RepairCustomerAccess bool `json:"repairCustomerAccess" dc:"Explicitly repair roleless active admin-created customers through the public role assignment API; staff, frozen accounts and existing role allocations are excluded" eg:"false"`
	PageNum              int  `json:"pageNum" d:"1" v:"min:1|max:100000" dc:"Page to refresh; each page contains at most 100 visible host users" eg:"1"`
}
type ContactIndexRefreshRes struct {
	RepairCandidateUserIDs []int `json:"repairCandidateUserIds" dc:"Visible roleless active admin-created customer IDs eligible for an explicit repair" eg:"[42]"`
	AccessRepaired         bool  `json:"accessRepaired" dc:"Whether the explicitly requested page repair was applied" eg:"false"`
	Refreshed              int   `json:"refreshed" dc:"Customer indexes refreshed on this page" eg:"25"`
	Total                  int   `json:"total" dc:"Total visible host users; no global account count is exposed" eg:"100"`
}
