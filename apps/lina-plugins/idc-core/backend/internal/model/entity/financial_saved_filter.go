// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialSavedFilter is the golang structure for table financial_saved_filter.
type FinancialSavedFilter struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	FilterNo     string     `json:"filterNo"     orm:"filter_no"     description:""`
	OwnerUserId  int        `json:"ownerUserId"  orm:"owner_user_id" description:""`
	ViewCode     string     `json:"viewCode"     orm:"view_code"     description:""`
	DisplayName  string     `json:"displayName"  orm:"display_name"  description:""`
	FilterJson   string     `json:"filterJson"   orm:"filter_json"   description:""`
	FilterDigest string     `json:"filterDigest" orm:"filter_digest" description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
