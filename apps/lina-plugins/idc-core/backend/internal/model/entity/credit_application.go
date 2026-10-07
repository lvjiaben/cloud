// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditApplication is the golang structure for table credit_application.
type CreditApplication struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	ApplicationNo         string     `json:"applicationNo"         orm:"application_no"          description:""`
	CustomerNo            string     `json:"customerNo"            orm:"customer_no"             description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"           description:""`
	PolicyNo              string     `json:"policyNo"              orm:"policy_no"               description:""`
	PolicyVersion         int64      `json:"policyVersion"         orm:"policy_version"          description:""`
	RequestedLimitMinor   int64      `json:"requestedLimitMinor"   orm:"requested_limit_minor"   description:""`
	RequestedValidFrom    *time.Time `json:"requestedValidFrom"    orm:"requested_valid_from"    description:""`
	RequestedValidUntil   *time.Time `json:"requestedValidUntil"   orm:"requested_valid_until"   description:""`
	RequiredApprovalCount int        `json:"requiredApprovalCount" orm:"required_approval_count" description:""`
	RiskEvidenceReference string     `json:"riskEvidenceReference" orm:"risk_evidence_reference" description:""`
	RiskEvidenceDigest    string     `json:"riskEvidenceDigest"    orm:"risk_evidence_digest"    description:""`
	RequestDigest         string     `json:"requestDigest"         orm:"request_digest"          description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	RequestActorTypeCode  string     `json:"requestActorTypeCode"  orm:"request_actor_type_code" description:""`
	RequestedBy           int        `json:"requestedBy"           orm:"requested_by"            description:""`
	DecidedAt             *time.Time `json:"decidedAt"             orm:"decided_at"              description:""`
	CancelledAt           *time.Time `json:"cancelledAt"           orm:"cancelled_at"            description:""`
	Version               int64      `json:"version"               orm:"version"                 description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"              description:""`
}
