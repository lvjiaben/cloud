// Package enterpriseverification owns customer enterprise-verification draft,
// encrypted material upload, and submission state transitions.
package enterpriseverification

import (
	"context"
	"crypto/rand"
	"io"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// enterpriseSubjectType is the fixed compliance case subject for this service.
	enterpriseSubjectType = "enterprise"
	// manualProviderCode records that the decision is made by merchant staff.
	manualProviderCode = "manual_enterprise"
	// maximumMaterialBytes bounds one decrypted original in application memory.
	maximumMaterialBytes = 5 * 1024 * 1024
	// maximumSelectedCount bounds reservations, selected evidence, and storage reads.
	maximumSelectedCount = 8
)

// MaterialType is a bounded enterprise evidence category.
type MaterialType string

const (
	// MaterialBusinessLicense identifies the mandatory business-license evidence.
	MaterialBusinessLicense MaterialType = "business_license"
	// MaterialLegalRepresentativeID identifies legal-representative identity evidence.
	MaterialLegalRepresentativeID MaterialType = "legal_representative_id"
	// MaterialAgentID identifies authorized-agent identity evidence.
	MaterialAgentID MaterialType = "agent_id"
	// MaterialAuthorizationLetter identifies the authorized-agent mandate.
	MaterialAuthorizationLetter MaterialType = "authorization_letter"
	// MaterialOrganizationCert identifies optional organization evidence.
	MaterialOrganizationCert MaterialType = "organization_certificate"
	// MaterialOther identifies one additional bounded evidence category.
	MaterialOther MaterialType = "other"
)

// ApplicantRole identifies who submits enterprise verification evidence.
type ApplicantRole string

const (
	// ApplicantLegalRepresentative means the legal representative submits directly.
	ApplicantLegalRepresentative ApplicantRole = "legal_representative"
	// ApplicantAuthorizedAgent means an authorized agent submits with a mandate.
	ApplicantAuthorizedAgent ApplicantRole = "authorized_agent"
)

// CreateDraftInput opens one customer-owned enterprise review case.
type CreateDraftInput struct {
	ConsentNo            string
	LegalChangeRequestNo string
	IdempotencyKey       string
}

// Draft is the customer-safe enterprise case projection.
type Draft struct {
	CaseNo               string
	StatusCode           string
	LegalChangeRequestNo string
	Reverification       bool
	Reused               bool
}

// ReserveMaterialInput creates a non-sensitive, idempotent upload reservation.
type ReserveMaterialInput struct {
	CaseNo         string
	MaterialType   MaterialType
	IdempotencyKey string
}

// MaterialReservation is safe to return before any original bytes are sent.
type MaterialReservation struct {
	MaterialNo   string
	MaterialType MaterialType
	StatusCode   string
	Reused       bool
}

// UploadMaterialInput carries one bounded original for a server-assigned
// material number. Filename and caller idempotency are deliberately absent.
type UploadMaterialInput struct {
	CaseNo            string
	MaterialNo        string
	DeclaredMediaType string
	Body              io.Reader
}

// Material is the customer-safe material projection. It contains no storage
// path, digest, filename, encryption key reference, or permanent access URL.
type Material struct {
	MaterialNo   string
	MaterialType MaterialType
	SequenceNo   int
	MediaType    string
	ByteSize     int64
	StatusCode   string
	Reused       bool
}

// SubmitInput contains legal values only for one encrypted snapshot and a
// pending core projection. Every caller-owned legal and applicant identity
// buffer is cleared before Submit returns.
type SubmitInput struct {
	CaseNo                            string
	LegalName                         []byte
	RegistrationCountryCode           string
	RegistrationIdentifier            []byte
	LegalRepresentativeName           []byte
	LegalRepresentativeIdentityNumber []byte
	AgentName                         []byte
	AgentIdentityNumber               []byte
	IndustryCode                      string
	ApplicantRole                     ApplicantRole
	MaterialNos                       []string
	IdempotencyKey                    string
}

