package v1

import "github.com/gogf/gf/v2/frame/g"

type GrantListReq struct {
	g.Meta         `path:"/admin/staff-access/grants" method:"get" tags:"Staff Permissions" summary:"List staff business qualifications" dc:"Returns a bounded, visible page of target-staff business qualifications." permission:"system:staff-access:list"`
	Keyword        string `json:"keyword" dc:"Grant, employee, capability, or scope keyword"`
	CapabilityCode string `json:"capabilityCode" dc:"Exact capability code"`
	Status         int    `json:"status" d:"-1" v:"in:-1,0,1" dc:"-1 all, 0 disabled, 1 enabled"`
	Page           int    `json:"page" d:"1" v:"min:1"`
	PageSize       int    `json:"pageSize" d:"20" v:"min:1|max:100"`
}

type GrantItem struct {
	GrantNo        string `json:"grantNo"`
	UserID         int64  `json:"userId"`
	Username       string `json:"username"`
	Nickname       string `json:"nickname"`
	CapabilityCode string `json:"capabilityCode"`
	ScopeType      string `json:"scopeType"`
	ScopeKey       string `json:"scopeKey"`
	Status         int    `json:"status"`
	Version        int64  `json:"version"`
	CreatedAt      int64  `json:"createdAt" dc:"Creation time in Unix milliseconds"`
	UpdatedAt      int64  `json:"updatedAt" dc:"Last update time in Unix milliseconds"`
	ValidFrom      *int64 `json:"validFrom" dc:"Optional activation time in Unix milliseconds"`
	ValidUntil     *int64 `json:"validUntil" dc:"Optional expiry time in Unix milliseconds"`
}

type GrantListRes struct {
	Items []GrantItem `json:"items"`
	Total int64       `json:"total"`
}
