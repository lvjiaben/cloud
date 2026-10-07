// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ChannelIntent is the golang structure for table channel_intent.
type ChannelIntent struct {
	TenantId                      int        `json:"tenantId"                      orm:"tenant_id"                        description:""`
	OperationNo                   string     `json:"operationNo"                   orm:"operation_no"                     description:""`
	ChannelCode                   string     `json:"channelCode"                   orm:"channel_code"                     description:""`
	OperationCode                 string     `json:"operationCode"                 orm:"operation_code"                   description:""`
	StatusCode                    string     `json:"statusCode"                    orm:"status_code"                      description:""`
	ActorUserId                   int64      `json:"actorUserId"                   orm:"actor_user_id"                    description:""`
	BaseStatusCode                string     `json:"baseStatusCode"                orm:"base_status_code"                 description:""`
	BaseVersion                   int64      `json:"baseVersion"                   orm:"base_version"                     description:""`
	RequestShapeDigest            string     `json:"requestShapeDigest"            orm:"request_shape_digest"             description:""`
	DesiredProviderCode           any        `json:"desiredProviderCode"           orm:"desired_provider_code"            description:""`
	DesiredNonSecretConfiguration string     `json:"desiredNonSecretConfiguration" orm:"desired_non_secret_configuration" description:""`
	SecretOperationKey            string     `json:"secretOperationKey"            orm:"secret_operation_key"             description:""`
	CredentialSecretRef           any        `json:"credentialSecretRef"           orm:"credential_secret_ref"            description:""`
	SecretExpectedUpdatedAt       any        `json:"secretExpectedUpdatedAt"       orm:"secret_expected_updated_at"       description:""`
	SecretResultUpdatedAt         any        `json:"secretResultUpdatedAt"         orm:"secret_result_updated_at"         description:""`
	SecretResultMaskedTail        string     `json:"secretResultMaskedTail"        orm:"secret_result_masked_tail"        description:""`
	AvailableAt                   any        `json:"availableAt"                   orm:"available_at"                     description:""`
	LeaseToken                    any        `json:"leaseToken"                    orm:"lease_token"                      description:""`
	LeaseExpiresAt                any        `json:"leaseExpiresAt"                orm:"lease_expires_at"                 description:""`
	AttemptCount                  int        `json:"attemptCount"                  orm:"attempt_count"                    description:""`
	LastErrorCode                 string     `json:"lastErrorCode"                 orm:"last_error_code"                  description:""`
	CompletedAt                   any        `json:"completedAt"                   orm:"completed_at"                     description:""`
	CreatedAt                     *time.Time `json:"createdAt"                     orm:"created_at"                       description:""`
	UpdatedAt                     *time.Time `json:"updatedAt"                     orm:"updated_at"                       description:""`
}
