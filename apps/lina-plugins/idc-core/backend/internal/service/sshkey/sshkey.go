// Package sshkey owns customer SSH public-key validation, persistence, and revocation.
package sshkey

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/audit"
)

// Status is the stable customer-visible key lifecycle state.
type Status string

const (
	// StatusActive allows a key to be selected for a compatible operation.
	StatusActive Status = "active"
	// StatusRevoked preserves history but prevents all future selection.
	StatusRevoked Status = "revoked"
)

// PublicKey is the safe customer projection. It deliberately excludes key material.
type PublicKey struct {
	KeyNo       string
	Name        string
	Algorithm   string
	Fingerprint string
	Status      Status
	Version     int64
	CreatedAt   time.Time
	RevokedAt   *time.Time
}

// CreateInput contains one named authorized-key line and a retry-safe request key.
type CreateInput struct {
	Name           string
	AuthorizedKey  string
	IdempotencyKey string
}

// RevokeInput revokes one currently active owned key using optimistic locking.
type RevokeInput struct {
	KeyNo           string
	ExpectedVersion int64
	IdempotencyKey  string
}

// InjectionKey identifies the active key consumed inside one synchronous injection boundary.
// PublicKey is cleared as soon as the callback returns and must never be persisted or logged.
type InjectionKey struct {
	KeyNo       string
	PublicKey   []byte
	Fingerprint string
}

// InjectionConsumer sends one temporary public-key value to a Provider only after the
// frozen product and Provider capability snapshot has explicitly allowed SSH injection.
type InjectionConsumer func(InjectionKey) error

// Service manages the current customer's public keys and consumes an active key for one operation.
type Service interface {
	// List returns at most one hundred owned keys without exposing public-key material.
	List(context.Context) ([]PublicKey, error)
	// Create strictly parses one public key, rejects private keys, and returns only its safe projection.
	Create(context.Context, CreateInput) (*PublicKey, error)
	// Revoke permanently removes one owned key from future operation choices while retaining history.
	Revoke(context.Context, RevokeInput) (*PublicKey, error)
	// ConsumeActive opens a synchronous, one-call boundary for an active owned public key.
	// Callers must finish product/Provider capability checks before calling it. The service
	// clears its byte buffer immediately after consume returns and never audits the material.
	ConsumeActive(context.Context, string, InjectionConsumer) error
}

type serviceImpl struct {
	database  gdb.DB
	customers identitycap.CurrentCustomerReader
	audit     audit.Writer
}

var _ Service = (*serviceImpl)(nil)

// New creates the customer-owned SSH public-key service from shared runtime dependencies.
func New(database gdb.DB, customers identitycap.CurrentCustomerReader, auditWriter audit.Writer) (Service, error) {
	if database == nil || customers == nil || auditWriter == nil {
		return nil, gerror.New("SSH public key service requires all runtime dependencies")
	}
	return &serviceImpl{database: database, customers: customers, audit: auditWriter}, nil
}
