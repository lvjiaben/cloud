// This file exposes real, permission-controlled ownership management.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementOwnershipListReq struct {
	g.Meta `path:"/admin/sales-management/ownership" method:"get" tags:"IDC Sales Management" summary:"List ownership" dc:"Return a bounded page of current ownership with display names." permission:"idc:admin:sales-channel:overview:view"`
	SalesManagementPageQuery
}
type SalesManagementOwnershipListRes struct {
	Items []SalesManagementOwnership `json:"items" dc:"Current page records" eg:"[]"`
	Total int                        `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementOwnershipCreateReq struct {
	g.Meta             `path:"/admin/sales-management/ownership" method:"post" tags:"IDC Sales Management" summary:"Create ownership" dc:"Persist one versioned ownership change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:assignment:manage"`
	CustomerNo         string `json:"customerNo" v:"required|max-length:128" dc:"Sales management customerNo" eg:"example"`
	ReferrerCustomerNo string `json:"referrerCustomerNo" v:"max-length:128" dc:"Sales management referrerCustomerNo" eg:"example"`
	StaffUserID        int    `json:"staffUserId" dc:"Sales management staffUserId" eg:"1"`
	ExpectedVersion    int64  `json:"expectedVersion" v:"in:0" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementOwnershipCreateRes struct {
	Item SalesManagementOwnership `json:"item" dc:"Saved record" eg:"{}"`
}
type SalesManagementOwnershipUpdateReq struct {
	g.Meta             `path:"/admin/sales-management/ownership" method:"put" tags:"IDC Sales Management" summary:"Update ownership" dc:"Persist one versioned ownership change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:assignment:manage"`
	CustomerNo         string `json:"customerNo" v:"required|max-length:128" dc:"Sales management customerNo" eg:"example"`
	ReferrerCustomerNo string `json:"referrerCustomerNo" v:"max-length:128" dc:"Sales management referrerCustomerNo" eg:"example"`
	StaffUserID        int    `json:"staffUserId" dc:"Sales management staffUserId" eg:"1"`
	ExpectedVersion    int64  `json:"expectedVersion" v:"required|min:1" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementOwnershipUpdateRes struct {
	Item SalesManagementOwnership `json:"item" dc:"Saved record" eg:"{}"`
}
