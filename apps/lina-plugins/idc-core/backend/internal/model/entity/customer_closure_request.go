// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerClosureRequest is the golang structure for table customer_closure_request.
type CustomerClosureRequest struct {
	Id                       int64      `json:"id"                       orm:"id"                          description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                   description:""`
	RequestNo                string     `json:"requestNo"                orm:"request_no"                  description:""`
	CustomerNo               string     `json:"customerNo"               orm:"customer_no"                 description:""`
	ConsentNo                string     `json:"consentNo"                orm:"consent_no"                  description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                 description:""`
	SubjectVersion           int64      `json:"subjectVersion"           orm:"subject_version"             description:""`
	UnsettledFundsCount      int        `json:"unsettledFundsCount"      orm:"unsettled_funds_count"       description:""`
	ActiveServiceCount       int        `json:"activeServiceCount"       orm:"active_service_count"        description:""`
	UnsettledInvoiceCount    int        `json:"unsettledInvoiceCount"    orm:"unsettled_invoice_count"     description:""`
	ActiveContractCount      int        `json:"activeContractCount"      orm:"active_contract_count"       description:""`
	InProgressOperationCount int        `json:"inProgressOperationCount" orm:"in_progress_operation_count" description:""`
	LegalRetentionCount      int        `json:"legalRetentionCount"      orm:"legal_retention_count"       description:"Retention findings do not by themselves authorize deletion or bypass closure review; legally retained evidence survives ordinary-profile anonymization"`
	PrecheckContractVersion  int        `json:"precheckContractVersion"  orm:"precheck_contract_version"   description:""`
	PrecheckedAt             *time.Time `json:"precheckedAt"             orm:"prechecked_at"               description:""`
	RetentionPolicyCode      string     `json:"retentionPolicyCode"      orm:"retention_policy_code"       description:""`
	AnonymizationPolicyCode  string     `json:"anonymizationPolicyCode"  orm:"anonymization_policy_code"   description:""`
	IdempotencyKeyDigest     string     `json:"idempotencyKeyDigest"     orm:"idempotency_key_digest"      description:""`
	RequestFingerprint       string     `json:"requestFingerprint"       orm:"request_fingerprint"         description:""`
	RequestedByUserId        int        `json:"requestedByUserId"        orm:"requested_by_user_id"        description:""`
	ConfirmedAt              *time.Time `json:"confirmedAt"              orm:"confirmed_at"                description:""`
	RequestedAt              *time.Time `json:"requestedAt"              orm:"requested_at"                description:""`
	ReviewedByUserId         int        `json:"reviewedByUserId"         orm:"reviewed_by_user_id"         description:""`
	ReviewedAt               *time.Time `json:"reviewedAt"               orm:"reviewed_at"                 description:""`
	CompletedAt              *time.Time `json:"completedAt"              orm:"completed_at"                description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                  description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                  description:""`
}
