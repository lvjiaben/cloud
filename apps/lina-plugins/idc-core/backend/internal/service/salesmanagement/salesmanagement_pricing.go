// This file defines the bounded owner-to-owner pricing contract. Customer
// identity is established by checkout; callers cannot supply a price or tier.
package salesmanagement

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"time"
)

// PricingService is consumed by trusted commerce owners, never directly by HTTP.
type PricingService interface {
	// EvaluatePrices resolves current customer and tier rules in bounded queries.
	// Setup charges are excluded. Results include matching zero-discount rules.
	EvaluatePrices(context.Context, PriceEvaluationInput) ([]PriceEvaluation, error)
}

// PriceLine identifies one authoritative, tax-exclusive commerce line.
type PriceLine struct {
	ReferenceNo, ProductNo, ChargeTypeCode, PeriodUnitCode string
	PeriodCount, Quantity, PurchaseCycles                  int
	PeriodSubtotalMinor                                    int64
}

// PriceEvaluationInput supplies customer ownership and catalog facts from commerce.
type PriceEvaluationInput struct {
	CustomerNo, CurrencyCode string
	Now                      time.Time
	Lines                    []PriceLine
}

// PriceEvaluation freezes rule provenance and the authoritative tax-exclusive periodic price.
type PriceEvaluation struct {
	ReferenceNo, PriceNo, ContentDigest    string
	Version                                int64
	OriginalAmountMinor, TargetAmountMinor int64
}

// PriceRule is a display-safe version of a per-period customer or tier price.
type PriceRule struct {
	PriceNo, CustomerNo, CustomerName, CustomerCode, LevelNo, LevelName, ProductNo, ProductName, CurrencyCode, ChargeTypeCode, PeriodUnitCode, AdjustmentType, Status string
	PeriodCount                                                                                                                                                       int
	FixedPriceMinor                                                                                                                                                   string
	DiscountBasisPoints                                                                                                                                               *int
	Version                                                                                                                                                           int64
}

// PricePage has bounded records and an accurate filtered total.
type PricePage struct {
	Items []PriceRule
	Total int
}

// PriceInput appends a new version of a logical rule.
type PriceInput struct {
	PriceNo, CustomerNo, LevelNo, ProductNo, CurrencyCode, ChargeTypeCode, PeriodUnitCode, AdjustmentType, Status, FixedPriceMinor string
	PeriodCount                                                                                                                    int
	DiscountBasisPoints                                                                                                            *int
	ExpectedVersion                                                                                                                int64
}

// NewPricing constructs a stateless read-only price owner for controlled runtime
// assembly. It exposes no administrative writes, identity overrides or caches.
func NewPricing(database gdb.DB) (PricingService, error) {
	if database == nil {
		return nil, gerror.New("sales pricing requires a database")
	}
	return &serviceImpl{database: database}, nil
}

// ResolvedPriceRule is the effective whole-item rule, never an invented option price.
type ResolvedPriceRule struct {
	CurrencyCode, ChargeTypeCode, PeriodUnitCode, Kind string
	PeriodCount                                        int
	FixedAmountMinor                                   *int64
	DiscountBasisPoints                                *int
}

// PriceRuleReader exports negotiated rules to the trusted catalog adapter.
type PriceRuleReader interface {
	ResolvePriceRules(context.Context, string, string, time.Time) ([]ResolvedPriceRule, error)
}

// NewPriceRuleReader assembles the stateless rule projection at startup.
func NewPriceRuleReader(database gdb.DB) (PriceRuleReader, error) {
	if database == nil {
		return nil, gerror.New("sales price rule database is required")
	}
	return &serviceImpl{database: database}, nil
}
