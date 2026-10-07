// Package marketing owns deterministic promotion eligibility and reservation state.
package marketing

import (
	"context"
	"crypto/rand"
	"io"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	PurchasePhaseNewPurchase = "new_purchase"
	PurchasePhaseRenewal     = "renewal"

	ReservationStatusActive   = "active"
	ReservationStatusConsumed = "consumed"
	ReservationStatusReleased = "released"
	ReservationStatusExpired  = "expired"
	ReservationStatusReversed = "reversed"

	maximumPromotionCandidates = 512
	maximumPromotionScopes     = 4096
	maximumPromotionAssets     = 64
	defaultReservationTTL      = 15 * time.Minute
	maximumReservationTTL      = 2 * time.Hour
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	currencyPattern   = regexp.MustCompile(`^[A-Z]{3}$`)

	CodeInvalid = bizerr.MustDefine(
		"IDC_MARKETING_PROMOTION_INVALID",
		"The promotion request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_MARKETING_PROMOTION_CONFLICT",
		"The promotion request conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeQuotaUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_PROMOTION_QUOTA_UNAVAILABLE",
		"The promotion quota is no longer available",
		gcode.CodeInvalidOperation,
	)
	CodeReservationExpired = bizerr.MustDefine(
		"IDC_MARKETING_RESERVATION_EXPIRED",
		"The promotion reservation has expired",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_PROMOTION_UNAVAILABLE",
		"The promotion service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodePublicCampaignInvalid = bizerr.MustDefine(
		"IDC_MARKETING_PUBLIC_CAMPAIGN_INVALID",
		"The public campaign request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodePublicCampaignUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_PUBLIC_CAMPAIGN_UNAVAILABLE",
		"Public campaigns are temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodePointsInvalid = bizerr.MustDefine(
		"IDC_MARKETING_POINTS_INVALID",
		"The points request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodePointsConflict = bizerr.MustDefine(
		"IDC_MARKETING_POINTS_CONFLICT",
		"The points request conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodePointsUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_POINTS_UNAVAILABLE",
		"The points service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeValueGrantInvalid = bizerr.MustDefine(
		"IDC_MARKETING_VALUE_GRANT_INVALID",
		"The marketing value grant request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeValueGrantConflict = bizerr.MustDefine(
		"IDC_MARKETING_VALUE_GRANT_CONFLICT",
		"The marketing value grant request conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeValueGrantUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_VALUE_GRANT_UNAVAILABLE",
		"The marketing value grant service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeEngagementInvalid = bizerr.MustDefine(
		"IDC_MARKETING_ENGAGEMENT_INVALID",
		"The marketing engagement request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeEngagementConflict = bizerr.MustDefine(
		"IDC_MARKETING_ENGAGEMENT_CONFLICT",
		"The marketing engagement request conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeEngagementUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_ENGAGEMENT_UNAVAILABLE",
		"The marketing engagement service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeManagementOverviewNotAccessible = bizerr.MustDefine(
		"IDC_MARKETING_OVERVIEW_NOT_ACCESSIBLE",
		"The marketing operations overview is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeManagementOverviewUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_OVERVIEW_UNAVAILABLE",
		"The marketing operations overview is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeManagementDirectoryInvalid = bizerr.MustDefine(
		"IDC_MARKETING_DIRECTORY_INVALID",
		"The marketing management request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeManagementDirectoryNotAccessible = bizerr.MustDefine(
		"IDC_MARKETING_DIRECTORY_NOT_ACCESSIBLE",
		"The requested marketing management data is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeManagementDirectoryNotFound = bizerr.MustDefine(
		"IDC_MARKETING_DIRECTORY_NOT_FOUND",
		"The requested marketing management resource was not found",
		gcode.CodeNotFound,
	)
	CodeManagementDirectoryUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_DIRECTORY_UNAVAILABLE",
		"The marketing management directory is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeManagementActionInvalid = bizerr.MustDefine(
		"IDC_MARKETING_ACTION_INVALID",
		"The marketing management action is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeManagementActionNotAccessible = bizerr.MustDefine(
		"IDC_MARKETING_ACTION_NOT_ACCESSIBLE",
		"The requested marketing management action is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeManagementActionConflict = bizerr.MustDefine(
		"IDC_MARKETING_ACTION_CONFLICT",
		"The marketing management action conflicts with the current state",
		gcode.CodeInvalidOperation,
	)
	CodeManagementActionUnavailable = bizerr.MustDefine(
		"IDC_MARKETING_ACTION_UNAVAILABLE",
		"The marketing management action is temporarily unavailable",
		gcode.CodeInternalError,
	)
)

