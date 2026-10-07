// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TaskSecretBinding is the golang structure for table task_secret_binding.
type TaskSecretBinding struct {
	Id               int64      `json:"id"               orm:"id"                description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"         description:""`
	BindingNo        string     `json:"bindingNo"        orm:"binding_no"        description:""`
	TaskNo           string     `json:"taskNo"           orm:"task_no"           description:""`
	BindingName      string     `json:"bindingName"      orm:"binding_name"      description:""`
	BindingDirection string     `json:"bindingDirection" orm:"binding_direction" description:""`
	SecretRef        string     `json:"secretRef"        orm:"secret_ref"        description:"Sensitive secretcap routing reference excluded from task JSON, logs, audit, and API responses"`
	OutputPolicy     string     `json:"outputPolicy"     orm:"output_policy"     description:"One-time credential or short-lived ticket delivery policy for output bindings"`
	ExpiresAt        *time.Time `json:"expiresAt"        orm:"expires_at"        description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"       description:""`
	RevealedAt       *time.Time `json:"revealedAt"       orm:"revealed_at"       description:""`
	RevokedAt        *time.Time `json:"revokedAt"        orm:"revoked_at"        description:""`
	Version          int64      `json:"version"          orm:"version"           description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"        description:""`
}
