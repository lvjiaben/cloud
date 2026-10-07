// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// UpstreamTaskBinding is the golang structure for table upstream_task_binding.
type UpstreamTaskBinding struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	BindingNo           string     `json:"bindingNo"           orm:"binding_no"            description:""`
	TaskNo              string     `json:"taskNo"              orm:"task_no"               description:""`
	StepNo              int        `json:"stepNo"              orm:"step_no"               description:""`
	ChannelNo           string     `json:"channelNo"           orm:"channel_no"            description:""`
	ExternalTaskRef     string     `json:"externalTaskRef"     orm:"external_task_ref"     description:""`
	ExternalResourceRef string     `json:"externalResourceRef" orm:"external_resource_ref" description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	LastProgress        int        `json:"lastProgress"        orm:"last_progress"         description:""`
	LastObservedAt      *time.Time `json:"lastObservedAt"      orm:"last_observed_at"      description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
}
