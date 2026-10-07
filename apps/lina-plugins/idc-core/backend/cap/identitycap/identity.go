// Package identitycap defines the source-only customer identity projection boundary.
//
// The capability is deliberately unavailable to dynamic plugins and HTTP. It
// lets the compliance source plugin prove the current personal or enterprise
// customer and an effective purpose-specific consent, then publish only a
// bounded local payment-gate projection back to the core owner.
package identitycap

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

// ErrDuplicateIdentity reports a database uniqueness-fence conflict while the
// caller owns the ambient transaction. The caller must let that transaction
// roll back before committing a review-required outcome in a fresh transaction.
var ErrDuplicateIdentity = errors.New("personal identity uniqueness fence conflict")

// ErrDuplicateEnterprise reports a database uniqueness-fence conflict for an
// enterprise registration identifier. The ambient transaction must roll back
// before the caller records a review-required outcome.
var ErrDuplicateEnterprise = errors.New("enterprise registration uniqueness fence conflict")

const (
	// OwnerPluginID is the only source owner of the identity projection.
	OwnerPluginID = "idc-core"
	// AllowedCallerPluginID is the only source plugin composed with this boundary.
	AllowedCallerPluginID = "idc-compliance"
	// PersonalConsentPolicyReference is the immutable policy identity for the
	// separate sensitive-information decision collected before verification.
	PersonalConsentPolicyReference = "idc://compliance/personal-verification-sensitive-information"
	// PersonalConsentPolicyVersion is the policy revision currently accepted by
	// the personal-verification workflow.
	PersonalConsentPolicyVersion = "1"
	// EnterpriseConsentPolicyReference identifies the complete enterprise
	// verification notice covering purpose, necessity, recipients, storage,
	// retention, impact, and data-subject rights.
	EnterpriseConsentPolicyReference = "idc://compliance/enterprise-verification"
	// EnterpriseConsentPolicyVersion is the accepted enterprise notice revision.
	EnterpriseConsentPolicyVersion = "1"
)

// Status is the bounded core payment-gate identity status.
type Status string

const (
	// StatusUnverified is the initial customer-visible projection state.
	StatusUnverified Status = "unverified"
	StatusPending    Status = "pending"
	StatusVerified   Status = "verified"
	StatusRejected   Status = "rejected"
	StatusExpired    Status = "expired"
	// StatusRevoked is a customer-visible state that requires a new verification.
	StatusRevoked Status = "revoked"
)

// PersonalSubject is the minimum authenticated subject projection needed to
// create one compliance-owned verification case.
type PersonalSubject struct {
	CustomerNo         string
	PrincipalReference string
	ActorUserID        int
}

// PersonalConsentInput records one separate sensitive-information grant. The
// raw idempotency key is used only to reserve an opaque business number and is
// never persisted.
type PersonalConsentInput struct {
	PolicyVersion  string
	Locale         string
	IdempotencyKey string
}

// PersonalConsent is the bounded evidence returned to the current customer.
type PersonalConsent struct {
	ConsentNo     string
	PolicyVersion string
	OccurredAt    time.Time
	Reused        bool
}

// PersonalProjectionInput is consumed synchronously by ProjectPersonal.
// LegalName and IdentityNumber are required only for StatusVerified, are never
// persisted verbatim outside the core legal profile, and are cleared before
// the method returns.
type PersonalProjectionInput struct {
	CustomerNo       string
	CaseNo           string
	Status           Status
	LegalName        []byte
	MaskedName       string
	IdentityNumber   []byte
	MaskedIdentityNo string
	VerifiedAt       *time.Time
	ExpiresAt        *time.Time
}

// PersonalProjection is the safe owner result. DuplicateRisk means another
// customer already owns the same keyed identity digest and no verified update
// was written.
type PersonalProjection struct {
	Status               Status
	IdentityLookupDigest string
	Version              int64
	DuplicateRisk        bool
	Reused               bool
}

// EnterpriseSubject is the minimum authenticated enterprise projection needed
// to create one compliance-owned manual-review case.
type EnterpriseSubject struct {
	CustomerNo         string
	PrincipalReference string
	ActorUserID        int
}

// CustomerSubject is the minimum authenticated customer projection for
// customer-owned resources that apply to both personal and enterprise accounts.
type CustomerSubject struct {
	CustomerNo         string
	CustomerTypeCode   string
	PrincipalReference string
	ActorUserID        int
}

// CurrentCustomerReader proves the current customer without accepting a
// customer identifier from an HTTP request.
type CurrentCustomerReader interface {
	CurrentCustomer(context.Context) (*CustomerSubject, error)
}

