// This file prepares identity-neutral entitlement IP and resize prices.
// Callers authenticate the real entitlement and freeze the returned facts in their
// own commercial intent; this component never creates a channel identity, order,
// invoice, settlement, legacy server instance, or provider write task.
package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// NativePriceQuoteTTL bounds the resource reservation and frozen commercial quote equally.
const NativePriceQuoteTTL = 15 * time.Minute

// NativePriceContext contains core-authoritative entitlement facts and an owner-resolved
// route. No field is accepted directly as a browser assertion of ownership.
type NativePriceContext struct {
	EntitlementNo, CustomerNo, CurrencyCode, ProductNo, VersionNo, BillingRuleNo string
	PriceConfigurationNo, StructuredConfigurationNo, PublicSlug, PeriodUnitCode  string
	LocalResourceNo, ChannelNo, PoolNo                                           string
	PeriodCount                                                                  int
	EntitlementVersion, CustomerVersion                                          int64
	StartedAt                                                                    time.Time
	ExpiresAt                                                                    *time.Time
}

// NativePriceTaxContext freezes the merchant policy used to split the inclusive charge.
type NativePriceTaxContext struct {
	PolicyNo, Treatment, Category, Scope, Mode string
	Rate                                       *int
	Quantum                                    int64
	PriceIncludesTax                           bool
}

// NativePriceResizeInput contains public catalog selections, never supplier parameters or prices.
type NativePriceResizeInput struct {
	Revision, TermNo, CurrencyCode string
	Selections                     []catalog.PublicConfigurationSelection
}

// PreparedNativePrice is a value snapshot for either genuine channel or native
// commercial persistence. A paid quote is never execution or settlement proof.
type PreparedNativePrice struct {
	Context                                                           NativePriceContext
	Action                                                            serveroperationresourcecap.Action
	Direction                                                         string
	Parameters                                                        json.RawMessage
	CurrentRecurringMinor, TargetRecurringMinor, CatalogChargeMinor   int64
	CurrentPeriodicNetMinor                                           int64
	SubtotalMinor, NetAmountMinor, TaxAmountMinor, TotalAmountMinor   int64
	TargetSubtotalMinor, TargetNetMinor, TargetTaxMinor               int64
	TargetVersionNo, TargetBillingRuleNo                              string
	TargetQuoteNo, TargetQuoteItemNo, TargetStructuredConfigurationNo string
	PurchaseCycles                                                    int
	ResolvedConfiguration                                             *catalog.ServerResizeConfiguration
	Tax                                                               NativePriceTaxContext
	PolicyNo                                                          string
	PolicyVersion                                                     int64
	Reservation                                                       *serveroperationresourcecap.IPReservation
	ReservationRequest                                                serveroperationresourcecap.IPReservationRequest
	ResourceSnapshotNo, OldIPSetDigest                                string
	IPQuantity, MaximumSnapshotAgeSeconds                             int
	EffectiveAt, IssuedAt, ExpiresAt                                  time.Time
}

// NativePriceBuilder shares the pricing algorithm while the calling admission layer keeps
// ownership, idempotency, transaction fencing, payment and audit responsibilities.
type NativePriceBuilder struct {
	database gdb.DB
	catalog  catalog.CatalogReader
	source   serveroperationresourcecap.Source
}

// NewNativePriceBuilder requires the existing startup service instances; it creates no independent
// service graph or cached authorization/entitlement state.
func NewNativePriceBuilder(database gdb.DB, reader catalog.CatalogReader, source serveroperationresourcecap.Source) (*NativePriceBuilder, error) {
	if nilServerOperationDependency(database) || nilServerOperationDependency(reader) || nilServerOperationDependency(source) {
		return nil, gerror.New("entitlement priced lifecycle dependencies are required")
	}
	return &NativePriceBuilder{database: database, catalog: reader, source: source}, nil
}

func validNativePriceContext(ctx context.Context, value NativePriceContext, now time.Time) bool {
	return ctx != nil && ctx.Err() == nil && value.EntitlementNo != "" && value.CustomerNo != "" && value.ProductNo != "" && value.VersionNo != "" && value.BillingRuleNo != "" && value.CurrencyCode != "" && value.LocalResourceNo != "" && value.ChannelNo != "" && value.PoolNo != "" && value.EntitlementVersion > 0 && value.CustomerVersion > 0 && !now.IsZero()
}

// LoadTax reads exactly one version/currency core policy; missing or ambiguous policies
// fail closed without inferring tax from supplier values.
func (s *NativePriceBuilder) LoadTax(ctx context.Context, versionNo, currency string) (*NativePriceTaxContext, error) {
	var rows []NativePriceTaxContext
	if err := s.database.GetScan(ctx, &rows, `SELECT tax_policy_no AS policy_no,
 tax_treatment_code AS treatment,tax_rate_basis_points AS rate,tax_category_code AS category,
 rounding_scope_code AS scope,rounding_mode_code AS mode,rounding_quantum_minor AS quantum,
 price_includes_tax FROM plugin_idc_core_catalog_tax_policy
 WHERE tenant_id=0 AND version_no=? AND currency_code=?
 ORDER BY id LIMIT 2`, versionNo, currency); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return &rows[0], nil
}
