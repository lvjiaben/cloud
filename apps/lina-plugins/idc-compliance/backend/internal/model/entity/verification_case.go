// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// VerificationCase is the golang structure for table verification_case.
type VerificationCase struct {
	Id                             int64      `json:"id"                             orm:"id"                                description:""`
	TenantId                       int        `json:"tenantId"                       orm:"tenant_id"                         description:""`
	CaseNo                         string     `json:"caseNo"                         orm:"case_no"                           description:""`
	CustomerNo                     string     `json:"customerNo"                     orm:"customer_no"                       description:""`
	PrincipalNo                    string     `json:"principalNo"                    orm:"principal_no"                      description:""`
	SubjectTypeCode                string     `json:"subjectTypeCode"                orm:"subject_type_code"                 description:""`
	ProviderCode                   string     `json:"providerCode"                   orm:"provider_code"                     description:""`
	StatusCode                     string     `json:"statusCode"                     orm:"status_code"                       description:""`
	ConsentNo                      string     `json:"consentNo"                      orm:"consent_no"                        description:""`
	ProviderCaseRef                *string    `json:"providerCaseRef"                orm:"provider_case_ref"                 description:""`
	ProviderSubjectRef             *string    `json:"providerSubjectRef"             orm:"provider_subject_ref"              description:""`
	LegalProfileSnapshotRef        *string    `json:"legalProfileSnapshotRef"        orm:"legal_profile_snapshot_ref"        description:"Private encrypted object reference; identity documents, face photos, face videos, and raw provider payloads are forbidden in this table"`
	LegalProfileSha256             *string    `json:"legalProfileSha256"             orm:"legal_profile_sha256"              description:"SHA-256 of the authenticated encrypted legal-profile envelope; never a digest of legal plaintext"`
	MaskedName                     string     `json:"maskedName"                     orm:"masked_name"                       description:""`
	MaskedIdentityNo               string     `json:"maskedIdentityNo"               orm:"masked_identity_no"                description:""`
	IdentityLookupDigest           *string    `json:"identityLookupDigest"           orm:"identity_lookup_digest"            description:"Keyed lookup digest used for duplicate-risk checks; raw identity numbers and unsalted public hashes are forbidden"`
	DecisionReasonCode             string     `json:"decisionReasonCode"             orm:"decision_reason_code"              description:""`
	SubmittedAt                    *time.Time `json:"submittedAt"                    orm:"submitted_at"                      description:""`
	VerifiedAt                     *time.Time `json:"verifiedAt"                     orm:"verified_at"                       description:""`
	RejectedAt                     *time.Time `json:"rejectedAt"                     orm:"rejected_at"                       description:""`
	ExpiresAt                      *time.Time `json:"expiresAt"                      orm:"expires_at"                        description:""`
	DecidedByPrincipalNo           *string    `json:"decidedByPrincipalNo"           orm:"decided_by_principal_no"           description:""`
	Version                        int64      `json:"version"                        orm:"version"                           description:""`
	CreatedAt                      *time.Time `json:"createdAt"                      orm:"created_at"                        description:""`
	UpdatedAt                      *time.Time `json:"updatedAt"                      orm:"updated_at"                        description:""`
	LegalChangeRequestNo           string     `json:"legalChangeRequestNo"           orm:"legal_change_request_no"           description:""`
	RequestIdempotencyKeyDigest    string     `json:"requestIdempotencyKeyDigest"    orm:"request_idempotency_key_digest"    description:""`
	RequestFingerprint             string     `json:"requestFingerprint"             orm:"request_fingerprint"               description:""`
	SubmissionIdempotencyKeyDigest string     `json:"submissionIdempotencyKeyDigest" orm:"submission_idempotency_key_digest" description:""`
	SubmissionFingerprint          string     `json:"submissionFingerprint"          orm:"submission_fingerprint"            description:""`
	EncryptionKeyNo                *string    `json:"encryptionKeyNo"                orm:"encryption_key_no"                 description:""`
	SnapshotStoredByteSize         int64      `json:"snapshotStoredByteSize"         orm:"snapshot_stored_byte_size"         description:""`
	ApplicantRoleCode              string     `json:"applicantRoleCode"              orm:"applicant_role_code"               description:""`
	ReviewIdempotencyKeyDigest     string     `json:"reviewIdempotencyKeyDigest"     orm:"review_idempotency_key_digest"     description:"SHA-256 of the bounded review-start idempotency key; the raw key is never persisted"`
	ReviewFingerprint              string     `json:"reviewFingerprint"              orm:"review_fingerprint"                description:""`
	DecisionIdempotencyKeyDigest   string     `json:"decisionIdempotencyKeyDigest"   orm:"decision_idempotency_key_digest"   description:"SHA-256 of the bounded decision idempotency key; the raw key is never persisted"`
	DecisionFingerprint            string     `json:"decisionFingerprint"            orm:"decision_fingerprint"              description:""`
	DecisionActionCode             string     `json:"decisionActionCode"             orm:"decision_action_code"              description:""`
	RetentionUntil                 *time.Time `json:"retentionUntil"                 orm:"retention_until"                   description:""`
	LegalHold                      bool       `json:"legalHold"                      orm:"legal_hold"                        description:""`
}
