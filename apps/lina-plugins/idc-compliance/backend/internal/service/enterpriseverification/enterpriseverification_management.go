// This file defines the staff-only enterprise-verification review boundary.

package enterpriseverification

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// The permission catalog and hard bounds below define the complete staff-review envelope.
	permissionEnterpriseCaseList     = "idc:admin:compliance:enterprise-verification:list"
	permissionEnterpriseCaseQuery    = "idc:admin:compliance:enterprise-verification:query"
	permissionEnterpriseCaseReview   = "idc:admin:compliance:enterprise-verification:review"
	permissionEnterpriseCaseApprove  = "idc:admin:compliance:enterprise-verification:approve"
	permissionEnterpriseCaseReject   = "idc:admin:compliance:enterprise-verification:reject"
	permissionEnterpriseMaterialRead = "idc:admin:compliance:enterprise-material:access"

	enterpriseCaseDirectoryTarget = "enterprise-verification-directory"
	maximumManagementPageSize     = 100
	defaultManagementPageSize     = 20
	maximumCursorBytes            = 2048
	maximumSnapshotPlaintextBytes = 32 * 1024
)

// CaseListInput defines one bounded, stable keyset page.
type CaseListInput struct {
	Page       int
	StatusCode string
	CustomerNo string
	PageSize   int
	Cursor     string
}

// CaseListItem is the non-sensitive management queue projection.
type CaseListItem struct {
	CaseNo               string
	CustomerNo           string
	StatusCode           string
	MaskedLegalName      string
	MaskedRegistrationID string
	ApplicantRoleCode    string
	Reverification       bool
	DecisionReasonCode   string
	SubmittedAt          time.Time
	UpdatedAt            time.Time
	Version              int64
}

// CaseList is one count-free keyset page.
type CaseList struct {
	Total      int
	Items      []CaseListItem
	NextCursor string
}

// MaskedLegalProfile exposes only the minimum default review projection.
type MaskedLegalProfile struct {
	LegalName               string
	RegistrationCountryCode string
	RegistrationIdentifier  string
	LegalRepresentativeName string
	LegalRepresentativeID   string
	AgentPresent            bool
	AgentName               string
	AgentIdentityNumber     string
	IndustryCode            string
	ApplicantRoleCode       string
}

// MaterialItem excludes private paths, ciphertext digests, keys, and URLs.
type MaterialItem struct {
	MaterialNo     string
	MaterialType   MaterialType
	SequenceNo     int
	MediaType      string
	ByteSize       int64
	StatusCode     string
	RetentionUntil *time.Time
	LegalHold      bool
}

// CaseDetail combines one safe case projection with one bounded material batch.
type CaseDetail struct {
	CaseListItem
	LegalProfile MaskedLegalProfile
	Materials    []MaterialItem
	VerifiedAt   *time.Time
	RejectedAt   *time.Time
	ExpiresAt    *time.Time
}

// MaterialAccessContext is the core-signed, session-bound, one-time access proof.
type MaterialAccessContext struct {
	AccessContext string
	ExpiresAt     time.Time
}

// MaterialContent is an in-memory, bounded original cleared by the controller.
type MaterialContent struct {
	MediaType    string
	MaterialType MaterialType
	Content      []byte
}

// ReviewInput starts or restarts one manual review with optimistic concurrency.
type ReviewInput struct {
	CaseNo          string
	ExpectedVersion int64
	IdempotencyKey  string
}

// DecisionInput resolves one verifying case. Approvals ignore ReasonCode and
// use the fixed server-owned success reason; rejections require a closed code.
type DecisionInput struct {
	CaseNo          string
	ExpectedVersion int64
	ReasonCode      string
	IdempotencyKey  string
}

// ReviewResult is the safe post-transition projection.
type ReviewResult struct {
	CaseNo             string
	StatusCode         string
	DecisionReasonCode string
	Version            int64
	Reused             bool
}

