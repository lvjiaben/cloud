// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// DeliveryQuery is the golang structure for table delivery_query.
type DeliveryQuery struct {
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	QueryNo        string     `json:"queryNo"        orm:"query_no"        description:""`
	TaskNo         string     `json:"taskNo"         orm:"task_no"         description:""`
	SourceCode     string     `json:"sourceCode"     orm:"source_code"     description:""`
	ResultClass    string     `json:"resultClass"    orm:"result_class"    description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest" description:""`
	ObservedAt     *time.Time `json:"observedAt"     orm:"observed_at"     description:""`
}
