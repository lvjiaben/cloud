package commerce

import (
	"context"
	"time"
)

const (
	QuoteAdjustmentCampaignDiscount = "campaign_discount"
	QuoteAdjustmentCoupon           = "coupon"
	QuoteAdjustmentVoucher          = "voucher"
	QuoteAdjustmentCustomerPrice    = "customer_price"
	QuoteAdjustmentAgentPrice       = "agent_price"
	QuoteAdjustmentManualDiscount   = "manual_discount"
)

// QuoteRequest carries cart concurrency and explicitly selected customer assets.
// Asset ownership and eligibility remain authoritative in the adjustment owner.
type QuoteRequest struct {
	ExpectedCartVersion int64
	AssetNumbers        []string
}

// QuoteAdjustmentLineInput is the tax-exclusive, pre-adjustment amount exposed
// to a quote-wide local adjustment evaluator.
type QuoteAdjustmentLineInput struct {
	ReferenceNo   string
	ProductNo     string
	CategoryNo    string
	SubtotalMinor int64
}

// QuoteAdjustmentInput is evaluated once per quote, never once per item.
type QuoteAdjustmentInput struct {
	QuoteNo       string
	CustomerNo    string
	CurrencyCode  string
	PurchasePhase string
	CycleNumber   int
	IssuedAt      time.Time
	AssetNumbers  []string
	Lines         []QuoteAdjustmentLineInput
}

// QuoteAdjustment is already allocated to one quote line. Digests and optional
// reservations are authoritative facts from the adjustment owner.
type QuoteAdjustment struct {
	ReferenceNo          string
	TypeCode             string
	SourceReference      string
	SourceVersionDigest  string
	ReservationReference string
	AmountMinor          int64
}

// QuoteAdjustmentResult expires no later than its earliest local reservation.
type QuoteAdjustmentResult struct {
	Adjustments []QuoteAdjustment
	ExpiresAt   *time.Time
}

// QuoteAdjustmentEvaluator must join the caller's local database transaction.
// Implementations may reserve local quota but must never perform network I/O.
type QuoteAdjustmentEvaluator interface {
	EvaluateAndReserveQuote(context.Context, QuoteAdjustmentInput) (*QuoteAdjustmentResult, error)
}

// QuoteReservationFinalizer is implemented by adjustment owners that reserve
// quota while producing an immutable quote. Checkout invokes it only when the
// persisted quote contains adjustment rows and always inside the order transaction.
type QuoteReservationFinalizer interface {
	ConsumeQuote(context.Context, string, string) error
}

// QuoteReservationReverser returns consumed quota only after the commerce
// owner proves the originating order was never paid.
type QuoteReservationReverser interface {
	ReverseConsumedQuote(context.Context, string, string) error
}

type noQuoteReservationFinalizer struct{}

func (noQuoteReservationFinalizer) ConsumeQuote(context.Context, string, string) error { return nil }

type noQuoteReservationReverser struct{}

func (noQuoteReservationReverser) ReverseConsumedQuote(context.Context, string, string) error {
	return nil
}

type noQuoteAdjustments struct{}

func (noQuoteAdjustments) EvaluateAndReserveQuote(
	context.Context,
	QuoteAdjustmentInput,
) (*QuoteAdjustmentResult, error) {
	return &QuoteAdjustmentResult{Adjustments: []QuoteAdjustment{}}, nil
}

// QuoteAdjustmentView is the immutable customer-visible discount explanation.
type QuoteAdjustmentView struct {
	TypeCode        string
	SourceReference string
	AmountMinor     int64
}

// QuoteItem is an immutable final quote item projection.
type QuoteItem struct {
	QuoteItemNo           string
	CartItemNo            string
	ProductNameZhCN       string
	ProductNameEnUS       string
	PublicSlug            string
	ConfigurationRequired bool
	ConfigurationFrozen   bool
	ConfigurationRevision string
	Quantity              int
	PurchaseCycles        int
	FulfillmentModeCode   string
	RiskDecisionCode      string
	InventoryDecisionCode string
	SetupSubtotalMinor    int64
	PeriodSubtotalMinor   int64
	SubtotalMinor         int64
	DiscountMinor         int64
	NetAmountMinor        int64
	TaxAmountMinor        int64
	TotalAmountMinor      int64
	Adjustments           []QuoteAdjustmentView
}

// Quote is a newly persisted immutable final quote.
type Quote struct {
	QuoteNo          string
	CartNo           string
	CurrencyCode     string
	CartVersion      int64
	SubtotalMinor    int64
	DiscountMinor    int64
	NetAmountMinor   int64
	TaxAmountMinor   int64
	TotalAmountMinor int64
	IssuedAt         time.Time
	ExpiresAt        time.Time
	Items            []QuoteItem
}