// Config bounds how long promotion quota may remain reserved without payment.
type Config struct {
	ReservationTTL time.Duration
}

// EvaluateInput contains only locally authoritative quote and customer facts.
type EvaluateInput struct {
	CustomerNo     string
	QuoteReference string
	CurrencyCode   string
	SubtotalMinor  int64
	PurchasePhase  string
	CycleNumber    int
	ProductNo      string
	CategoryNo     string
	AssetNumbers   []string
}

// AppliedBenefit is one selected, ordered discount with its immutable evidence.
type AppliedBenefit struct {
	CampaignNo            string
	BenefitNo             string
	AssetNo               string
	BenefitType           string
	StackPolicy           string
	StackGroup            string
	Priority              int
	DiscountAmountMinor   int64
	BenefitSnapshotDigest string
}

// ExcludedBenefit explains an otherwise eligible rule omitted by deterministic stacking.
type ExcludedBenefit struct {
	CampaignNo string
	BenefitNo  string
	ReasonCode string
}

// Evaluation is the deterministic local promotion projection for one quote.
type Evaluation struct {
	CurrencyCode       string
	SubtotalMinor      int64
	DiscountTotalMinor int64
	PayableMinor       int64
	Applied            []AppliedBenefit
	Excluded           []ExcludedBenefit
}

// Reservation is one quote-wide set of campaign and optional asset reservations.
type Reservation struct {
	CustomerNo     string
	QuoteReference string
	CurrencyCode   string
	Status         string
	ExpiresAt      time.Time
	Applied        []AppliedBenefit
	Reused         bool
}

// QuoteLineInput is one tax-exclusive line in a quote-wide promotion decision.
type QuoteLineInput struct {
	ReferenceNo   string
	ProductNo     string
	CategoryNo    string
	SubtotalMinor int64
}

// QuoteReserveInput is evaluated and reserved once for the complete quote.
type QuoteReserveInput struct {
	CustomerNo     string
	QuoteReference string
	CurrencyCode   string
	PurchasePhase  string
	CycleNumber    int
	AssetNumbers   []string
	Lines          []QuoteLineInput
}

// QuoteBenefitAllocation assigns one selected campaign discount to a line.
type QuoteBenefitAllocation struct {
	LineReference string
	AmountMinor   int64
}

// QuoteAppliedBenefit includes the campaign reservation and exact line allocation.
type QuoteAppliedBenefit struct {
	AppliedBenefit
	ReservationReference string
	Allocations          []QuoteBenefitAllocation
}

// QuoteReservation is the atomic, quote-wide marketing result.
type QuoteReservation struct {
	CustomerNo     string
	QuoteReference string
	CurrencyCode   string
	ExpiresAt      *time.Time
	Applied        []QuoteAppliedBenefit
}

// Service exposes a local-only promotion boundary. Callers own customer data authorization.
type Service interface {
	Evaluate(ctx context.Context, input EvaluateInput) (*Evaluation, error)
	Reserve(ctx context.Context, input EvaluateInput) (*Reservation, error)
	Consume(ctx context.Context, customerNo string, quoteReference string) (*Reservation, error)
	Release(ctx context.Context, customerNo string, quoteReference string) (*Reservation, error)
	// ReverseConsumed returns quota consumed by an order that was cancelled
	// before any payment. Only an entirely consumed quote can be reversed.
	ReverseConsumed(ctx context.Context, customerNo string, quoteReference string) (*Reservation, error)
	ReserveQuote(ctx context.Context, input QuoteReserveInput) (*QuoteReservation, error)
}

type serviceImpl struct {
	database       gdb.DB
	reservationTTL time.Duration
	random         io.Reader
}

var _ Service = (*serviceImpl)(nil)

// New constructs the promotion service without starting background work.
func New(database gdb.DB, config Config) (Service, error) {
	return newService(database, config, rand.Reader)
}

func newService(database gdb.DB, config Config, random io.Reader) (*serviceImpl, error) {
	if database == nil {
		return nil, gerror.New("marketing service requires a database")
	}
	ttl := config.ReservationTTL
	if ttl == 0 {
		ttl = defaultReservationTTL
	}
	if ttl <= 0 || ttl > maximumReservationTTL {
		return nil, gerror.New("marketing reservation TTL is outside the safe range")
	}
	if random == nil {
		return nil, gerror.New("marketing service requires a random source")
	}
	return &serviceImpl{database: database, reservationTTL: ttl, random: random}, nil
}
