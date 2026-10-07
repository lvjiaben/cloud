// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditAccountMovement is the golang structure for table credit_account_movement.
type CreditAccountMovement struct {
	Id                    int64      `json:"id"                    orm:"id"                       description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"                description:""`
	MovementNo            string     `json:"movementNo"            orm:"movement_no"              description:""`
	CreditAccountNo       string     `json:"creditAccountNo"       orm:"credit_account_no"        description:""`
	CustomerNo            string     `json:"customerNo"            orm:"customer_no"              description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"            description:""`
	ExposureNo            *string    `json:"exposureNo"            orm:"exposure_no"              description:""`
	StatementNo           *string    `json:"statementNo"           orm:"statement_no"             description:""`
	RepaymentNo           *string    `json:"repaymentNo"           orm:"repayment_no"             description:""`
	MovementTypeCode      string     `json:"movementTypeCode"      orm:"movement_type_code"       description:""`
	AmountMinor           int64      `json:"amountMinor"           orm:"amount_minor"             description:""`
	AccountVersionBefore  int64      `json:"accountVersionBefore"  orm:"account_version_before"   description:""`
	AccountVersionAfter   int64      `json:"accountVersionAfter"   orm:"account_version_after"    description:""`
	StatusBeforeCode      *string    `json:"statusBeforeCode"      orm:"status_before_code"       description:""`
	StatusAfterCode       string     `json:"statusAfterCode"       orm:"status_after_code"        description:""`
	TotalLimitBeforeMinor int64      `json:"totalLimitBeforeMinor" orm:"total_limit_before_minor" description:""`
	TotalLimitAfterMinor  int64      `json:"totalLimitAfterMinor"  orm:"total_limit_after_minor"  description:""`
	AvailableBeforeMinor  int64      `json:"availableBeforeMinor"  orm:"available_before_minor"   description:""`
	AvailableAfterMinor   int64      `json:"availableAfterMinor"   orm:"available_after_minor"    description:""`
	UnbilledBeforeMinor   int64      `json:"unbilledBeforeMinor"   orm:"unbilled_before_minor"    description:""`
	UnbilledAfterMinor    int64      `json:"unbilledAfterMinor"    orm:"unbilled_after_minor"     description:""`
	BilledBeforeMinor     int64      `json:"billedBeforeMinor"     orm:"billed_before_minor"      description:""`
	BilledAfterMinor      int64      `json:"billedAfterMinor"      orm:"billed_after_minor"       description:""`
	DelinquentBeforeMinor int64      `json:"delinquentBeforeMinor" orm:"delinquent_before_minor"  description:""`
	DelinquentAfterMinor  int64      `json:"delinquentAfterMinor"  orm:"delinquent_after_minor"   description:""`
	WrittenOffBeforeMinor int64      `json:"writtenOffBeforeMinor" orm:"written_off_before_minor" description:""`
	WrittenOffAfterMinor  int64      `json:"writtenOffAfterMinor"  orm:"written_off_after_minor"  description:""`
	SourceTypeCode        string     `json:"sourceTypeCode"        orm:"source_type_code"         description:""`
	SourceReference       string     `json:"sourceReference"       orm:"source_reference"         description:""`
	IdempotencyDigest     string     `json:"idempotencyDigest"     orm:"idempotency_digest"       description:""`
	EvidenceDigest        string     `json:"evidenceDigest"        orm:"evidence_digest"          description:""`
	ContentDigest         string     `json:"contentDigest"         orm:"content_digest"           description:""`
	ActorTypeCode         string     `json:"actorTypeCode"         orm:"actor_type_code"          description:""`
	ActorId               int        `json:"actorId"               orm:"actor_id"                 description:""`
	OccurredAt            *time.Time `json:"occurredAt"            orm:"occurred_at"              description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"               description:""`
}
