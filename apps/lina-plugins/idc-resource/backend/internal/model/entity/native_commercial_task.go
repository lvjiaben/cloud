// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// NativeCommercialTask is the golang structure for table native_commercial_task.
type NativeCommercialTask struct {
	OperationNo         string     `json:"operationNo"         orm:"operation_no"          description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	EntitlementNo       string     `json:"entitlementNo"       orm:"entitlement_no"        description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	TaskNo              string     `json:"taskNo"              orm:"task_no"               description:""`
	LocalResourceNo     string     `json:"localResourceNo"     orm:"local_resource_no"     description:""`
	ChannelNo           string     `json:"channelNo"           orm:"channel_no"            description:""`
	PoolNo              string     `json:"poolNo"              orm:"pool_no"               description:""`
	ExternalResourceRef string     `json:"externalResourceRef" orm:"external_resource_ref" description:""`
	ActionCode          string     `json:"actionCode"          orm:"action_code"           description:""`
	Parameters          string     `json:"parameters"          orm:"parameters"            description:""`
	RequestDigest       string     `json:"requestDigest"       orm:"request_digest"        description:""`
	IntentDigest        string     `json:"intentDigest"        orm:"intent_digest"         description:""`
	EntitlementVersion  int64      `json:"entitlementVersion"  orm:"entitlement_version"   description:""`
	AdmittedAt          *time.Time `json:"admittedAt"          orm:"admitted_at"           description:""`
	AdmittedAttempt     int        `json:"admittedAttempt"     orm:"admitted_attempt"      description:""`
	ResultStatus        string     `json:"resultStatus"        orm:"result_status"         description:""`
	NextReportAt        *time.Time `json:"nextReportAt"        orm:"next_report_at"        description:""`
	CompletedAt         *time.Time `json:"completedAt"         orm:"completed_at"          description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
}
