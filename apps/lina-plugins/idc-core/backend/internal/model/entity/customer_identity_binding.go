// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerIdentityBinding is the golang structure for table customer_identity_binding.
type CustomerIdentityBinding struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	BindingNo            string     `json:"bindingNo"            orm:"binding_no"             description:""`
	SysUserId            int        `json:"sysUserId"            orm:"sys_user_id"            description:"Host sys_user identity ID validated through host capabilities"`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"            description:"Bound customer business number"`
	SourceCode           string     `json:"sourceCode"           orm:"source_code"            description:"Binding source: onboarding, admin invitation, or migration"`
	SourceReferenceNo    string     `json:"sourceReferenceNo"    orm:"source_reference_no"    description:"Opaque onboarding, invitation, or migration reference"`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	ActivatedAt          *time.Time `json:"activatedAt"          orm:"activated_at"           description:""`
	RevokedAt            *time.Time `json:"revokedAt"            orm:"revoked_at"             description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedBy            int        `json:"createdBy"            orm:"created_by"             description:""`
	UpdatedBy            int        `json:"updatedBy"            orm:"updated_by"             description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
	MustChangePassword   bool       `json:"mustChangePassword"   orm:"must_change_password"   description:"Direct-created customer must replace the administrator-provided initial password before using account APIs"`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest" description:"Irreversible digest of the administrator direct-create retry key"`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:"Normalized direct-create request fingerprint excluding plaintext password"`
}
