// Package currency owns merchant currency admission and administrator configuration.
// Currency records are retained when disabled; no historical money is converted.
package currency

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// Item is a denomination's current admission status, never a financial balance.
type Item struct {
	CurrencyCode       string
	Enabled, IsDefault bool
	Version            int64
}

// Page is a bounded management result with an exact filtered total.
type Page struct {
	Items []Item
	Total int
}

// Reader exposes public, non-sensitive currency admission from the authoritative database.
type Reader interface {
	// Enabled returns up to 300 enabled ISO denominations, default first, without client data.
	Enabled(context.Context) ([]string, error)
	// IsRemoved distinguishes removed configuration from temporarily disabled currencies for renewals.
	IsRemoved(context.Context, string) (bool, error)
	// IsEnabled checks one exact denomination for a new quote; failures reject admission.
	IsEnabled(context.Context, string) (bool, error)
}

// Service configures merchant-wide denominations under direct-staff full-scope authorization.
type Service interface {
	Reader
	// List returns one numbered page after currency-read authorization.
	List(context.Context, int, int) (Page, error)
	// Create adds a valid ISO denomination with audit; duplicates are rejected.
	Create(context.Context, string) (Item, error)
	// Delete removes non-default configuration with version and audit protection; historical money is retained.
	Delete(context.Context, string, int64) error
	// Update uses the row version and serial merchant lock; a default cannot be disabled.
	Update(context.Context, string, int64, bool, bool) (Item, error)
}

type reader struct{ database gdb.DB }
type serviceImpl struct {
	reader
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	audit         idccap.BusinessAuditWriter
}

// NewReader constructs the stateless public admission projection at startup.
func NewReader(database gdb.DB) (Reader, error) {
	if database == nil {
		return nil, gerror.New("currency reader requires database")
	}
	return &reader{database: database}, nil
}

// New constructs the staff management service with shared trusted identity and audit dependencies.
func New(database gdb.DB, bizCtx bizctxcap.Service, identity principal.Reader, authorization authz.Service, audit idccap.BusinessAuditWriter) (Service, error) {
	if database == nil || bizCtx == nil || identity == nil || authorization == nil || audit == nil {
		return nil, gerror.New("currency management requires database, identity, authorization and audit")
	}
	return &serviceImpl{reader: reader{database: database}, bizCtx: bizCtx, principal: identity, authorization: authorization, audit: audit}, nil
}
