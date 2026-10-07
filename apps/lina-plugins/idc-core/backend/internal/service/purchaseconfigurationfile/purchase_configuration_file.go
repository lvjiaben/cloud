// Package purchaseconfigurationfile owns authenticated, private file uploads
// used by frozen customer purchase-configuration fields.
package purchaseconfigurationfile

import (
	"context"
	"crypto/rand"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	StatusReserved  = "reserved"
	StatusUploading = "uploading"
	StatusReady     = "ready"
	StatusConsumed  = "consumed"
	StatusRevoked   = "revoked"
	StatusRejected  = "rejected"
	StatusExpired   = "expired"
)

// Target is the exact frozen field and upload policy resolved from opaque
// public product and configuration tokens.
type Target struct {
	ProductVersionNo      string
	ConfigurationRevision string
	FieldKey              string
	Policy                *capmodel.ConfigurationUploadPolicyDeclaration
}

// TargetResolver resolves a customer-visible field without accepting plugin
// keys, product version numbers, or policy details from the browser.
type TargetResolver interface {
	ResolvePurchaseFileTarget(context.Context, string, string, string) (*Target, error)
}

// Scanner performs the host-governed safety decision. Implementations must
// return only a stable verdict and must not expose scanner output to callers.
type Scanner interface {
	Ready() bool
	Scan(context.Context, io.Reader, int64) (string, error)
}

// ReserveInput identifies one file selected for an exact published field.
type ReserveInput struct {
	PublicSlug, ConfigurationRevision, FieldToken string
	FileName                                      string
	SizeBytes                                     int64
}

// Reservation is the safe client projection of one private upload slot.
type Reservation struct {
	UploadToken string
	ExpiresAt   time.Time
}

// Item is safe upload metadata. It never contains an object path, storage
// credential, scanner response, customer number, or internal field key.
type Item struct {
	FileToken, FileName, MediaType, Status string
	SizeBytes                              int64
	ExpiresAt                              time.Time
}

// VerifyInput binds a bounded token set to one exact frozen field.
type VerifyInput struct {
	Target     *Target
	FileTokens []string
}

// OrderEvidenceOwner is trusted immutable order evidence, never browser input.
// It lets a background worker verify the original uploader without inventing
// an interactive customer session.
type OrderEvidenceOwner struct {
	OrderNo        string
	CustomerNo     string
	UploaderUserID int
}

type OrderEvidenceBatchInput struct {
	Owner  OrderEvidenceOwner
	Fields []VerifyInput
}

// VerifiedField preserves the caller's field order while returning only
// internal immutable evidence for the exact verified token set.
type VerifiedField struct {
	Target *Target
	Files  []VerifiedFile
}

// VerifiedFile is the immutable internal evidence returned after one batched
// ownership and policy verification.
type VerifiedFile struct {
	FileToken, FileName, MediaType, SHA256Hex, ObjectRef string
	SizeBytes                                            int64
}

// Service owns reservations, private uploads, safe status reads, and batched
// file-token verification for authenticated customers.
type Service interface {
	// Reserve authenticates the exact customer account, resolves the frozen
	// field and policy, and creates a short-lived private upload reservation.
	Reserve(context.Context, ReserveInput) (*Reservation, error)
	// Upload rechecks exact uploader ownership, inspects content and optional
	// safety scan, stores a private object, and makes its opaque token usable.
	Upload(context.Context, string, string, io.Reader) (*Item, error)
	// Get returns safe metadata only when the current exact uploader owns it.
	Get(context.Context, string) (*Item, error)
	// Revoke immediately prevents a reserved or ready token owned by the exact
	// current uploader from being used by a new order. Consumed order evidence
	// is immutable and cannot be revoked through the upload account surface.
	Revoke(context.Context, string) (*Item, error)
	// VerifyBatch performs one bounded authoritative query and rejects any
	// foreign, stale, uninspected, cross-field, or policy-mismatched token.
	VerifyBatch(context.Context, VerifyInput) ([]VerifiedFile, error)
	// VerifyFields verifies every file field with one bounded ownership query,
	// avoiding one database round trip per declared field.
	VerifyFields(context.Context, []VerifyInput) ([]VerifiedField, error)
	// ConsumeForOrder atomically and idempotently binds ready tokens to one
	// exact order. A token already bound to another order is never reusable.
	ConsumeForOrder(context.Context, string, []VerifyInput) ([]VerifiedField, error)
	// ConsumeForOrderTransaction performs the same exact-account transition on
	// the caller's checkout transaction so order creation and token binding can
	// never commit independently.
	ConsumeForOrderTransaction(context.Context, gdb.TX, string, []VerifyInput) ([]VerifiedField, error)
	// VerifyOrderEvidence accepts only permanent evidence already consumed by
	// the exact order. Ready, foreign-order, revoked, and unbound tokens fail.
	VerifyOrderEvidence(context.Context, OrderEvidenceOwner, []VerifyInput) ([]VerifiedField, error)
	// VerifyOrderEvidenceBatch proves multiple immutable snapshots with one
	// bounded database query; result order matches input order.
	VerifyOrderEvidenceBatch(context.Context, []OrderEvidenceBatchInput) ([][]VerifiedField, error)
}

type OrderEvidenceVerifier interface {
	VerifyOrderEvidence(context.Context, OrderEvidenceOwner, []VerifyInput) ([]VerifiedField, error)
	VerifyOrderEvidenceBatch(context.Context, []OrderEvidenceBatchInput) ([][]VerifiedField, error)
}

// OrderFileConsumer is the narrow checkout dependency for atomically consuming
// frozen file evidence inside the order transaction.
type OrderFileConsumer interface {
	ConsumeForOrderTransaction(context.Context, gdb.TX, string, []VerifyInput) ([]VerifiedField, error)
}

func NewOrderFileConsumer(database gdb.DB, principalReader principal.Reader) (OrderFileConsumer, error) {
	if database == nil || principalReader == nil {
		return nil, gerror.New("purchase configuration order consumption requires a database and identity")
	}
	return &serviceImpl{database: database, principal: principalReader}, nil
}

func NewOrderEvidenceVerifier(database gdb.DB) (OrderEvidenceVerifier, error) {
	if database == nil {
		return nil, gerror.New("purchase configuration order evidence requires a database")
	}
	return &serviceImpl{database: database}, nil
}

type serviceImpl struct {
	database  gdb.DB
	principal principal.Reader
	storage   storagecap.Service
	resolver  TargetResolver
	scanner   Scanner
	random    io.Reader
}

// New creates the governed purchase-configuration file service.
func New(database gdb.DB, principalReader principal.Reader, storage storagecap.Service, resolver TargetResolver, scanner Scanner) (Service, error) {
	if database == nil || principalReader == nil || storage == nil || resolver == nil || scanner == nil {
		return nil, gerror.New("purchase configuration files require database, identity, private storage, target resolver, and scanner")
	}
	return &serviceImpl{database: database, principal: principalReader, storage: storage, resolver: resolver, scanner: scanner, random: rand.Reader}, nil
}
