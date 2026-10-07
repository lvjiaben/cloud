// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TaskEvidence is the golang structure for table task_evidence.
type TaskEvidence struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	EvidenceNo   string     `json:"evidenceNo"   orm:"evidence_no"   description:""`
	TaskNo       string     `json:"taskNo"       orm:"task_no"       description:""`
	SourceKind   string     `json:"sourceKind"   orm:"source_kind"   description:""`
	EvidenceKind string     `json:"evidenceKind" orm:"evidence_kind" description:""`
	Reference    string     `json:"reference"    orm:"reference"     description:"Bounded upstream task or resource reference; never a secret handle or raw payload"`
	Summary      string     `json:"summary"      orm:"summary"       description:"Bounded credential-scanned Provider or owner summary; never a raw response"`
	ObservedAt   *time.Time `json:"observedAt"   orm:"observed_at"   description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
