// This file exposes real, permission-controlled agents management.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementAgentListReq struct {
	g.Meta `path:"/admin/sales-management/agents" method:"get" tags:"IDC Sales Management" summary:"List agents" dc:"Return a bounded page of current agents with display names." permission:"idc:admin:sales-channel:overview:view"`
	SalesManagementPageQuery
}
type SalesManagementAgentListRes struct {
	Items []SalesManagementAgent `json:"items" dc:"Current page records" eg:"[]"`
	Total int                    `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementAgentCreateReq struct {
	g.Meta          `path:"/admin/sales-management/agents" method:"post" tags:"IDC Sales Management" summary:"Create agents" dc:"Persist one versioned agents change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:qualification:manage"`
	CustomerNo      string `json:"customerNo" v:"required|max-length:128" dc:"Sales management customerNo" eg:"example"`
	LevelNo         string `json:"levelNo" v:"max-length:128" dc:"Sales management levelNo" eg:"example"`
	Status          string `json:"status" v:"required|in:active,suspended" dc:"Sales management status" eg:"active"`
	ExpiresAt       *int64 `json:"expiresAt" dc:"Optional Unix timestamp in milliseconds; omitted means no expiry" eg:"1"`
	ExpectedVersion int64  `json:"expectedVersion" v:"in:0" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementAgentCreateRes struct {
	Item SalesManagementAgent `json:"item" dc:"Saved record" eg:"{}"`
}
type SalesManagementAgentUpdateReq struct {
	g.Meta          `path:"/admin/sales-management/agents" method:"put" tags:"IDC Sales Management" summary:"Update agents" dc:"Persist one versioned agents change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:qualification:manage"`
	CustomerNo      string `json:"customerNo" v:"required|max-length:128" dc:"Sales management customerNo" eg:"example"`
	LevelNo         string `json:"levelNo" v:"max-length:128" dc:"Sales management levelNo" eg:"example"`
	Status          string `json:"status" v:"required|in:active,suspended" dc:"Sales management status" eg:"active"`
	ExpiresAt       *int64 `json:"expiresAt" dc:"Optional Unix timestamp in milliseconds; omitted means no expiry" eg:"1"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementAgentUpdateRes struct {
	Item SalesManagementAgent `json:"item" dc:"Saved record" eg:"{}"`
}
