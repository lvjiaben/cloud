// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyRetentionPolicy is the golang structure for table privacy_retention_policy.
type PrivacyRetentionPolicy struct {
	Id              int64      `json:"id"              orm:"id"                 description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"          description:""`
	PolicyNo        string     `json:"policyNo"        orm:"policy_no"          description:""`
	DataTypeCode    string     `json:"dataTypeCode"    orm:"data_type_code"     description:""`
	Version         int64      `json:"version"         orm:"version"            description:""`
	RetentionDays   int        `json:"retentionDays"   orm:"retention_days"     description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"        description:""`
	CreatedByUserId int        `json:"createdByUserId" orm:"created_by_user_id" description:""`
	SupersededAt    *time.Time `json:"supersededAt"    orm:"superseded_at"      description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"         description:""`
}
