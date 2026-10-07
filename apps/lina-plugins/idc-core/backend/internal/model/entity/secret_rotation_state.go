// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SecretRotationState is the golang structure for table secret_rotation_state.
type SecretRotationState struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	OwnerPluginId          string     `json:"ownerPluginId"          orm:"owner_plugin_id"          description:""`
	CallerScope            string     `json:"callerScope"            orm:"caller_scope"             description:""`
	Epoch                  int64      `json:"epoch"                  orm:"epoch"                    description:"Monotonic active-key verification generation"`
	ActiveKeyId            string     `json:"activeKeyId"            orm:"active_key_id"            description:"Active key identifier certified by the current generation"`
	StatusCode             string     `json:"statusCode"             orm:"status_code"              description:""`
	WriteFence             int64      `json:"writeFence"             orm:"write_fence"              description:"Monotonic payload mutation fence shared by all service instances"`
	VerificationCursorId   int64      `json:"verificationCursorId"   orm:"verification_cursor_id"   description:"Last immutable payload row authenticated in the current generation"`
	VerificationWriteFence int64      `json:"verificationWriteFence" orm:"verification_write_fence" description:"Write fence captured when the current full verification pass began"`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"               description:""`
}
