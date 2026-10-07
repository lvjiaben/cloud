// Package personalverification owns the personal Aliyun H5 FACE case state machine.
package personalverification

import (
	"context"
	"crypto/rand"
	"io"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	personalSubjectType  = "personal"
	providerCode         = aliyunface.ProviderCode
	preparingTimeout     = 2 * time.Minute
	maximumRecoveryBatch = 50
)

// Status is the customer-safe verification case status.
type Status string

const (
	StatusUnverified     Status = "unverified"
	StatusVerifying      Status = "verifying"
	StatusReviewRequired Status = "review_required"
	StatusVerified       Status = "verified"
	StatusRejected       Status = "rejected"
	StatusExpired        Status = "expired"
)

// StartInput contains identity bytes used only for one encrypted snapshot and
// provider initiation call. The service clears both buffers before returning.
// ReturnURL must be assembled by a trusted server-side controller.
type StartInput struct {
	ConsentNo string
	CertType  string
	CertName  []byte
	CertNo    []byte
	ReturnURL string
}

// Verification is a bounded customer projection. ContinuationURL is returned
// only by a fresh successful Start and is never persisted.
type Verification struct {
	CaseNo           string
	AttemptNo        string
	Status           Status
	ContinuationURL  string
	MaskedName       string
	MaskedIdentityNo string
	SubmittedAt      *time.Time
	VerifiedAt       *time.Time
	ExpiresAt        *time.Time
	ReasonCode       string
}

// RecoveryResult reports bounded background work without exposing customer identifiers.
type RecoveryResult struct {
	Selected       int
	Queried        int
	Verified       int
	Expired        int
	ReviewRequired int
	Pending        int
}

// ClientFactory binds one immutable provider configuration generation to a client.
type ClientFactory interface {
	// Create returns a client pinned to the supplied immutable configuration generation.
	Create(providerconfig.RuntimeConfiguration) (aliyunface.Client, error)
}

type privacyCollectionGate interface {
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
}

// Service owns customer initiation/query and system recovery.
type Service interface {
	// Current returns only locally persisted state for the authenticated personal
	// customer and never calls the provider.
	Current(context.Context) (*Verification, error)
	// Start durably prepares one case and issues exactly one provider initialization call.
	Start(context.Context, StartInput) (*Verification, error)
	// QueryCurrent rechecks an attempt owned by the authenticated personal customer.
	QueryCurrent(context.Context, string) (*Verification, error)
	// Recover processes at most the requested bounded number of persisted attempts.
	Recover(context.Context, int) (*RecoveryResult, error)
}

type serviceImpl struct {
	database         gdb.DB
	identities       identitycap.Service
	configurations   providerconfig.RuntimeSource
	clients          ClientFactory
	snapshotAccessor secretcap.Accessor
	snapshotManager  secretcap.Manager
	audit            idccap.BusinessAuditWriter
	privacyGate      privacyCollectionGate
	now              func() time.Time
	random           io.Reader
}

var _ Service = (*serviceImpl)(nil)

// New creates the personal-verification state machine from explicit source dependencies.
func New(
	database gdb.DB,
	identities identitycap.Service,
	configurations providerconfig.RuntimeSource,
	clients ClientFactory,
	snapshotAccessor secretcap.Accessor,
	snapshotManager secretcap.Manager,
	auditWriter idccap.BusinessAuditWriter,
) (Service, error) {
	return NewWithCollectionGate(database, identities, configurations, clients, snapshotAccessor, snapshotManager, auditWriter, openPrivacyCollectionGate{})
}

// NewWithCollectionGate blocks collection unless the merchant published both
// the assessment and the matching retention policy.
func NewWithCollectionGate(
	database gdb.DB,
	identities identitycap.Service,
	configurations providerconfig.RuntimeSource,
	clients ClientFactory,
	snapshotAccessor secretcap.Accessor,
	snapshotManager secretcap.Manager,
	auditWriter idccap.BusinessAuditWriter,
	privacyGate privacyCollectionGate,
) (Service, error) {
	if nilDependency(database) || nilDependency(identities) || nilDependency(configurations) ||
		nilDependency(clients) || nilDependency(snapshotAccessor) || nilDependency(snapshotManager) ||
		nilDependency(auditWriter) || nilDependency(privacyGate) {
		return nil, gerror.New("personal verification dependencies are invalid")
	}
	return &serviceImpl{
		database: database, identities: identities, configurations: configurations,
		clients: clients, snapshotAccessor: snapshotAccessor, snapshotManager: snapshotManager,
		audit: auditWriter, privacyGate: privacyGate,
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
