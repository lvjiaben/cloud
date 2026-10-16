// Package entitlementupgrade owns current-period paid configuration upgrades.
// Dependencies are injected at startup; service changes require proven supplier success.
package entitlementupgrade

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"time"
)

// PeriodPrice is trusted local accounting input, never an HTTP amount. Actual
// is the entire current paid period's tax-exclusive product/configuration fee,
// after discounts and without installation. Time precision is retained.
type PeriodPrice struct {
	ActualMinor, TargetMinor  int64
	StartedAt, ExpiresAt, Now time.Time
}

// Service serves customer actions and the bounded durable worker from one database backend.
type Service interface {
	// List returns at most 100 active owned services and recent owned intents; no foreign data is exposed.
	List(context.Context) (*Page, error)
	// Quote freezes one owned service and target quote using a stable retry key; it never debits money.
	Quote(context.Context, string, string, string) (*View, error)
	// Confirm rechecks the owned quote and service revision before creating one invoice; zero totals queue directly.
	Confirm(context.Context, string) (*View, error)
	// Pay settles the owned same-currency invoice once; previously paid invoices only reconcile state.
	Pay(context.Context, string) (*View, error)
	// Run is a system-only worker bounded to 1..100 intents; unknown results require manual review.
	Run(context.Context, int) (int, error)
}

// Target exposes safe sales identity for an owned, active local recurring service.
type Target struct{ EntitlementNo, PublicSlug, NameZhCN, NameEnUS, CurrencyCode string }

// View is a secret-free monetary and execution projection. Times are Unix milliseconds.
type View struct {
	UpgradeNo, EntitlementNo, CurrencyCode, StatusCode, InvoiceNo        string
	SourcePeriodMinor, TargetPeriodMinor, NetMinor, TaxMinor, TotalMinor int64
	ExpiresAt                                                            int64
}

// Page contains bounded customer targets and recent upgrade intents.
type Page struct {
	Targets []Target
	Items   []View
}

type serviceImpl struct {
	database  gdb.DB
	principal principal.Reader
	invoices  billing.InvoiceService
	source    serveroperationresourcecap.Source
	compiler  serveroperationresourcecap.UpgradeConfigurationCompiler
}

var _ Service = (*serviceImpl)(nil)

// New injects the startup-owned identity, billing and resource capabilities; it never creates a service graph.
func New(database gdb.DB, reader principal.Reader, invoices billing.InvoiceService, source serveroperationresourcecap.Source) (Service, error) {
	compiler, ok := source.(serveroperationresourcecap.UpgradeConfigurationCompiler)
	if database == nil || reader == nil || invoices == nil || !ok || compiler == nil {
		return nil, gerror.New("upgrade dependencies are unavailable")
	}
	return &serviceImpl{database, reader, invoices, source, compiler}, nil
}

// Worker owns bounded paid-intent reconciliation without requiring a manufactured customer principal.
type Worker interface {
	// Run processes 1..100 paid intents under SQL locks; ambiguous outcomes stop automatic execution.
	Run(context.Context, int) (int, error)
}

// NewWorker reuses the resource source and SQL backend built by the job composition root.
func NewWorker(database gdb.DB, invoices billing.InvoiceService, source serveroperationresourcecap.Source) (Worker, error) {
	compiler, ok := source.(serveroperationresourcecap.UpgradeConfigurationCompiler)
	if database == nil || invoices == nil || !ok || compiler == nil {
		return nil, gerror.New("upgrade worker dependencies unavailable")
	}
	return &serviceImpl{database: database, invoices: invoices, source: source, compiler: compiler}, nil
}
