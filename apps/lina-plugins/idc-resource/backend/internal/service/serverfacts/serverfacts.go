// Package serverfacts stores bounded, whitelisted historical resource facts in
// PostgreSQL. It never authorizes operations or performs supplier writes.
package serverfacts

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// ErrInvalid identifies an invalid or oversized entitlement query.
var ErrInvalid = gerror.New("invalid native server facts query")

// ProviderRuntime is the already assembled resource-owner provider runtime.
type ProviderRuntime interface {
	// ConstructSession opens a governed read session; the cleanup must be called.
	ConstructSession(context.Context, string) (resourcecap.Provider, func(), error)
}

// Fact contains only explicitly approved display fields, never arbitrary data.
type Fact struct {
	EntitlementNo      string
	ServerNo           string
	Hostname           string
	OSName             string
	Addresses          []string
	AddressesConfirmed bool
	LastObservedState  resourcecap.ResourceState
	ObservedAt         *time.Time
	Stale              bool
}

// Service reads customer-scoped local facts and refreshes fixed-size work units.
type Service interface {
	// Read filters customer ownership in SQL. Empty input returns empty; unknown,
	// foreign and inactive bindings are omitted; no supplier call occurs here.
	Read(context.Context, string, []string) ([]Fact, error)
	// RunOne leases at most one live native mapping and performs one read. Failed
	// reads retain previous facts and retry later; stale leases cannot publish.
	RunOne(context.Context) (bool, error)
}

type serviceImpl struct {
	database  gdb.DB
	providers ProviderRuntime
}

var _ Service = (*serviceImpl)(nil)

// New receives the shared database and startup-assembled provider runtime.
func New(database gdb.DB, providers ProviderRuntime) (Service, error) {
	if database == nil || providers == nil {
		return nil, gerror.New("native server facts dependencies unavailable")
	}
	return &serviceImpl{database: database, providers: providers}, nil
}
