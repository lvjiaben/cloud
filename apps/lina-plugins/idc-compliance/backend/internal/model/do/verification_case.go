// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// VerificationCase is the golang structure of table plugin_idc_compliance_verification_case for DAO operations like Where/Data.
type VerificationCase struct {
	g.Meta                         `orm:"table:plugin_idc_compliance_verification_case, do:true"`
	Id                             any        //
	TenantId                       any        //
	CaseNo                         any        //
	CustomerNo                     any        //
	PrincipalNo                    any        //
	SubjectTypeCode                any        //
	ProviderCode                   any        //
	StatusCode                     any        //
	ConsentNo                      any        //
	ProviderCaseRef                *string    //
	ProviderSubjectRef             *string    //
	LegalProfileSnapshotRef        *string    // Private encrypted object reference; identity documents, face photos, face videos, and raw provider payloads are forbidden in this table
	LegalProfileSha256             *string    // SHA-256 of the authenticated encrypted legal-profile envelope; never a digest of legal plaintext
	MaskedName                     any        //
	MaskedIdentityNo               any        //
	IdentityLookupDigest           *string    // Keyed lookup digest used for duplicate-risk checks; raw identity numbers and unsalted public hashes are forbidden
	DecisionReasonCode             any        //
	SubmittedAt                    *time.Time //
	VerifiedAt                     *time.Time //
	RejectedAt                     *time.Time //
	ExpiresAt                      *time.Time //
	DecidedByPrincipalNo           *string    //
	Version                        any        //
	CreatedAt                      *time.Time //
	UpdatedAt                      *time.Time //
	LegalChangeRequestNo           any        //
	RequestIdempotencyKeyDigest    any        //
	RequestFingerprint             any        //
	SubmissionIdempotencyKeyDigest any        //
	SubmissionFingerprint          any        //
	EncryptionKeyNo                *string    //
	SnapshotStoredByteSize         any        //
	ApplicantRoleCode              any        //
	ReviewIdempotencyKeyDigest     any        // SHA-256 of the bounded review-start idempotency key; the raw key is never persisted
	ReviewFingerprint              any        //
	DecisionIdempotencyKeyDigest   any        // SHA-256 of the bounded decision idempotency key; the raw key is never persisted
	DecisionFingerprint            any        //
	DecisionActionCode             any        //
	RetentionUntil                 *time.Time //
	LegalHold                      any        //
}
