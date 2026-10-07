// This file defines shared platform response DTOs for the tenant-core API.
package v1

// TenantItem is the platform tenant API projection.
type TenantItem struct {
	Id        int64  `json:"id" dc:"Tenant ID" eg:"1"`
	Code      string `json:"code" dc:"Stable tenant code" eg:"acme"`
	Name      string `json:"name" dc:"Tenant display name" eg:"Acme BU"`
	Status    string `json:"status" dc:"Tenant lifecycle status" eg:"active"`
	Remark    string `json:"remark" dc:"Tenant remark" eg:"Internal business unit"`
	CreatedAt *int64 `json:"createdAt" dc:"Tenant creation time as Unix timestamp in milliseconds" eg:"1778374800000"`
}