// Submission is the bounded pending-review result.
type Submission struct {
	CaseNo         string
	StatusCode     string
	Reverification bool
	Reused         bool
}

// CustomerCase is the current customer's latest safe enterprise case state.
// It excludes legal plaintext, storage references, digests, filenames, and keys.
type CustomerCase struct {
	CaseNo               string
	StatusCode           string
	DecisionReasonCode   string
	LegalChangeRequestNo string
	Reverification       bool
	SubmittedAt          *time.Time
	VerifiedAt           *time.Time
	RejectedAt           *time.Time
	ExpiresAt            *time.Time
	UpdatedAt            *time.Time
	Version              int64
	Materials            []Material
}

// Service is the customer-only enterprise submission boundary.
type Service interface {
	// Current returns the latest customer-owned case state without accepting a
	// customer number or exposing encrypted-storage metadata.
	Current(context.Context) (*CustomerCase, error)
	// CreateDraft creates or replays a customer-owned draft after rechecking the
	// current enterprise, consent, and optional legal-change binding.
	CreateDraft(context.Context, CreateDraftInput) (*Draft, error)
	// ReserveMaterial creates a bounded opaque upload number. The caller's raw
	// idempotency key is used only in this JSON command and is never persisted.
	ReserveMaterial(context.Context, ReserveMaterialInput) (*MaterialReservation, error)
	// UploadMaterial accepts original bytes only for a previously reserved,
	// customer-owned material number and confirms encrypted private storage.
	UploadMaterial(context.Context, UploadMaterialInput) (*Material, error)
	// Submit validates the complete evidence set and atomically creates the
	// pending enterprise projection, case transition, and audit evidence.
	Submit(context.Context, SubmitInput) (*Submission, error)
}

type serviceImpl struct {
	database    gdb.DB
	identities  identitycap.Service
	storage     storagecap.Service
	keyAccessor secretcap.Accessor
	keyManager  secretcap.GeneratedManager
	audit       idccap.BusinessAuditWriter
	privacyGate privacyCollectionGate
	now         func() time.Time
	random      io.Reader
}

type privacyCollectionGate interface {
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
}

var _ Service = (*serviceImpl)(nil)

// New creates the enterprise-verification submission service.
func New(
	database gdb.DB,
	identities identitycap.Service,
	storage storagecap.Service,
	keyAccessor secretcap.Accessor,
	keyManager secretcap.Manager,
	auditWriter idccap.BusinessAuditWriter,
) (Service, error) {
	return NewWithCollectionGate(database, identities, storage, keyAccessor, keyManager, auditWriter, openPrivacyCollectionGate{})
}

// NewWithCollectionGate applies the merchant's published privacy policy before originals are accepted.
func NewWithCollectionGate(
	database gdb.DB,
	identities identitycap.Service,
	storage storagecap.Service,
	keyAccessor secretcap.Accessor,
	keyManager secretcap.Manager,
	auditWriter idccap.BusinessAuditWriter,
	privacyGate privacyCollectionGate,
) (Service, error) {
	generated, ok := keyManager.(secretcap.GeneratedManager)
	if nilDependency(database) || nilDependency(identities) || nilDependency(storage) ||
		nilDependency(keyAccessor) || !ok || nilDependency(generated) || nilDependency(auditWriter) || nilDependency(privacyGate) {
		return nil, gerror.New("enterprise verification dependencies are invalid")
	}
	return &serviceImpl{
		database: database, identities: identities, storage: storage,
		keyAccessor: keyAccessor, keyManager: generated, audit: auditWriter, privacyGate: privacyGate,
		now: time.Now, random: rand.Reader,
	}, nil
}

type openPrivacyCollectionGate struct{}

func (openPrivacyCollectionGate) RequireSensitiveCollection(context.Context, string, string) (time.Time, error) {
	return time.Now().UTC().Add(24 * time.Hour), nil
}

func nilDependency(value any) bool {
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
