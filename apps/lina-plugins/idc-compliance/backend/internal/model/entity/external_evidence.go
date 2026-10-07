// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ExternalEvidence is the golang structure for table external_evidence.
type ExternalEvidence struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	EvidenceNo        string     `json:"evidenceNo"        orm:"evidence_no"         description:""`
	AggregateTypeCode string     `json:"aggregateTypeCode" orm:"aggregate_type_code" description:""`
	AggregateNo       string     `json:"aggregateNo"       orm:"aggregate_no"        description:""`
	SourceSystemCode  string     `json:"sourceSystemCode"  orm:"source_system_code"  description:""`
	SourceEventRef    string     `json:"sourceEventRef"    orm:"source_event_ref"    description:""`
	EvidenceTypeCode  string     `json:"evidenceTypeCode"  orm:"evidence_type_code"  description:""`
	PayloadSha256     string     `json:"payloadSha256"     orm:"payload_sha256"      description:""`
	PrivateObjectRef  *string    `json:"privateObjectRef"  orm:"private_object_ref"  description:""`
	MediaType         *string    `json:"mediaType"         orm:"media_type"          description:""`
	ByteSize          *int64     `json:"byteSize"          orm:"byte_size"           description:""`
	DispositionCode   string     `json:"dispositionCode"   orm:"disposition_code"    description:""`
	OccurredAt        *time.Time `json:"occurredAt"        orm:"occurred_at"         description:""`
	ReceivedAt        *time.Time `json:"receivedAt"        orm:"received_at"         description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
