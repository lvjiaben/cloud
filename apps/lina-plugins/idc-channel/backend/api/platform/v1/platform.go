// Package v1 defines current-customer platform metadata, rotation and call history.
package v1

import "github.com/gogf/gf/v2/frame/g"

// OverviewReq reads only the authenticated customer's authorization.
type OverviewReq struct {
	g.Meta `path:"/account/open-platform" method:"get" tags:"Open Platform" summary:"Read own API authorization" dc:"Read the current customer's API authorization without returning stored secrets."`
}

// OverviewRes excludes all stored secret material.
type OverviewRes struct {
	Authorized  bool     `json:"authorized" dc:"Whether an administrator has granted API access" eg:"true"`
	AppID       string   `json:"appId" dc:"Credential-bound application identifier" eg:"app_example"`
	DisplayName string   `json:"displayName" dc:"Application display name" eg:"My shop"`
	Status      string   `json:"status" dc:"Authorization state: approved, paused, pending_approval or closed" eg:"approved"`
	TokenID     int64    `json:"tokenId" dc:"Latest credential version for optimistic rotation" eg:"42"`
	CanReset    bool     `json:"canReset" dc:"Whether the current authorization can rotate its secret" eg:"true"`
	Scopes      []string `json:"scopes" dc:"Administrator-granted interface permissions" eg:"catalog.read"`
	SourceCIDRs []string `json:"sourceCidrs" dc:"Allowed source networks; empty means unrestricted" eg:"192.0.2.0/24"`
	ExpiresAt   *int64   `json:"expiresAt" dc:"Expiry as a Unix timestamp in milliseconds; null means no expiry" eg:"1798761600000"`
}

// RotateReq creates a replacement credential under the unchanged administrator policy.
type RotateReq struct {
	g.Meta  `path:"/account/open-platform/secrets" method:"post" tags:"Open Platform" summary:"Rotate own API secret" dc:"Atomically replace the current secret and revoke the old secret without changing permissions, sources or expiry."`
	TokenID int64 `json:"tokenId" v:"required|min:1" dc:"Displayed credential version; stale versions are rejected" eg:"42"`
}

// RotateRes returns the secret once and is never cacheable.
type RotateRes struct {
	AppID  string `json:"appId" dc:"Unchanged application identifier" eg:"app_example"`
	Secret string `json:"secret" dc:"One-time replacement secret; never stored in plaintext" eg:"example_secret"`
}

// CallsReq pages all retained calls for the current customer.
type CallsReq struct {
	g.Meta   `path:"/account/open-platform/calls" method:"get" tags:"Open Platform" summary:"List own API calls" dc:"Page sanitized success and failure metadata for the current customer only, without request bodies or credentials."`
	Page     int `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number, starting at one" eg:"1"`
	PageSize int `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Page size, maximum one hundred" eg:"10"`
}

// Call exposes no private request content.
type Call struct {
	ID             int64  `json:"id" dc:"Call record identifier" eg:"42"`
	Method         string `json:"method" dc:"HTTP request method" eg:"GET"`
	Operation      string `json:"operation" dc:"Sanitized interface permission identifier" eg:"catalog.read"`
	Succeeded      bool   `json:"succeeded" dc:"Whether the call succeeded" eg:"true"`
	DurationMillis int64  `json:"durationMillis" dc:"Execution duration in milliseconds" eg:"12"`
	CreatedAt      int64  `json:"createdAt" dc:"Call time as a Unix timestamp in milliseconds" eg:"1798761600000"`
}

// CallsRes contains one bounded page and the exact customer-scoped count.
type CallsRes struct {
	List  []Call `json:"list" dc:"Current page of call records" eg:"[]"`
	Total int    `json:"total" dc:"Exact total under the current customer scope" eg:"10"`
}