// EnterpriseConsentInput records the enterprise verification notice grant.
// The raw idempotency key is never persisted.
type EnterpriseConsentInput struct {
	PolicyVersion  string
	Locale         string
	IdempotencyKey string
}

// EnterpriseConsent is bounded consent evidence returned to the customer.
type EnterpriseConsent struct {
	ConsentNo     string
	PolicyVersion string
	OccurredAt    time.Time
	Reused        bool
}

// PrivacyConsentType is one customer-withdrawable future-processing grant.
type PrivacyConsentType string

const (
	PrivacyConsentSensitivePersonal PrivacyConsentType = "sensitive_personal_information"
	PrivacyConsentEnterprise        PrivacyConsentType = "enterprise_verification"
)

// PrivacyConsentDecision is the latest append-only decision for one purpose.
type PrivacyConsentDecision struct {
	ConsentNo       string
	ConsentType     PrivacyConsentType
	PolicyReference string
	PolicyVersion   string
	ActionCode      string
	Locale          string
	OccurredAt      time.Time
	Reused          bool
}

// PrivacyConsentWithdrawalInput withdraws only future processing that relies
// on one purpose-specific grant. Historical legal evidence remains untouched.
type PrivacyConsentWithdrawalInput struct {
	ConsentType    PrivacyConsentType
	IdempotencyKey string
}

// PrivacyConsentService is a separate source-only seam so existing identity
// fakes are not forced to implement customer privacy-center behavior.
type PrivacyConsentService interface {
	ListCurrentPrivacyConsents(context.Context) ([]PrivacyConsentDecision, error)
	WithdrawCurrentPrivacyConsent(
		context.Context,
		PrivacyConsentWithdrawalInput,
	) (*PrivacyConsentDecision, error)
}

// EnterprisePreparation proves the current customer, consent, and optional
// core-owned legal-change workflow without exposing proposed legal values.
type EnterprisePreparation struct {
	Subject               EnterpriseSubject
	LegalChangeRequestNo  string
	ReviewCaseNo          string
	CurrentCaseNo         string
	CurrentProfileVersion int64
	Reverification        bool
}

// EnterpriseProjectionInput is consumed synchronously by ProjectEnterprise.
// Raw legal fields are required only for a verified decision and are cleared
// before return. LegalChangeRequestNo is set only for re-verification.
type EnterpriseProjectionInput struct {
	CustomerNo                   string
	CaseNo                       string
	Status                       Status
	LegalChangeRequestNo         string
	ReviewActorUserID            int
	DecisionReasonCode           string
	LegalName                    []byte
	RegistrationCountryCode      string
	RegistrationIdentifier       []byte
	MaskedRegistrationIdentifier string
	IndustryCode                 string
	VerifiedAt                   *time.Time
	ExpiresAt                    *time.Time
}

// EnterpriseProjection is the safe core-owned verification result. For a
// re-verification pending/rejected result, Version remains the unchanged
// verified profile version and PreservedCurrent is true.
type EnterpriseProjection struct {
	Status                   Status
	RegistrationLookupDigest string
	Version                  int64
	DuplicateRisk            bool
	PreservedCurrent         bool
	Reused                   bool
}

// EnterpriseLegalChangeField declares one changed legal category without its value.
type EnterpriseLegalChangeField string

const (
	EnterpriseLegalNameField       EnterpriseLegalChangeField = "legal_name"
	EnterpriseLegalIdentifierField EnterpriseLegalChangeField = "legal_identifier"
)

// EnterpriseLegalChangeReason is the closed customer-visible change reason.
type EnterpriseLegalChangeReason string

const (
	EnterpriseLegalNameChangeReason      EnterpriseLegalChangeReason = "legal_name_change"
	EnterpriseIdentifierCorrectionReason EnterpriseLegalChangeReason = "identifier_correction"
	EnterpriseRegistrationChangeReason   EnterpriseLegalChangeReason = "enterprise_registration_change"
	EnterpriseOtherLegalChangeReason     EnterpriseLegalChangeReason = "other"
)

// EnterpriseLegalChangeStatus is the core-owned workflow lifecycle.
type EnterpriseLegalChangeStatus string

const (
	EnterpriseLegalChangeSubmitted   EnterpriseLegalChangeStatus = "submitted"
	EnterpriseLegalChangeUnderReview EnterpriseLegalChangeStatus = "under_review"
	EnterpriseLegalChangeApproved    EnterpriseLegalChangeStatus = "approved"
	EnterpriseLegalChangeRejected    EnterpriseLegalChangeStatus = "rejected"
	EnterpriseLegalChangeCancelled   EnterpriseLegalChangeStatus = "cancelled"
)

