// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerConsent is the golang structure for table customer_consent.
type CustomerConsent struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	ConsentNo       string     `json:"consentNo"       orm:"consent_no"        description:""`
	CustomerNo      string     `json:"customerNo"      orm:"customer_no"       description:""`
	ConsentTypeCode string     `json:"consentTypeCode" orm:"consent_type_code" description:""`
	PolicyReference string     `json:"policyReference" orm:"policy_reference"  description:""`
	PolicyVersion   string     `json:"policyVersion"   orm:"policy_version"    description:""`
	ActionCode      string     `json:"actionCode"      orm:"action_code"       description:""`
	SourceCode      string     `json:"sourceCode"      orm:"source_code"       description:""`
	Locale          string     `json:"locale"          orm:"locale"            description:""`
	EvidenceDigest  string     `json:"evidenceDigest"  orm:"evidence_digest"   description:"Digest of canonical consent evidence; raw request metadata remains outside ordinary fields"`
	OccurredAt      *time.Time `json:"occurredAt"      orm:"occurred_at"       description:""`
	RecordedBy      int        `json:"recordedBy"      orm:"recorded_by"       description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
}
