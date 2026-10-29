// Package purchaseserverlogin stores customer-entered server login choices as
// private owner-scoped credentials. Only opaque tokens enter cart snapshots.
package purchaseserverlogin

import (
	"context"
	"crypto/rand"
	"io"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// HostnameRules contains generator presentation rules, never a required prefix
// or fixed length for a customer-entered hostname.
type HostnameRules struct {
	Prefix     string
	Length     int
	Characters []string
}

// PasswordRules contains generator presentation rules. A typed password may
// have any supported length, independently of the generator's selected length.
type PasswordRules struct {
	Length     int
	Characters []string
}

// Target is trusted published product evidence, resolved without client IDs.
type Target struct {
	ProductVersionNo, ConfigurationRevision, ChannelNo string
	HostnameEnabled, PasswordEnabled                   bool
	HostnameRules                                      HostnameRules
	PasswordRules                                      PasswordRules
}

// TargetResolver rechecks a publicly purchasable product and its exact revision.
type TargetResolver interface {
	// ResolvePurchaseServerLoginTarget resolves a public slug and revision to
	// trusted version, channel and input switches; unavailable/stale targets fail.
	ResolvePurchaseServerLoginTarget(context.Context, string, string) (*Target, error)
}

// Input contains optional customer choices. Password must never be serialized
// into a cart, quote, order, runtime payload, diagnostic, or audit record.
type Input struct {
	PublicSlug, ConfigurationRevision, Hostname, PreviousServerLoginToken string
	Password                                                              string `json:"-"`
}

// Created is the safe browser projection; it never exposes a secret handle.
type Created struct {
	ServerLoginToken, Hostname string
	PasswordConfigured         bool
}

// Resolved is owner-private evidence for cart and fulfillment composition.
// PasswordSecretRef may only be consumed using the resource channel scope.
type Resolved struct{ Hostname, PasswordSecretRef, ChannelNo, CartItemNo string }

// Service owns immutable choices bound to the exact customer login and version.
type Service interface {
	// Create requires an active customer and enabled frozen merchant switches;
	// empty values preserve automatic generation. Invalid input fails safely.
	Create(context.Context, Input) (*Created, error)
	// Resolve checks token, exact current login, customer and product version in
	// one indexed query. Repeated reads support cart retries; empty token returns
	// nil and foreign or malformed tokens return the same inaccessible error.
	Resolve(context.Context, string, string, string) (*Resolved, error)
	// Bind atomically reserves a token for one cart item. The
	// same cart item may retry; any different cart item or login is rejected. Caller
	// must use the checkout transaction context so failures roll back the binding.
	Bind(context.Context, string, string, string, string) error
}

type serviceImpl struct {
	database  gdb.DB
	principal principal.Reader
	secrets   secretcap.Source
	resolver  TargetResolver
	random    io.Reader
}

var _ Service = (*serviceImpl)(nil)

// New binds the database, trusted customer identity, shared secret source and
// published target resolver. No request constructs an independent service graph.
func New(database gdb.DB, identity principal.Reader, secrets secretcap.Source, resolver TargetResolver) (Service, error) {
	if database == nil || identity == nil || secrets == nil || resolver == nil {
		return nil, gerror.New("purchase server login requires database, identity, secret source and target resolver")
	}
	return &serviceImpl{database: database, principal: identity, secrets: secrets, resolver: resolver, random: rand.Reader}, nil
}
