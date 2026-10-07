// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAffiliateLink is the golang structure for table sales_affiliate_link.
type SalesAffiliateLink struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	LinkNo                   string     `json:"linkNo"                   orm:"link_no"                    description:""`
	OwnerCustomerNo          string     `json:"ownerCustomerNo"          orm:"owner_customer_no"          description:""`
	CodeDigest               string     `json:"codeDigest"               orm:"code_digest"                description:""`
	AttributionPolicyCode    string     `json:"attributionPolicyCode"    orm:"attribution_policy_code"    description:""`
	AttributionWindowSeconds int        `json:"attributionWindowSeconds" orm:"attribution_window_seconds" description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                description:""`
	ExpiresAt                *time.Time `json:"expiresAt"                orm:"expires_at"                 description:""`
	Version                  int64      `json:"version"                  orm:"version"                    description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                 description:""`
}
