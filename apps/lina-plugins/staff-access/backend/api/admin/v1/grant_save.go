package v1

import "github.com/gogf/gf/v2/frame/g"

type GrantSaveReq struct {
	g.Meta          `path:"/admin/staff-access/grants" method:"put" tags:"Staff Permissions" summary:"Create or change a staff business qualification" dc:"Requires the exact management permission, a direct session, a recent full login, a visible enabled target, a reason, and optimistic version evidence." permission:"system:staff-access:manage"`
	UserID          int64  `json:"userId" v:"required|min:1"`
	CapabilityCode  string `json:"capabilityCode" v:"required|length:3,128"`
	ScopeType       string `json:"scopeType" v:"required|in:global,department"`
	ScopeKey        string `json:"scopeKey" v:"length:0,128"`
	Enabled         bool   `json:"enabled"`
	Reason          string `json:"reason" v:"required|length:3,500"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:0"`
	ValidFrom       *int64 `json:"validFrom" dc:"Optional activation time in Unix milliseconds"`
	ValidUntil      *int64 `json:"validUntil" dc:"Optional expiry time in Unix milliseconds"`
}

type GrantSaveRes struct {
	Item GrantItem `json:"item"`
}
