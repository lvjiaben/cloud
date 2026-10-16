// This file defines staff customer-bound authorization, call history and secret rotation.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	platformv1 "lina-plugin-idc-channel/backend/api/platform/v1"
)

// CustomerPlatformReq reads a selected customer's API metadata, never its stored secret.
type CustomerPlatformReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/open-platform" method:"get" tags:"Open Platform" summary:"Read customer API authorization" dc:"Read one customer-bound application's authorization and latest credential metadata under staff scope." permission:"idc:admin:channel:token"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Selected customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
}

// CustomerPlatformRes reuses the safe platform projection.
type CustomerPlatformRes struct{ platformv1.OverviewRes }

// CustomerAPICallsReq pages all retained successful and failed calls for one customer.
type CustomerAPICallsReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/open-platform/calls" method:"get" tags:"Open Platform" summary:"List customer API calls" dc:"Page sanitized call metadata with an exact customer-bound total and no request bodies or credentials." permission:"idc:admin:channel:token"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Selected customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	PageRequest
}

// CustomerAPICallsRes reuses the sanitized call page.
type CustomerAPICallsRes struct{ platformv1.CallsRes }

// CustomerSecretRotateReq rotates the displayed credential under its existing policy.
type CustomerSecretRotateReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/open-platform/secrets" method:"post" tags:"Open Platform" summary:"Rotate customer API secret" dc:"Replace the current secret atomically without changing scopes, sources, expiry or rate-window usage; stale versions are rejected." permission:"idc:admin:channel:token"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Selected customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	TokenID    int64  `json:"tokenId" v:"required|min:1" dc:"Displayed credential version; stale versions are rejected" eg:"42"`
}

// CustomerSecretRotateRes returns the new secret once.
type CustomerSecretRotateRes struct{ platformv1.RotateRes }
