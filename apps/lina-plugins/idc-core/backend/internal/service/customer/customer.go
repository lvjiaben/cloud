// Package customer owns customer-subject profile access and self-service
// mutations inside the single-merchant IDC core plugin.
package customer

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/internal/service/reauth"
)

// Type is the immutable customer subject kind selected during onboarding.
type Type string

const (
	TypeIndividual Type = "individual"
	TypeEnterprise Type = "enterprise"
)

// Status is the customer account lifecycle state.
type Status string

const (
	StatusPendingActivation Status = "pending_activation"
	StatusActive            Status = "active"
	StatusRestricted        Status = "restricted"
	StatusDisabled          Status = "disabled"
	StatusAnonymized        Status = "anonymized"
)

// VerificationStatus is a personal or enterprise identity projection state.
type VerificationStatus string

const (
	VerificationUnverified VerificationStatus = "unverified"
	VerificationPending    VerificationStatus = "pending"
	VerificationVerified   VerificationStatus = "verified"
	VerificationRejected   VerificationStatus = "rejected"
	VerificationExpired    VerificationStatus = "expired"
	VerificationRevoked    VerificationStatus = "revoked"
)

// Subject is the non-credential customer account projection.
type Subject struct {
	CustomerNo       string
	CustomerCode     int64
	Type             Type
	Status           Status
	StatusReasonCode string
	StatusReason     string
	DisplayName      string
	CountryCode      string
	PreferredLocale  string
	Version          int64
	AnonymizedAt     *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IndividualProfile contains only legal identity projections safe for the
// customer profile domain. Raw identity numbers and documents are not stored.
type IndividualProfile struct {
	LegalName                string
	LegalNameMasked          string
	IdentityIdentifierMasked string
	IdentityCaseNo           string
	IdentityStatus           VerificationStatus
	IdentityVerifiedAt       *time.Time
	IdentityExpiresAt        *time.Time
	Version                  int64
	UpdatedAt                time.Time
}

// EnterpriseProfile contains the single-operator enterprise legal projection.
type EnterpriseProfile struct {
	LegalName                    string
	RegistrationCountryCode      string
	RegistrationIdentifierMasked string
	IndustryCode                 string
	VerificationCaseNo           string
	VerificationStatus           VerificationStatus
	VerifiedAt                   *time.Time
	ExpiresAt                    *time.Time
	Version                      int64
	UpdatedAt                    time.Time
}

// Profile is an immutable caller-owned subject and exactly one typed profile.
type Profile struct {
	Subject    Subject
	Individual *IndividualProfile
	Enterprise *EnterpriseProfile
}

// UpdateOwnBasicsInput changes only ordinary customer-maintained subject data.
// It cannot alter legal names, identity identifiers, verification, status, or
// business restrictions.
type UpdateOwnBasicsInput struct {
	CustomerNo      string
	DisplayName     string
	CountryCode     string
	PreferredLocale string
	ExpectedVersion int64
	IdempotencyKey  string
}

// LegalChangeReason classifies why a verified legal projection needs review.
// Proposed legal values and supporting documents remain compliance-owned and
// are deliberately absent from this domain contract.
type LegalChangeReason string

const (
	LegalChangeReasonNameChange             LegalChangeReason = "legal_name_change"
	LegalChangeReasonIdentifierCorrection   LegalChangeReason = "identifier_correction"
	LegalChangeReasonEnterpriseRegistration LegalChangeReason = "enterprise_registration_change"
	LegalChangeReasonOther                  LegalChangeReason = "other"
)

// LegalChangeField is a category-only declaration. It never carries the raw
// replacement legal name, identifier, document, or certification material.
type LegalChangeField string

const (
	LegalChangeFieldName       LegalChangeField = "legal_name"
	LegalChangeFieldIdentifier LegalChangeField = "legal_identifier"
)

// LegalChangeStatus is the retained review lifecycle.
type LegalChangeStatus string

const (
	LegalChangeStatusSubmitted   LegalChangeStatus = "submitted"
	LegalChangeStatusUnderReview LegalChangeStatus = "under_review"
	LegalChangeStatusApproved    LegalChangeStatus = "approved"
	LegalChangeStatusRejected    LegalChangeStatus = "rejected"
	LegalChangeStatusCancelled   LegalChangeStatus = "cancelled"
)

// RequestOwnLegalChangeInput opens a review request against the current
// verified projection without changing that projection.
type RequestOwnLegalChangeInput struct {
	CustomerNo             string
	Fields                 []LegalChangeField
	Reason                 LegalChangeReason
	ExpectedProfileVersion int64
	IdempotencyKey         string
}

// LegalChangeRequest is the category-only, non-credential workflow projection.
type LegalChangeRequest struct {
	RequestNo             string
	CustomerNo            string
	CustomerType          Type
	Status                LegalChangeStatus
	Reason                LegalChangeReason
	Fields                []LegalChangeField
	VerificationStatus    VerificationStatus
	VerificationCaseNo    string
	CurrentProfileVersion int64
	SubmittedByUserID     int
	SubmittedAt           time.Time
}

// OwnLegalChangeService is the focused customer-owned enterprise re-verification
// workflow. It cannot mutate account state or access export and closure flows.
type OwnLegalChangeService interface {
	CurrentProfile(ctx context.Context) (*Profile, error)
	CurrentOwnLegalChange(ctx context.Context) (*LegalChangeRequest, error)
	RequestOwnLegalChange(ctx context.Context, input RequestOwnLegalChangeInput) (*LegalChangeRequest, error)
}

// AccountTransitionReason is a bounded lifecycle reason, not free-form PII.
type AccountTransitionReason string

const (
	AccountReasonActivationCompleted  AccountTransitionReason = "activation_completed"
	AccountReasonVerificationRequired AccountTransitionReason = "verification_required"
	AccountReasonRiskRestriction      AccountTransitionReason = "risk_restriction"
	AccountReasonRestrictionResolved  AccountTransitionReason = "restriction_resolved"
	AccountReasonManualDisable        AccountTransitionReason = "manual_disable"
	AccountReasonAccountReactivated   AccountTransitionReason = "account_reactivated"
)

// TransitionAccountInput requests one explicit staff-controlled state change.
// Approved closure and migration transitions are intentionally not exposed by
// this contract and use their owning future workflows.
type TransitionAccountInput struct {
	CustomerNo      string
	ToStatus        Status
	Reason          AccountTransitionReason
	ExpectedVersion int64
	IdempotencyKey  string
}

// AccountStateTransition is retained append-only lifecycle evidence.
type AccountStateTransition struct {
	TransitionNo   string
	CustomerNo     string
	FromStatus     Status
	ToStatus       Status
	Reason         AccountTransitionReason
	ActorUserID    int
	SubjectVersion int64
	OccurredAt     time.Time
}

// ClosureFindingCode is one bounded cross-domain aggregate needed to decide
// whether a closure request may enter review.
type ClosureFindingCode string

const (
	ClosureFindingUnsettledFunds       ClosureFindingCode = "unsettled_funds"
	ClosureFindingActiveServices       ClosureFindingCode = "active_services"
	ClosureFindingUnsettledInvoices    ClosureFindingCode = "unsettled_invoices"
	ClosureFindingActiveContracts      ClosureFindingCode = "active_contracts"
	ClosureFindingInProgressOperations ClosureFindingCode = "in_progress_operations"
	ClosureFindingLegalRetention       ClosureFindingCode = "legal_retention"
)

// ClosureFinding contains an aggregate count only; it does not expose another
// domain's rows, identifiers, balances, documents, or evidence.
type ClosureFinding struct {
	Code  ClosureFindingCode
	Count int
}

// ClosurePrecheckResult is versioned so providers can evolve behind an
// application contract without customer-domain cross-table queries.
type ClosurePrecheckResult struct {
	CustomerNo      string
	ContractVersion int
	CheckedAt       time.Time
	Findings        []ClosureFinding
}

// ClosurePrecheckService is the explicit, injectable batch contract owned by
// customer workflows. Implementations aggregate their own domains in bounded
// set-based queries and return exactly one result per requested customer.
type ClosurePrecheckService interface {
	BatchCheck(ctx context.Context, customerNos []string) ([]ClosurePrecheckResult, error)
}

// ClosureStatus covers a transient blocked precheck outcome and the retained
// controlled request lifecycle. Self-service never anonymizes directly.
type ClosureStatus string

const (
	ClosureStatusBlocked       ClosureStatus = "blocked"
	ClosureStatusPendingReview ClosureStatus = "pending_review"
	ClosureStatusApproved      ClosureStatus = "approved"
	ClosureStatusRejected      ClosureStatus = "rejected"
	ClosureStatusAnonymizing   ClosureStatus = "anonymizing"
	ClosureStatusCompleted     ClosureStatus = "completed"
	ClosureStatusCancelled     ClosureStatus = "cancelled"
)

// RequestOwnClosureInput records explicit customer confirmation. Completion is
// a separate reviewed workflow that must rerun all prechecks before anonymizing.
type RequestOwnClosureInput struct {
	CustomerNo      string
	ExpectedVersion int64
	Confirmed       bool
	IdempotencyKey  string
}

// ClosureRequest returns either a transient blocked precheck or freezes the
// bounded precheck and retention decisions used for a retained submission.
// Legal retention is evidence preservation, not a blocker or deletion grant.
type ClosureRequest struct {
	RequestNo               string
	CustomerNo              string
	ConsentNo               string
	Status                  ClosureStatus
	SubjectVersion          int64
	Findings                []ClosureFinding
	PrecheckContractVersion int
	PrecheckedAt            time.Time
	RetentionPolicyCode     string
	AnonymizationPolicyCode string
	RequestedByUserID       int
	ConfirmedAt             time.Time
	RequestedAt             time.Time
}

// DataExportScope and DataExportFormat deliberately expose only the first
// release's bounded, customer-owned deliverable.
type DataExportScope string

// DataExportFormat is the private artifact container format.
type DataExportFormat string

const (
	DataExportScopeOwnDeliverable DataExportScope  = "own_deliverable_data"
	DataExportFormatJSONZip       DataExportFormat = "json_zip"
)

// OwnDataExportIntent is the immutable preview bound to recent-authentication.
type OwnDataExportIntent struct {
	CustomerNo      string
	Scope           DataExportScope
	Format          DataExportFormat
	ExpectedVersion int64
	IdempotencyKey  string
	Confirmed       bool
}

// RequestOwnDataExportInput consumes a single-use proof for the exact intent.
type RequestOwnDataExportInput struct {
	OwnDataExportIntent
	ProofToken string
}

// DataExportRequest links the retained request to the existing durable task
// service by opaque task number; artifact delivery remains private and bounded.
type DataExportRequest struct {
	RequestNo         string
	CustomerNo        string
	ConsentNo         string
	TaskNo            string
	Scope             DataExportScope
	Format            DataExportFormat
	SubjectVersion    int64
	RequestedByUserID int
	RequestedAt       time.Time
}

// Service exposes the narrow customer-owned profile and workflow boundary.
// Management reads and account-status transitions use separate staff contracts.
type Service interface {
	// CurrentProfile returns the current active customer audience's own profile.
	CurrentProfile(ctx context.Context) (*Profile, error)
	// GetOwnedProfile applies ownership before any database lookup, returning the
	// same inaccessible error for malformed, missing, or foreign customer numbers.
	GetOwnedProfile(ctx context.Context, customerNo string) (*Profile, error)
	// UpdateOwnBasics updates only ordinary self-service fields with optimistic
	// versioning and an audit event in the same transaction.
	UpdateOwnBasics(ctx context.Context, input UpdateOwnBasicsInput) (*Profile, error)
	// RequestOwnLegalChange records a manual-review request but never overwrites
	// the current certification projection or stores raw compliance material.
	RequestOwnLegalChange(ctx context.Context, input RequestOwnLegalChangeInput) (*LegalChangeRequest, error)
	// RequestOwnClosure runs the injected batch precheck and records a controlled
	// request. It never performs closure or anonymization directly.
	RequestOwnClosure(ctx context.Context, input RequestOwnClosureInput) (*ClosureRequest, error)
	// IssueOwnDataExportProof proves a recent full login for one exact export intent.
	IssueOwnDataExportProof(ctx context.Context, input OwnDataExportIntent) (*reauth.ProofContext, error)
	// RequestOwnDataExport consumes the bound proof and atomically creates the
	// export workflow row, durable async task, and audit transition.
	RequestOwnDataExport(ctx context.Context, input RequestOwnDataExportInput) (*DataExportRequest, error)
}

// AccountStateAuthorizer is the mandatory staff-side authorization seam. Its
// implementation must require the exact account-state permission and apply the
// target customer's host data-permission scope; generic IDC permissions are
// insufficient. Absence or uncertainty must return an error.
type AccountStateAuthorizer interface {
	AuthorizeTransition(ctx context.Context, customerNo string) error
}

// AccountLifecycleService is intentionally separate from the customer
// self-service capability so a portal controller can never acquire staff state
// mutation by depending on Service.
type AccountLifecycleService interface {
	TransitionAccount(ctx context.Context, input TransitionAccountInput) (*AccountStateTransition, error)
}
