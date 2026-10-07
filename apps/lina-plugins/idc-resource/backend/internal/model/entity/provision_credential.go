// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProvisionCredential is the golang structure for table provision_credential.
type ProvisionCredential struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	CredentialNo string     `json:"credentialNo" orm:"credential_no" description:""`
	ServiceNo    string     `json:"serviceNo"    orm:"service_no"    description:""`
	TaskNo       string     `json:"taskNo"       orm:"task_no"       description:""`
	ChannelNo    string     `json:"channelNo"    orm:"channel_no"    description:""`
	SecretRef    string     `json:"secretRef"    orm:"secret_ref"    description:"Sensitive secretcap handle reference; never returned, logged, audited, or sent as ordinary JSON"`
	StatusCode   string     `json:"statusCode"   orm:"status_code"   description:""`
	RevealedAt   *time.Time `json:"revealedAt"   orm:"revealed_at"   description:""`
	RevokedAt    *time.Time `json:"revokedAt"    orm:"revoked_at"    description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
