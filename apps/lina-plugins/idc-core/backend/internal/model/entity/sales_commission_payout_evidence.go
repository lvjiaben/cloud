// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionPayoutEvidence is the golang structure for table sales_commission_payout_evidence.
type SalesCommissionPayoutEvidence struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	EvidenceNo               string     `json:"evidenceNo"               orm:"evidence_no"                description:""`
	PayoutItemNo             string     `json:"payoutItemNo"             orm:"payout_item_no"             description:""`
	EvidenceTypeCode         string     `json:"evidenceTypeCode"         orm:"evidence_type_code"         description:""`
	ProtectedObjectReference *string    `json:"protectedObjectReference" orm:"protected_object_reference" description:""`
	EvidenceDigest           string     `json:"evidenceDigest"           orm:"evidence_digest"            description:""`
	ProviderReferenceDigest  *string    `json:"providerReferenceDigest"  orm:"provider_reference_digest"  description:""`
	ObservedAt               *time.Time `json:"observedAt"               orm:"observed_at"                description:""`
}
