// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// VerificationCaseDao is the data access object for the table plugin_idc_compliance_verification_case.
type VerificationCaseDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  VerificationCaseColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// VerificationCaseColumns defines and stores column names for the table plugin_idc_compliance_verification_case.
type VerificationCaseColumns struct {
	Id                             string //
	TenantId                       string //
	CaseNo                         string //
	CustomerNo                     string //
	PrincipalNo                    string //
	SubjectTypeCode                string //
	ProviderCode                   string //
	StatusCode                     string //
	ConsentNo                      string //
	ProviderCaseRef                string //
	ProviderSubjectRef             string //
	LegalProfileSnapshotRef        string // Private encrypted object reference; identity documents, face photos, face videos, and raw provider payloads are forbidden in this table
	LegalProfileSha256             string // SHA-256 of the authenticated encrypted legal-profile envelope; never a digest of legal plaintext
	MaskedName                     string //
	MaskedIdentityNo               string //
	IdentityLookupDigest           string // Keyed lookup digest used for duplicate-risk checks; raw identity numbers and unsalted public hashes are forbidden
	DecisionReasonCode             string //
	SubmittedAt                    string //
	VerifiedAt                     string //
	RejectedAt                     string //
	ExpiresAt                      string //
	DecidedByPrincipalNo           string //
	Version                        string //
	CreatedAt                      string //
	UpdatedAt                      string //
	LegalChangeRequestNo           string //
	RequestIdempotencyKeyDigest    string //
	RequestFingerprint             string //
	SubmissionIdempotencyKeyDigest string //
	SubmissionFingerprint          string //
	EncryptionKeyNo                string //
	SnapshotStoredByteSize         string //
	ApplicantRoleCode              string //
	ReviewIdempotencyKeyDigest     string // SHA-256 of the bounded review-start idempotency key; the raw key is never persisted
	ReviewFingerprint              string //
	DecisionIdempotencyKeyDigest   string // SHA-256 of the bounded decision idempotency key; the raw key is never persisted
	DecisionFingerprint            string //
	DecisionActionCode             string //
	RetentionUntil                 string //
	LegalHold                      string //
}

// verificationCaseColumns holds the columns for the table plugin_idc_compliance_verification_case.
var verificationCaseColumns = VerificationCaseColumns{
	Id:                             "id",
	TenantId:                       "tenant_id",
	CaseNo:                         "case_no",
	CustomerNo:                     "customer_no",
	PrincipalNo:                    "principal_no",
	SubjectTypeCode:                "subject_type_code",
	ProviderCode:                   "provider_code",
	StatusCode:                     "status_code",
	ConsentNo:                      "consent_no",
	ProviderCaseRef:                "provider_case_ref",
	ProviderSubjectRef:             "provider_subject_ref",
	LegalProfileSnapshotRef:        "legal_profile_snapshot_ref",
	LegalProfileSha256:             "legal_profile_sha256",
	MaskedName:                     "masked_name",
	MaskedIdentityNo:               "masked_identity_no",
	IdentityLookupDigest:           "identity_lookup_digest",
	DecisionReasonCode:             "decision_reason_code",
	SubmittedAt:                    "submitted_at",
	VerifiedAt:                     "verified_at",
	RejectedAt:                     "rejected_at",
	ExpiresAt:                      "expires_at",
	DecidedByPrincipalNo:           "decided_by_principal_no",
	Version:                        "version",
	CreatedAt:                      "created_at",
	UpdatedAt:                      "updated_at",
	LegalChangeRequestNo:           "legal_change_request_no",
	RequestIdempotencyKeyDigest:    "request_idempotency_key_digest",
	RequestFingerprint:             "request_fingerprint",
	SubmissionIdempotencyKeyDigest: "submission_idempotency_key_digest",
	SubmissionFingerprint:          "submission_fingerprint",
	EncryptionKeyNo:                "encryption_key_no",
	SnapshotStoredByteSize:         "snapshot_stored_byte_size",
	ApplicantRoleCode:              "applicant_role_code",
	ReviewIdempotencyKeyDigest:     "review_idempotency_key_digest",
	ReviewFingerprint:              "review_fingerprint",
	DecisionIdempotencyKeyDigest:   "decision_idempotency_key_digest",
	DecisionFingerprint:            "decision_fingerprint",
	DecisionActionCode:             "decision_action_code",
	RetentionUntil:                 "retention_until",
	LegalHold:                      "legal_hold",
}

// NewVerificationCaseDao creates and returns a new DAO object for table data access.
func NewVerificationCaseDao(handlers ...gdb.ModelHandler) *VerificationCaseDao {
	return &VerificationCaseDao{
		group:    "default",
		table:    "plugin_idc_compliance_verification_case",
		columns:  verificationCaseColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *VerificationCaseDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *VerificationCaseDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *VerificationCaseDao) Columns() VerificationCaseColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *VerificationCaseDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *VerificationCaseDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *VerificationCaseDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
