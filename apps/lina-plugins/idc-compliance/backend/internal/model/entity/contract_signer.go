// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractSigner is the golang structure for table contract_signer.
type ContractSigner struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	SignerNo         string     `json:"signerNo"         orm:"signer_no"          description:""`
	FlowNo           string     `json:"flowNo"           orm:"flow_no"            description:""`
	SequenceNo       int        `json:"sequenceNo"       orm:"sequence_no"        description:""`
	SubjectTypeCode  string     `json:"subjectTypeCode"  orm:"subject_type_code"  description:""`
	SubjectNo        string     `json:"subjectNo"        orm:"subject_no"         description:""`
	RoleCode         string     `json:"roleCode"         orm:"role_code"          description:""`
	ProviderActorRef *string    `json:"providerActorRef" orm:"provider_actor_ref" description:""`
	SnapshotSha256   string     `json:"snapshotSha256"   orm:"snapshot_sha256"    description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"        description:""`
	SignedAt         *time.Time `json:"signedAt"         orm:"signed_at"          description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"         description:""`
}
