// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// WecomEmployeeBinding is the golang structure for table wecom_employee_binding.
type WecomEmployeeBinding struct {
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                  description:""`
	BindingNo               string     `json:"bindingNo"               orm:"binding_no"                 description:""`
	HostUserId              int64      `json:"hostUserId"              orm:"host_user_id"               description:""`
	StaffVersion            int64      `json:"staffVersion"            orm:"staff_version"              description:""`
	CorpId                  string     `json:"corpId"                  orm:"corp_id"                    description:""`
	ChannelGeneration       int64      `json:"channelGeneration"       orm:"channel_generation"         description:""`
	IdentityDigest          string     `json:"identityDigest"          orm:"identity_digest"            description:""`
	IdentitySecretRef       any        `json:"identitySecretRef"       orm:"identity_secret_ref"        description:""`
	IdentitySecretUpdatedAt any        `json:"identitySecretUpdatedAt" orm:"identity_secret_updated_at" description:""`
	SecretOperationKey      string     `json:"secretOperationKey"      orm:"secret_operation_key"       description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"                description:""`
	TerminalStatusCode      string     `json:"terminalStatusCode"      orm:"terminal_status_code"       description:""`
	RequestNo               string     `json:"requestNo"               orm:"request_no"                 description:""`
	AvailableAt             any        `json:"availableAt"             orm:"available_at"               description:""`
	LeaseToken              any        `json:"leaseToken"              orm:"lease_token"                description:""`
	LeaseExpiresAt          any        `json:"leaseExpiresAt"          orm:"lease_expires_at"           description:""`
	AttemptCount            int        `json:"attemptCount"            orm:"attempt_count"              description:""`
	LastErrorCode           string     `json:"lastErrorCode"           orm:"last_error_code"            description:""`
	Version                 int64      `json:"version"                 orm:"version"                    description:""`
	BoundAt                 any        `json:"boundAt"                 orm:"bound_at"                   description:""`
	TerminalAt              any        `json:"terminalAt"              orm:"terminal_at"                description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                 description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                 description:""`
}
