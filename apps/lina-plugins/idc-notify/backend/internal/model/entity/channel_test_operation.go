// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ChannelTestOperation is the golang structure for table channel_test_operation.
type ChannelTestOperation struct {
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	OperationNo             string     `json:"operationNo"             orm:"operation_no"              description:""`
	ChannelCode             string     `json:"channelCode"             orm:"channel_code"              description:""`
	ChannelVersion          int64      `json:"channelVersion"          orm:"channel_version"           description:""`
	TemplateNo              string     `json:"templateNo"              orm:"template_no"               description:""`
	TemplateVersion         int64      `json:"templateVersion"         orm:"template_version"          description:""`
	TargetMask              string     `json:"targetMask"              orm:"target_mask"               description:""`
	RequestDigest           string     `json:"requestDigest"           orm:"request_digest"            description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	ResultClass             string     `json:"resultClass"             orm:"result_class"              description:""`
	CrossedExternalBoundary bool       `json:"crossedExternalBoundary" orm:"crossed_external_boundary" description:""`
	ProviderReference       any        `json:"providerReference"       orm:"provider_reference"        description:""`
	ActorUserId             int64      `json:"actorUserId"             orm:"actor_user_id"             description:""`
	CompletedAt             any        `json:"completedAt"             orm:"completed_at"              description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
}
