// This file defines the startup-frozen supplier collection contract and the
// bounded neutral observations accepted by the installation-local gateway.

package localusage

import (
	"context"
	"errors"
	"math"
	"regexp"
	"time"
)

const (
	// MaximumCollectionEvidenceBytes bounds one authoritative supplier frame.
	MaximumCollectionEvidenceBytes = 256 << 10
	// MaximumReconciliationRows bounds original-currency comparisons per collection.
	MaximumReconciliationRows = 8
	// CollectionPollInterval is the normal successful collection cadence.
	CollectionPollInterval = 15 * time.Minute
	// CollectionLeaseDuration fences one provider collection across local processes.
	CollectionLeaseDuration = 60 * time.Second
	// CollectionExecutionTimeout leaves time to persist before the lease expires.
	CollectionExecutionTimeout = 45 * time.Second
)

var (
	// ErrCollectorDisabled reports that no externally approved reader is installed.
	ErrCollectorDisabled = errors.New("supplier usage collector is disabled")
	// ErrCollectionRejected reports invalid or conflicting normalized evidence.
	ErrCollectionRejected = errors.New("supplier usage collection was rejected")
	// ErrCollectionLeaseLost reports a stale or superseded worker result.
	ErrCollectionLeaseLost = errors.New("supplier usage collection lease was lost")
	errCollectionConflict  = errors.New("supplier usage event conflicts with stored evidence")

	metricCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,47}$`)
)

// CollectionFailureCode is the only persisted supplier-read failure vocabulary.
type CollectionFailureCode string

const (
	// CollectionFailureTransport records an unavailable provider transport.
	CollectionFailureTransport CollectionFailureCode = "transport_unavailable"
	// CollectionFailureTimeout records a provider call deadline.
	CollectionFailureTimeout CollectionFailureCode = "timeout"
	// CollectionFailureMalformed records a response outside the neutral contract.
	CollectionFailureMalformed CollectionFailureCode = "malformed_response"
	// CollectionFailureRejected records an authoritative provider or replay conflict.
	CollectionFailureRejected CollectionFailureCode = "provider_rejected"
	// CollectionFailurePersistence records an unavailable local atomic commit.
	CollectionFailurePersistence CollectionFailureCode = "persistence_unavailable"
)

// CollectionError lets a provider reader return one stable failure category.
type CollectionError struct {
	// Code is the stable failure category safe to persist and monitor.
	Code CollectionFailureCode
}

func (e *CollectionError) Error() string {
	if e == nil || !validCollectionFailureCode(e.Code) {
		return ErrCollectionRejected.Error()
	}
	return string(e.Code)
}

// Collector performs one read-only provider collection and has no write method.
type Collector interface {
	// Collect returns one bounded provider-authoritative observation without writes.
	Collect(context.Context) (Collection, error)
}

// PlanObservation is one normalized authoritative provider plan observation.
type PlanObservation struct {
	// Reference is the provider plan identity and is never stored in plaintext.
	Reference []byte
	// StatusCode is the provider-neutral current plan state.
	StatusCode string
	// Currency is the optional original ISO 4217 currency code.
	Currency string
	// ValidFrom is the optional inclusive provider validity start.
	ValidFrom *time.Time
	// ValidUntil is the optional exclusive provider validity end.
	ValidUntil *time.Time
}

// UsageObservation is one normalized supplier usage and optional original-currency cost.
type UsageObservation struct {
	// MetricCode identifies one bounded provider-neutral usage metric.
	MetricCode string
	// PeriodStartedAt is the inclusive observation window start.
	PeriodStartedAt time.Time
	// PeriodEndedAt is the exclusive observation window end.
	PeriodEndedAt time.Time
	// QuantityNumerator is the non-negative exact quantity numerator.
	QuantityNumerator int64
	// QuantityDenominator is the positive exact quantity denominator.
	QuantityDenominator int64
	// SupplierCostMinor is the optional provider cost in minor units.
	SupplierCostMinor *int64
	// CurrencyCode is present exactly when SupplierCostMinor is present.
	CurrencyCode string
}

// ReconciliationInput keeps supplier and installation-local costs separate.
type ReconciliationInput struct {
	// PeriodStartedAt is the inclusive reconciliation window start.
	PeriodStartedAt time.Time
	// PeriodEndedAt is the exclusive reconciliation window end.
	PeriodEndedAt time.Time
	// CurrencyCode preserves the provider's original ISO 4217 currency.
	CurrencyCode string
	// SupplierCostMinor is the provider-authoritative amount in minor units.
	SupplierCostMinor int64
	// LocalObservedCostMinor is the same-currency local evidence amount.
	LocalObservedCostMinor int64
	// LocalEvidenceComplete controls exact comparison versus insufficient evidence.
	LocalEvidenceComplete bool
}

// SLAObservation contains bounded current-installation operation measurements.
type SLAObservation struct {
	// PeriodStartedAt is the inclusive local SLA measurement window start.
	PeriodStartedAt time.Time
	// PeriodEndedAt is the exclusive local SLA measurement window end.
	PeriodEndedAt time.Time
	// RequestCount is the bounded total request count.
	RequestCount int64
	// SuccessCount is the bounded successful request count.
	SuccessCount int64
	// LatencyP95Millis is the non-negative p95 latency in milliseconds.
	LatencyP95Millis int64
	// LatencyP99Millis is the p99 latency and cannot be below p95.
	LatencyP99Millis int64
}

// Collection is the provider-neutral result accepted by the local persistence core.
// EventID, plan reference, and evidence bytes are cleared after processing.
type Collection struct {
	// ProviderCode is one of the two fixed installation-local providers.
	ProviderCode string
	// EventID is the provider event identity and is stored only as a keyed digest.
	EventID []byte
	// EvidenceFrame is the bounded authoritative raw frame sealed before storage.
	EvidenceFrame []byte
	// ObservedAt is the provider observation time bounded by database time.
	ObservedAt time.Time
	// Plan is the single authoritative plan observation.
	Plan PlanObservation
	// Usage contains at most MaximumUsageRows bounded metric observations.
	Usage []UsageObservation
	// Reconciliations contains bounded original-currency comparison rows.
	Reconciliations []ReconciliationInput
	// SLA is the single current-installation measurement observation.
	SLA SLAObservation
}

// CollectionResult is the safe outcome of one local collection commit.
type CollectionResult struct {
	// ProviderCode identifies the committed provider projection.
	ProviderCode string
	// EvidenceNo is the local opaque encrypted-evidence identifier.
	EvidenceNo string
	// PlanSnapshotNo is the local opaque plan snapshot identifier.
	PlanSnapshotNo string
	// UsageRows is the number of usage rows committed or recovered.
	UsageRows int
	// BudgetAlerts is the number of configured alerts committed or recovered.
	BudgetAlerts int
	// Reconciliations is the number of reconciliation rows committed or recovered.
	Reconciliations int
	// SLASnapshotNo is the local opaque SLA snapshot identifier.
	SLASnapshotNo string
	// Replayed reports exact recovery of an already committed event.
	Replayed bool
}

func validCollection(input Collection) bool {
	if !validProvider(input.ProviderCode) || len(input.EventID) == 0 || len(input.EventID) > 256 ||
		len(input.EvidenceFrame) == 0 || len(input.EvidenceFrame) > MaximumCollectionEvidenceBytes ||
		input.ObservedAt.IsZero() || !validPlanObservation(input.Plan) ||
		len(input.Usage) > MaximumUsageRows ||
		len(input.Reconciliations) > MaximumReconciliationRows ||
		!validSLAObservation(input.SLA) {
		return false
	}
	for _, item := range input.Usage {
		if !validUsageObservation(item) {
			return false
		}
	}
	currencies := make(map[string]struct{}, len(input.Reconciliations))
	for _, item := range input.Reconciliations {
		if !validReconciliationInput(item) {
			return false
		}
		key := item.CurrencyCode + "\x00" + item.PeriodStartedAt.UTC().Format(time.RFC3339Nano) +
			"\x00" + item.PeriodEndedAt.UTC().Format(time.RFC3339Nano)
		if _, duplicate := currencies[key]; duplicate {
			return false
		}
		currencies[key] = struct{}{}
	}
	return true
}

func validPlanObservation(input PlanObservation) bool {
	if len(input.Reference) == 0 || len(input.Reference) > 512 {
		return false
	}
	switch input.StatusCode {
	case "active", "suspended", "expired", "unknown":
	default:
		return false
	}
	if input.Currency != "" && !validCurrency(input.Currency) {
		return false
	}
	if input.ValidUntil != nil {
		return input.ValidFrom != nil && input.ValidUntil.After(*input.ValidFrom)
	}
	return true
}

func validUsageObservation(input UsageObservation) bool {
	if !metricCodePattern.MatchString(input.MetricCode) ||
		!validPeriod(input.PeriodStartedAt, input.PeriodEndedAt) ||
		input.QuantityNumerator < 0 || input.QuantityDenominator <= 0 {
		return false
	}
	if input.SupplierCostMinor == nil {
		return input.CurrencyCode == ""
	}
	return *input.SupplierCostMinor >= 0 && validCurrency(input.CurrencyCode)
}

func validReconciliationInput(input ReconciliationInput) bool {
	return validPeriod(input.PeriodStartedAt, input.PeriodEndedAt) &&
		validCurrency(input.CurrencyCode) && input.SupplierCostMinor >= 0 &&
		input.SupplierCostMinor <= math.MaxInt64/2 &&
		input.LocalObservedCostMinor >= 0 && input.LocalObservedCostMinor <= math.MaxInt64/2
}

func validSLAObservation(input SLAObservation) bool {
	const maximumCount = int64(1_000_000_000_000)
	return validPeriod(input.PeriodStartedAt, input.PeriodEndedAt) &&
		input.RequestCount >= 0 && input.RequestCount <= maximumCount &&
		input.SuccessCount >= 0 && input.SuccessCount <= input.RequestCount &&
		input.LatencyP95Millis >= 0 && input.LatencyP99Millis >= input.LatencyP95Millis
}

func validPeriod(from, to time.Time) bool {
	return !from.IsZero() && !to.IsZero() && to.After(from) && to.Sub(from) <= maximumQueryWindow
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validCollectionFailureCode(code CollectionFailureCode) bool {
	switch code {
	case CollectionFailureTransport, CollectionFailureTimeout, CollectionFailureMalformed,
		CollectionFailureRejected, CollectionFailurePersistence:
		return true
	default:
		return false
	}
}

func collectionFailureCode(err error) CollectionFailureCode {
	if errors.Is(err, context.DeadlineExceeded) {
		return CollectionFailureTimeout
	}
	var classified *CollectionError
	if errors.As(err, &classified) && validCollectionFailureCode(classified.Code) {
		return classified.Code
	}
	return CollectionFailureTransport
}

func clearCollection(input *Collection) {
	if input == nil {
		return
	}
	clear(input.EventID)
	clear(input.EvidenceFrame)
	clear(input.Plan.Reference)
	input.EventID = nil
	input.EvidenceFrame = nil
	input.Plan.Reference = nil
}
