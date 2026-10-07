// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrincipalTask is the golang structure for table principal_task.
type PrincipalTask struct {
	Id                           int64      `json:"id"                           orm:"id"                              description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                       description:""`
	TaskNo                       string     `json:"taskNo"                       orm:"task_no"                         description:""`
	CustomerNo                   string     `json:"customerNo"                   orm:"customer_no"                     description:""`
	ChannelNo                    string     `json:"channelNo"                    orm:"channel_no"                      description:""`
	UsernameSecretRef            string     `json:"usernameSecretRef"            orm:"username_secret_ref"             description:"Channel-scoped secretcap handle; never a username or customer profile field"`
	PasswordSecretRef            string     `json:"passwordSecretRef"            orm:"password_secret_ref"             description:"Channel-scoped secretcap handle; plaintext is never persisted in the resource schema"`
	StatusCode                   string     `json:"statusCode"                   orm:"status_code"                     description:""`
	ExpectedExternalPrincipalRef string     `json:"expectedExternalPrincipalRef" orm:"expected_external_principal_ref" description:""`
	ResultUnknown                bool       `json:"resultUnknown"                orm:"result_unknown"                  description:"Hard replay fence: when true, only exact lookup or manual review is allowed"`
	AttemptCount                 int        `json:"attemptCount"                 orm:"attempt_count"                   description:"Count of write-eligible claims; unknown writes are never made claimable again"`
	ObservationAttemptCount      int        `json:"observationAttemptCount"      orm:"observation_attempt_count"       description:"Read-only exact lookup attempts; never authorizes another create"`
	AvailableAt                  *time.Time `json:"availableAt"                  orm:"available_at"                    description:""`
	LeaseTokenDigest             string     `json:"leaseTokenDigest"             orm:"lease_token_digest"              description:""`
	LeaseExpiresAt               *time.Time `json:"leaseExpiresAt"               orm:"lease_expires_at"                description:""`
	FailureCode                  string     `json:"failureCode"                  orm:"failure_code"                    description:""`
	EvidenceDigest               string     `json:"evidenceDigest"               orm:"evidence_digest"                 description:""`
	FinishedAt                   *time.Time `json:"finishedAt"                   orm:"finished_at"                     description:"Confirmed mappings revoke both handles; manual review revokes the password and retains only its encrypted username handle for operator reconciliation"`
	Version                      int64      `json:"version"                      orm:"version"                         description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                      description:""`
	UpdatedAt                    *time.Time `json:"updatedAt"                    orm:"updated_at"                      description:""`
}
