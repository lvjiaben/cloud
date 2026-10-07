// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TrialQualificationMovement is the golang structure for table trial_qualification_movement.
type TrialQualificationMovement struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	MovementNo         string     `json:"movementNo"         orm:"movement_no"          description:""`
	ScopeNo            string     `json:"scopeNo"            orm:"scope_no"             description:""`
	CustomerNo         string     `json:"customerNo"         orm:"customer_no"          description:""`
	ProductNo          string     `json:"productNo"          orm:"product_no"           description:""`
	OrderNo            string     `json:"orderNo"            orm:"order_no"             description:""`
	MovementTypeCode   string     `json:"movementTypeCode"   orm:"movement_type_code"   description:""`
	Quantity           int        `json:"quantity"           orm:"quantity"             description:""`
	HeldBefore         int        `json:"heldBefore"         orm:"held_before"          description:""`
	HeldAfter          int        `json:"heldAfter"          orm:"held_after"           description:""`
	ConsumedBefore     int        `json:"consumedBefore"     orm:"consumed_before"      description:""`
	ConsumedAfter      int        `json:"consumedAfter"      orm:"consumed_after"       description:""`
	ScopeVersionBefore int64      `json:"scopeVersionBefore" orm:"scope_version_before" description:""`
	ScopeVersionAfter  int64      `json:"scopeVersionAfter"  orm:"scope_version_after"  description:""`
	EvidenceDigest     string     `json:"evidenceDigest"     orm:"evidence_digest"      description:""`
	ContentDigest      string     `json:"contentDigest"      orm:"content_digest"       description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
}
