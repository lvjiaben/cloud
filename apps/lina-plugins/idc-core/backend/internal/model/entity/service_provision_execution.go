// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceProvisionExecution is the golang structure for table service_provision_execution.
type ServiceProvisionExecution struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	ExecutionNo              string     `json:"executionNo"              orm:"execution_no"               description:""`
	OperationNo              string     `json:"operationNo"              orm:"operation_no"               description:""`
	ServiceNo                string     `json:"serviceNo"                orm:"service_no"                 description:""`
	ResourceOwnerCode        string     `json:"resourceOwnerCode"        orm:"resource_owner_code"        description:""`
	ProviderTaskNo           string     `json:"providerTaskNo"           orm:"provider_task_no"           description:"Opaque resource-owner task reference; never a Provider task identifier"`
	ChannelNo                string     `json:"channelNo"                orm:"channel_no"                 description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                description:""`
	RequestDigest            string     `json:"requestDigest"            orm:"request_digest"             description:""`
	CapabilityEvidenceNo     string     `json:"capabilityEvidenceNo"     orm:"capability_evidence_no"     description:""`
	CapabilityEvidenceDigest string     `json:"capabilityEvidenceDigest" orm:"capability_evidence_digest" description:""`
	TaskEvidenceDigest       string     `json:"taskEvidenceDigest"       orm:"task_evidence_digest"       description:""`
	LastFailureCode          string     `json:"lastFailureCode"          orm:"last_failure_code"          description:""`
	OutcomeReportStatus      string     `json:"outcomeReportStatus"      orm:"outcome_report_status"      description:""`
	OutcomeReportedAt        *time.Time `json:"outcomeReportedAt"        orm:"outcome_reported_at"        description:""`
	ObservedAt               *time.Time `json:"observedAt"               orm:"observed_at"                description:""`
	CompletedAt              *time.Time `json:"completedAt"              orm:"completed_at"               description:""`
	Version                  int64      `json:"version"                  orm:"version"                    description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                 description:""`
}
