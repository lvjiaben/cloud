// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerOperationExecution is the golang structure for table server_operation_execution.
type ServerOperationExecution struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	ExecutionNo               string     `json:"executionNo"               orm:"execution_no"                 description:""`
	OperationNo               string     `json:"operationNo"               orm:"operation_no"                 description:""`
	ServiceNo                 string     `json:"serviceNo"                 orm:"service_no"                   description:""`
	ActionCode                string     `json:"actionCode"                orm:"action_code"                  description:""`
	ResourceOwnerCode         string     `json:"resourceOwnerCode"         orm:"resource_owner_code"          description:""`
	ChannelNo                 string     `json:"channelNo"                 orm:"channel_no"                   description:""`
	PoolNo                    string     `json:"poolNo"                    orm:"pool_no"                      description:""`
	ExpectedServiceVersion    int64      `json:"expectedServiceVersion"    orm:"expected_service_version"     description:""`
	OriginalRuntimeStatusCode string     `json:"originalRuntimeStatusCode" orm:"original_runtime_status_code" description:""`
	Parameters                string     `json:"parameters"                orm:"parameters"                   description:""`
	SecretBindings            string     `json:"secretBindings"            orm:"secret_bindings"              description:"Opaque input secret references only; plaintext is forbidden"`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                  description:""`
	ProviderTaskNo            *string    `json:"providerTaskNo"            orm:"provider_task_no"             description:""`
	RequestDigest             string     `json:"requestDigest"             orm:"request_digest"               description:""`
	CapabilityEvidenceNo      *string    `json:"capabilityEvidenceNo"      orm:"capability_evidence_no"       description:""`
	CapabilityEvidenceDigest  *string    `json:"capabilityEvidenceDigest"  orm:"capability_evidence_digest"   description:""`
	TaskEvidenceDigest        *string    `json:"taskEvidenceDigest"        orm:"task_evidence_digest"         description:""`
	LastFailureCode           string     `json:"lastFailureCode"           orm:"last_failure_code"            description:""`
	OutputBindings            string     `json:"outputBindings"            orm:"output_bindings"              description:"Opaque confidential result references with one-time or short-lived delivery policy"`
	ObservedAt                *time.Time `json:"observedAt"                orm:"observed_at"                  description:""`
	CompletedAt               *time.Time `json:"completedAt"               orm:"completed_at"                 description:""`
	Version                   int64      `json:"version"                   orm:"version"                      description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
	UpdatedAt                 *time.Time `json:"updatedAt"                 orm:"updated_at"                   description:""`
}
