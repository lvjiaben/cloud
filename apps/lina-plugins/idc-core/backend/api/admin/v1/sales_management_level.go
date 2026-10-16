// This file exposes real, permission-controlled levels management.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementLevelListReq struct {
	g.Meta `path:"/admin/sales-management/levels" method:"get" tags:"IDC Sales Management" summary:"List levels" dc:"Return a bounded page of current levels with display names." permission:"idc:admin:sales-channel:overview:view"`
	SalesManagementPageQuery
}
type SalesManagementLevelListRes struct {
	Items []SalesManagementLevel `json:"items" dc:"Current page records" eg:"[]"`
	Total int                    `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementLevelCreateReq struct {
	g.Meta          `path:"/admin/sales-management/levels" method:"post" tags:"IDC Sales Management" summary:"Create levels" dc:"Persist one versioned levels change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:qualification:manage"`
	LevelNo         string `json:"levelNo" v:"max-length:128" dc:"Sales management levelNo" eg:"example"`
	Name            string `json:"name" v:"required|max-length:128" dc:"Sales management name" eg:"example"`
	EnglishName     string `json:"englishName" v:"max-length:128" dc:"Sales management englishName" eg:"example"`
	Status          string `json:"status" v:"required|in:active,inactive" dc:"Sales management status" eg:"active"`
	ExpectedVersion int64  `json:"expectedVersion" v:"in:0" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementLevelCreateRes struct {
	Item SalesManagementLevel `json:"item" dc:"Saved record" eg:"{}"`
}
type SalesManagementLevelUpdateReq struct {
	g.Meta          `path:"/admin/sales-management/levels" method:"put" tags:"IDC Sales Management" summary:"Update levels" dc:"Persist one versioned levels change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:qualification:manage"`
	LevelNo         string `json:"levelNo" v:"max-length:128" dc:"Sales management levelNo" eg:"example"`
	Name            string `json:"name" v:"required|max-length:128" dc:"Sales management name" eg:"example"`
	EnglishName     string `json:"englishName" v:"max-length:128" dc:"Sales management englishName" eg:"example"`
	Status          string `json:"status" v:"required|in:active,inactive" dc:"Sales management status" eg:"active"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementLevelUpdateRes struct {
	Item SalesManagementLevel `json:"item" dc:"Saved record" eg:"{}"`
}
