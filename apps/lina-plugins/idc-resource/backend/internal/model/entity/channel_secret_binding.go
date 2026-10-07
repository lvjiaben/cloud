// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ChannelSecretBinding is the golang structure for table channel_secret_binding.
type ChannelSecretBinding struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	BindingNo    string     `json:"bindingNo"    orm:"binding_no"    description:""`
	ChannelNo    string     `json:"channelNo"    orm:"channel_no"    description:""`
	BindingName  string     `json:"bindingName"  orm:"binding_name"  description:"Adapter-defined credential slot such as username or password"`
	SecretRef    string     `json:"secretRef"    orm:"secret_ref"    description:"Sensitive routing metadata from secretcap Handle.OpaqueRef; never returned, logged, or audited"`
	StatusCode   string     `json:"statusCode"   orm:"status_code"   description:"Recoverable owner lifecycle: preparing, retryable_failed, active, superseded, or revoked"`
	BoundAt      *time.Time `json:"boundAt"      orm:"bound_at"      description:""`
	SupersededAt *time.Time `json:"supersededAt" orm:"superseded_at" description:""`
	RevokedAt    *time.Time `json:"revokedAt"    orm:"revoked_at"    description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedBy    int        `json:"createdBy"    orm:"created_by"    description:""`
	UpdatedBy    int        `json:"updatedBy"    orm:"updated_by"    description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
