// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// DeliveryAttempt is the golang structure for table delivery_attempt.
type DeliveryAttempt struct {
	TenantId                int    `json:"tenantId"                orm:"tenant_id"                 description:""`
	AttemptNo               string `json:"attemptNo"               orm:"attempt_no"                description:""`
	TaskNo                  string `json:"taskNo"                  orm:"task_no"                   description:""`
	RetrySequence           int    `json:"retrySequence"           orm:"retry_sequence"            description:""`
	AttemptFence            string `json:"attemptFence"            orm:"attempt_fence"             description:""`
	CrossedExternalBoundary bool   `json:"crossedExternalBoundary" orm:"crossed_external_boundary" description:""`
	ResultClass             string `json:"resultClass"             orm:"result_class"              description:""`
	EvidenceDigest          string `json:"evidenceDigest"          orm:"evidence_digest"           description:""`
	ProviderReference       any    `json:"providerReference"       orm:"provider_reference"        description:""`
	StartedAt               any    `json:"startedAt"               orm:"started_at"                description:""`
	FinishedAt              any    `json:"finishedAt"              orm:"finished_at"               description:""`
}