// ManagementService is the direct-staff-only enterprise review boundary.
type ManagementService interface {
	// ListCases returns one authorized, count-free keyset page without raw legal values.
	ListCases(context.Context, CaseListInput) (*CaseList, error)
	// GetCase returns one authorized masked detail and one bounded material batch.
	GetCase(context.Context, string) (*CaseDetail, error)
	// IssueMaterialAccess issues one recent-login, session-bound proof for an exact material.
	IssueMaterialAccess(context.Context, string, string) (*MaterialAccessContext, error)
	// OpenMaterial consumes one exact proof and durably audits the private-byte release.
	OpenMaterial(context.Context, string, string, string) (*MaterialContent, error)
	// StartReview moves a pending or review-required case into manual verification.
	StartReview(context.Context, ReviewInput) (*ReviewResult, error)
	// Approve projects only the server-authenticated snapshot into the core identity boundary.
	Approve(context.Context, DecisionInput) (*ReviewResult, error)
	// Reject records one closed actionable reason while preserving any prior verified projection.
	Reject(context.Context, DecisionInput) (*ReviewResult, error)
}

// SensitiveAccessPrivacyGate rotates the binding generation after an anomaly
// and blocks new or existing sensitive-file access while the incident is open.
type SensitiveAccessPrivacyGate interface {
	CurrentSensitiveAccessGeneration(context.Context) (generation int64, blocked bool, err error)
}

// managementServiceImpl owns staff authorization, private evidence reads, review transitions, and audit.
type managementServiceImpl struct {
	database        gdb.DB
	identities      identitycap.Service
	storage         storagecap.Service
	keyAccessor     secretcap.Accessor
	keyManager      secretcap.GeneratedManager
	authorizer      managementcap.Authorizer
	sensitiveAccess managementcap.SensitiveAccessAuthenticator
	audit           idccap.BusinessAuditWriter
	privacyGate     SensitiveAccessPrivacyGate
	now             func() time.Time
}

// Ensure managementServiceImpl implements ManagementService.
var _ ManagementService = (*managementServiceImpl)(nil)

// NewManagement creates the staff review service without exposing raw identity
// values or private object references outside this package.
func NewManagement(
	database gdb.DB,
	identities identitycap.Service,
	storage storagecap.Service,
	keyAccessor secretcap.Accessor,
	keyManager secretcap.Manager,
	runtime managementcap.Runtime,
) (ManagementService, error) {
	return NewManagementWithPrivacyGate(
		database, identities, storage, keyAccessor, keyManager, runtime, openSensitiveAccessPrivacyGate{},
	)
}

// NewManagementWithPrivacyGate binds privacy incident revocation to every
// issued and consumed access proof. Production composition always uses it.
func NewManagementWithPrivacyGate(
	database gdb.DB,
	identities identitycap.Service,
	storage storagecap.Service,
	keyAccessor secretcap.Accessor,
	keyManager secretcap.Manager,
	runtime managementcap.Runtime,
	privacyGate SensitiveAccessPrivacyGate,
) (ManagementService, error) {
	generated, ok := keyManager.(secretcap.GeneratedManager)
	if nilDependency(database) || nilDependency(identities) || nilDependency(storage) ||
		nilDependency(keyAccessor) || !ok || nilDependency(generated) ||
		nilDependency(runtime.Authorizer) || nilDependency(runtime.SensitiveAccess) ||
		nilDependency(runtime.Audit) || nilDependency(privacyGate) {
		return nil, gerror.New("enterprise verification management dependencies are invalid")
	}
	return &managementServiceImpl{
		database: database, identities: identities, storage: storage,
		keyAccessor: keyAccessor, keyManager: generated,
		authorizer: runtime.Authorizer, sensitiveAccess: runtime.SensitiveAccess,
		audit: runtime.Audit, privacyGate: privacyGate, now: time.Now,
	}, nil
}

type openSensitiveAccessPrivacyGate struct{}

func (openSensitiveAccessPrivacyGate) CurrentSensitiveAccessGeneration(context.Context) (int64, bool, error) {
	return 1, false, nil
}
