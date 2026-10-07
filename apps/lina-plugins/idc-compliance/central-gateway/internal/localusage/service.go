// Package localusage exposes bounded read-only projections of the current
// installation's direct supplier plan, usage, cost evidence, reconciliation,
// and SLA. It has no balance, credit, price publication, or payment methods.
package localusage

import (
	"context"
	"errors"
	"reflect"
	"time"
)

const (
	ProviderTencentESSBasic = "tencent_essbasic"
	ProviderNuonuoInvoice   = "nuonuo_invoice"
	MaximumUsageRows        = 200
	maximumQueryWindow      = 400 * 24 * time.Hour
)

var (
	ErrChargeUnavailable = errors.New("local supplier plan does not allow a new charged operation")
	ErrInvalid           = errors.New("local supplier usage query is invalid")
	ErrUnavailable       = errors.New("local supplier usage projection is unavailable")
)

type Plan struct {
	ProviderCode string
	StatusCode   string
	CurrencyCode string
	ValidFrom    *time.Time
	ValidUntil   *time.Time
	ObservedAt   time.Time
}

type Usage struct {
	ProviderCode        string
	MetricCode          string
	PeriodStartedAt     time.Time
	PeriodEndedAt       time.Time
	QuantityNumerator   int64
	QuantityDenominator int64
	SupplierCostMinor   *int64
	CurrencyCode        string
	ObservedAt          time.Time
}

type Reconciliation struct {
	ProviderCode           string
	PeriodStartedAt        time.Time
	PeriodEndedAt          time.Time
	CurrencyCode           string
	SupplierCostMinor      int64
	LocalObservedCostMinor int64
	DifferenceMinor        int64
	StatusCode             string
	ObservedAt             time.Time
}

type SLA struct {
	ProviderCode     string
	PeriodStartedAt  time.Time
	PeriodEndedAt    time.Time
	RequestCount     int64
	SuccessCount     int64
	AvailabilityPPM  int
	LatencyP95Millis int64
	LatencyP99Millis int64
}

type Query struct {
	ProviderCode string
	From         time.Time
	To           time.Time
	Limit        int
}

type Projection struct {
	Plan           *Plan
	Usage          []Usage
	Reconciliation *Reconciliation
	SLA            *SLA
}

// Store deliberately exposes only bounded reads from the local database.
type Store interface {
	LatestPlan(context.Context, string) (*Plan, error)
	ListUsage(context.Context, string, time.Time, time.Time, int) ([]Usage, error)
	LatestReconciliation(context.Context, string, time.Time, time.Time) (*Reconciliation, error)
	LatestSLA(context.Context, string, time.Time, time.Time) (*SLA, error)
}

type Service struct{ store Store }

func New(store Store) (*Service, error) {
	if nilDependency(store) {
		return nil, ErrUnavailable
	}
	return &Service{store: store}, nil
}

// AdmitNewCharge blocks only when the latest local supplier projection
// positively proves that this buyer account cannot start another charged
// operation. Missing projections remain subject to the operation's own
// runtime capability check; they never fall back to a software-vendor service.
func (s *Service) AdmitNewCharge(ctx context.Context, provider string) error {
	if s == nil || ctx == nil || ctx.Err() != nil || !validProvider(provider) {
		return ErrInvalid
	}
	plan, err := s.store.LatestPlan(ctx, provider)
	if err != nil {
		return ErrUnavailable
	}
	if plan == nil {
		return nil
	}
	switch plan.StatusCode {
	case "quota_exhausted", "authorization_missing", "suspended", "revoked", "expired":
		return ErrChargeUnavailable
	default:
		return nil
	}
}

// Get performs exactly four bounded local reads and never contacts a supplier
// or software-vendor service. Costs remain grouped by their original currency.
func (s *Service) Get(ctx context.Context, query Query) (*Projection, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !validProvider(query.ProviderCode) ||
		query.From.IsZero() || query.To.IsZero() || !query.To.After(query.From) ||
		query.To.Sub(query.From) > maximumQueryWindow || query.Limit < 1 ||
		query.Limit > MaximumUsageRows {
		return nil, ErrInvalid
	}
	plan, err := s.store.LatestPlan(ctx, query.ProviderCode)
	if err != nil {
		return nil, ErrUnavailable
	}
	usage, err := s.store.ListUsage(
		ctx, query.ProviderCode, query.From.UTC(), query.To.UTC(), query.Limit,
	)
	if err != nil || len(usage) > query.Limit {
		return nil, ErrUnavailable
	}
	reconciliation, err := s.store.LatestReconciliation(
		ctx, query.ProviderCode, query.From.UTC(), query.To.UTC(),
	)
	if err != nil {
		return nil, ErrUnavailable
	}
	sla, err := s.store.LatestSLA(ctx, query.ProviderCode, query.From.UTC(), query.To.UTC())
	if err != nil {
		return nil, ErrUnavailable
	}
	return &Projection{
		Plan: plan, Usage: append([]Usage(nil), usage...),
		Reconciliation: reconciliation, SLA: sla,
	}, nil
}

func validProvider(provider string) bool {
	return provider == ProviderTencentESSBasic || provider == ProviderNuonuoInvoice
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
