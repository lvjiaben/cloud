// Package providerhealth owns database-authoritative Provider write protection.
package providerhealth

import (
	"context"
	"crypto/rand"
	"io"
	"time"
)

// Default protection constants preserve conservative bounded behavior before
// a merchant saves an explicit policy.
const (
	defaultWindowSampleLimit         = 20
	defaultMinimumSamplesToProtect   = 5
	defaultConsecutiveFailureLimit   = 3
	defaultUnknownResultLimit        = 2
	defaultFailureLimit              = 3
	defaultProtectionDurationSeconds = 300
	maximumGateChannelCount          = 32
)

// Operation kinds classify the exact Provider boundary being observed.
const (
	// HealthHealthy means the latest observed operation set is healthy.
	HealthHealthy HealthCode = "healthy"
	// HealthDegraded means at least one operation is degraded.
	HealthDegraded HealthCode = "degraded"
	// HealthUnavailable means automatic writes are currently protected.
	HealthUnavailable HealthCode = "unavailable"
	// HealthUnknown means no usable observation is available yet.
	HealthUnknown HealthCode = "unknown"
)

// Outcomes normalize Provider results without retaining raw upstream errors.
const (
	// OperationAuthentication identifies Provider authentication checks.
	OperationAuthentication OperationKind = "authentication"
	// OperationQuery identifies read-only Provider queries.
	OperationQuery OperationKind = "query"
	// OperationWrite identifies Provider writes governed by protection.
	OperationWrite OperationKind = "write"
	// OperationProviderCapacity identifies providerCapacity projection refreshes.
	OperationProviderCapacity OperationKind = "provider_capacity"
)

const (
	// OutcomeSucceeded identifies a confirmed successful Provider call.
	OutcomeSucceeded Outcome = "succeeded"
	// OutcomeFailed identifies a confirmed failed Provider call.
	OutcomeFailed Outcome = "failed"
	// OutcomeResultUnknown identifies a write whose final result is not known.
	OutcomeResultUnknown Outcome = "result_unknown"
)

// HealthCode is one normalized current health state exposed to management clients.
type HealthCode string

// OperationKind separates authentication, query, write, and providerCapacity health.
type OperationKind string

// Outcome is one normalized, non-sensitive Provider call result.
type Outcome string

// Observation is one bounded health input. SummaryCode must be a stable
// machine code and can never carry upstream error text.
type Observation struct {
	ChannelNo             string
	Operation             OperationKind
	Outcome               Outcome
	SummaryCode           string
	Latency               time.Duration
	AuthenticationFailure bool
}

// Gate is the exact owner decision for starting a new Provider write.
type Gate struct {
	ProtectedUntil time.Time
}

// AllowsWrite reports whether a new automatic write may start now.
func (g Gate) AllowsWrite() bool { return g.ProtectedUntil.IsZero() }

// Service owns database-authoritative channel health and write protection.
type Service interface {
	// CheckWrites returns only active write gates for at most 32 valid channels.
	// Empty input returns an empty map; validation or database failures return an error.
	CheckWrites(ctx context.Context, channelNos []string) (map[string]Gate, error)
	// CheckWrite returns one channel gate; a zero gate allows writes, while an
	// invalid channel or database failure returns an error.
	CheckWrite(ctx context.Context, channelNo string) (Gate, error)
	// Record atomically advances write state and appends immutable observations;
	// invalid or sensitive-shaped input and database failures return an error.
	Record(ctx context.Context, input Observation) error
}

type serviceImpl struct {
	random io.Reader
}

var _ Service = (*serviceImpl)(nil)

// New constructs the health owner over this plugin's generated DAO boundary.
func New() Service { return &serviceImpl{random: rand.Reader} }
