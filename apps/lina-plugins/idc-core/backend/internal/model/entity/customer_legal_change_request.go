// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerLegalChangeRequest is the golang structure for table customer_legal_change_request.
type CustomerLegalChangeRequest struct {
	Id                            int64      `json:"id"                            orm:"id"                               description:""`
	TenantId                      int        `json:"tenantId"                      orm:"tenant_id"                        description:""`
	RequestNo                     string     `json:"requestNo"                     orm:"request_no"                       description:""`
	CustomerNo                    string     `json:"customerNo"                    orm:"customer_no"                      description:""`
	CustomerTypeCode              string     `json:"customerTypeCode"              orm:"customer_type_code"               description:""`
	StatusCode                    string     `json:"statusCode"                    orm:"status_code"                      description:""`
	ReasonCode                    string     `json:"reasonCode"                    orm:"reason_code"                      description:""`
	ChangeLegalName               bool       `json:"changeLegalName"               orm:"change_legal_name"                description:"Declares a requested field category without storing the proposed raw legal value"`
	ChangeLegalIdentifier         bool       `json:"changeLegalIdentifier"         orm:"change_legal_identifier"          description:"Declares a requested identifier category without storing a raw identifier or document"`
	CurrentVerificationStatusCode string     `json:"currentVerificationStatusCode" orm:"current_verification_status_code" description:""`
	CurrentVerificationCaseNo     string     `json:"currentVerificationCaseNo"     orm:"current_verification_case_no"     description:"Opaque reference to the verification case whose result remains unchanged while this request is reviewed"`
	CurrentProfileVersion         int64      `json:"currentProfileVersion"         orm:"current_profile_version"          description:""`
	IdempotencyKeyDigest          string     `json:"idempotencyKeyDigest"          orm:"idempotency_key_digest"           description:""`
	RequestFingerprint            string     `json:"requestFingerprint"            orm:"request_fingerprint"              description:""`
	SubmittedByUserId             int        `json:"submittedByUserId"             orm:"submitted_by_user_id"             description:""`
	SubmittedAt                   *time.Time `json:"submittedAt"                   orm:"submitted_at"                     description:""`
	ReviewedByUserId              int        `json:"reviewedByUserId"              orm:"reviewed_by_user_id"              description:""`
	ReviewedAt                    *time.Time `json:"reviewedAt"                    orm:"reviewed_at"                      description:""`
	ResolutionCode                string     `json:"resolutionCode"                orm:"resolution_code"                  description:""`
	CreatedAt                     *time.Time `json:"createdAt"                     orm:"created_at"                       description:""`
	UpdatedAt                     *time.Time `json:"updatedAt"                     orm:"updated_at"                       description:""`
	ReviewVerificationCaseNo      string     `json:"reviewVerificationCaseNo"      orm:"review_verification_case_no"      description:"Opaque compliance case performing this legal-data review; proposed values and materials remain compliance-owned"`
}