// EnterpriseLegalChangeInput opens a category-only re-verification request.
// The current customer is derived from the trusted request context.
type EnterpriseLegalChangeInput struct {
	Fields                 []EnterpriseLegalChangeField
	Reason                 EnterpriseLegalChangeReason
	ExpectedProfileVersion int64
	IdempotencyKey         string
}

// EnterpriseLegalChange is the safe active re-verification workflow projection.
type EnterpriseLegalChange struct {
	RequestNo             string
	Status                EnterpriseLegalChangeStatus
	Reason                EnterpriseLegalChangeReason
	Fields                []EnterpriseLegalChangeField
	VerificationCaseNo    string
	CurrentProfileVersion int64
	SubmittedAt           time.Time
}

// EnterpriseAccountState is the current enterprise's own legal projection plus
// at most one active category-only legal-change request.
type EnterpriseAccountState struct {
	VerificationCaseNo           string
	VerificationStatus           Status
	LegalName                    string
	RegistrationCountryCode      string
	RegistrationIdentifierMasked string
	IndustryCode                 string
	VerifiedAt                   *time.Time
	ExpiresAt                    *time.Time
	ProfileVersion               int64
	ActiveLegalChange            *EnterpriseLegalChange
}

// EnterpriseAccountService exposes only the customer-owned state and legal-change
// entry needed by the compliance Portal. It never accepts legal replacement values.
type EnterpriseAccountService interface {
	CurrentEnterpriseAccount(context.Context) (*EnterpriseAccountState, error)
	RequestEnterpriseLegalChange(
		context.Context,
		EnterpriseLegalChangeInput,
	) (*EnterpriseLegalChange, error)
}

// Runtime is the complete source-only seam composed for the compliance owner.
// CustomerAudit resolves only the authenticated requester. SystemAudit resolves
// only the fixed recovery worker identity selected by core at construction.
type Runtime struct {
	Identity          Service
	EnterpriseAccount EnterpriseAccountService
	CustomerAudit     idccap.BusinessAuditWriter
	SystemAudit       idccap.BusinessAuditWriter
}

// ValidateRuntime rejects absent and typed-nil source dependencies.
func ValidateRuntime(runtime Runtime) error {
	if nilRuntimeDependency(runtime.Identity) || nilRuntimeDependency(runtime.EnterpriseAccount) ||
		nilRuntimeDependency(runtime.CustomerAudit) ||
		nilRuntimeDependency(runtime.SystemAudit) {
		return gerror.New("customer identity runtime is incomplete")
	}
	return nil
}

// Service is an in-process source capability. It is not registered as a
// dynamic host service because raw identity input must never cross a serialized
// plugin bridge.
type Service interface {
	// CurrentPersonal proves and returns the current eligible individual customer
	// without evaluating a new consent decision.
	CurrentPersonal(ctx context.Context) (*PersonalSubject, error)
	// RecordPersonalConsent appends or safely replays the current customer's
	// separate sensitive-personal-information grant for the fixed policy.
	RecordPersonalConsent(ctx context.Context, input PersonalConsentInput) (*PersonalConsent, error)
	// PreparePersonal proves the current active individual customer and verifies
	// that consentNo is that customer's latest sensitive-information decision.
	PreparePersonal(ctx context.Context, consentNo string) (*PersonalSubject, error)
	// ProjectPersonal applies one monotonic compliance case projection. Changed
	// case numbers cannot overwrite an in-flight or verified profile. When the
	// context carries a transaction for the shared default database group, the
	// projection joins it so the compliance terminal state and audit can commit
	// atomically with the payment-gate projection.
	ProjectPersonal(ctx context.Context, input PersonalProjectionInput) (*PersonalProjection, error)
	// CurrentEnterprise proves the current eligible enterprise customer.
	CurrentEnterprise(ctx context.Context) (*EnterpriseSubject, error)
	// RecordEnterpriseConsent appends or replays the fixed enterprise notice grant.
	RecordEnterpriseConsent(ctx context.Context, input EnterpriseConsentInput) (*EnterpriseConsent, error)
	// PrepareEnterprise proves the latest consent and, when supplied, validates
	// one active category-only legal-change request owned by this customer.
	PrepareEnterprise(
		ctx context.Context,
		consentNo string,
		legalChangeRequestNo string,
	) (*EnterprisePreparation, error)
	// ProjectEnterprise applies an initial or re-verification decision. During
	// re-verification, pending and rejected outcomes never overwrite the current
	// verified legal projection.
	ProjectEnterprise(
		ctx context.Context,
		input EnterpriseProjectionInput,
	) (*EnterpriseProjection, error)
}

func nilRuntimeDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
