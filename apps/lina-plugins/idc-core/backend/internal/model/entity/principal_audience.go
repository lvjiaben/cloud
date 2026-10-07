// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrincipalAudience is the golang structure for table principal_audience.
type PrincipalAudience struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	SysUserId    int        `json:"sysUserId"    orm:"sys_user_id"   description:"Host sys_user identity ID validated through host capabilities"`
	AudienceCode string     `json:"audienceCode" orm:"audience_code" description:"Audience code: customer or staff"`
	StatusCode   string     `json:"statusCode"   orm:"status_code"   description:"Binding status code: active or revoked"`
	BoundAt      *time.Time `json:"boundAt"      orm:"bound_at"      description:"Time when the audience binding became active"`
	RevokedAt    *time.Time `json:"revokedAt"    orm:"revoked_at"    description:"Time when the audience binding was revoked"`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedBy    int        `json:"createdBy"    orm:"created_by"    description:""`
	UpdatedBy    int        `json:"updatedBy"    orm:"updated_by"    description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
