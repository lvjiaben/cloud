// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Secret is the golang structure for table secret.
type Secret struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	SecretNo       string     `json:"secretNo"       orm:"secret_no"       description:"Opaque secret business number reserved by the core business-number ledger"`
	OwnerPluginId  string     `json:"ownerPluginId"  orm:"owner_plugin_id" description:"Plugin that owns and authorizes the secret binding"`
	CallerScope    string     `json:"callerScope"    orm:"caller_scope"    description:"Exact least-privilege caller scope bound into encryption AAD"`
	StatusCode     string     `json:"statusCode"     orm:"status_code"     description:"Secret lifecycle state: configured or revoked"`
	CurrentVersion int        `json:"currentVersion" orm:"current_version" description:"Current immutable encrypted payload version"`
	MaskedTail     string     `json:"maskedTail"     orm:"masked_tail"     description:"Non-secret fixed-prefix display mask with at most four printable trailing bytes"`
	RevokedAt      *time.Time `json:"revokedAt"      orm:"revoked_at"      description:"Time after which callback access is denied"`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"      description:""`
}
