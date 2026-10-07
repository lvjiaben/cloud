// Package supplierguard enforces installation-local PostgreSQL rate limits,
// one-shot supplier write leases, and persistent provider health protection.
package supplierguard

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	ProviderTencentESSBasic = "tencent_essbasic"
	ProviderNuonuoInvoice   = "nuonuo_invoice"

	ResultSucceeded          ResultClass = "succeeded"
	ResultDefinitiveFailure  ResultClass = "definitive_failure"
	ResultAvailabilityFailed ResultClass = "availability_failure"
	ResultUnknown            ResultClass = "result_unknown"
)

var (
	ErrInvalid     = errors.New("local supplier write guard input is invalid")
	ErrRejected    = errors.New("local supplier write guard rejected the operation")
	ErrConflict    = errors.New("local supplier write guard found conflicting state")
	ErrRateLimited = errors.New("local supplier write rate limit is active")
	ErrCircuitOpen = errors.New("local supplier write circuit is open")

	requestNoPattern      = regexp.MustCompile(`^lbr_[0-9a-f]{32}$`)
	attemptNoPattern      = regexp.MustCompile(`^lwa_[0-9a-f]{32}$`)
	installationIDPattern = regexp.MustCompile(`^linst_[0-9a-f]{64}$`)
	operationScopePattern = regexp.MustCompile(`^[a-z][a-z0-9.:-]{1,63}$`)
)

// ResultClass is the closed observation vocabulary for a real supplier write.
type ResultClass string

// RatePolicy is one PostgreSQL fixed-window limit.
type RatePolicy struct {
	Window time.Duration
	Limit  int
}

// Policy is versioned deployment-owned guard configuration. No browser or
// management API can alter it in the current delivery slice.
type Policy struct {
	Version                     int64
	InstallationRate            RatePolicy
	OperationRate               RatePolicy
	SampleWindow                int
	MinimumSamples              int
	FailureThreshold            int
	UnknownThreshold            int
	ConsecutiveFailureThreshold int
	OpenDuration                time.Duration
	ProbeTimeout                time.Duration
}

// DefaultPolicy returns the first reviewed installation-local safety policy.
func DefaultPolicy() Policy {
	return Policy{
		Version:                     1,
		InstallationRate:            RatePolicy{Window: time.Minute, Limit: 120},
		OperationRate:               RatePolicy{Window: time.Minute, Limit: 30},
		SampleWindow:                20,
		MinimumSamples:              5,
		FailureThreshold:            3,
		UnknownThreshold:            2,
		ConsecutiveFailureThreshold: 3,
		OpenDuration:                5 * time.Minute,
		ProbeTimeout:                time.Minute,
	}
}

// Admission identifies one already authenticated local business request.
type Admission struct {
	RequestNo      string
	Provider       string
	OperationScope string
}

// Lease is the only authority to cross a supplier write boundary. CallSupplier
// is true exactly once for a business request.
type Lease struct {
	AttemptNo      string
	RouteNo        string
	Provider       string
	OperationScope string
	CallSupplier   bool
	Recovered      bool
	Probe          bool
	CircuitVersion int64
	Status         string
	Result         ResultClass
}

// Observation records one and only one outcome after a lease crossed the
// supplier boundary. StableCode is hashed before persistence.
type Observation struct {
	AttemptNo  string
	Result     ResultClass
	StableCode string
	Elapsed    time.Duration
}

// RetryError exposes only a bounded retry delay and a stable error category.
type RetryError struct {
	Kind       error
	RetryAfter time.Duration
}

func (e *RetryError) Error() string {
	if e == nil || e.Kind == nil {
		return ErrRejected.Error()
	}
	return fmt.Sprintf("%s; retry after %s", e.Kind.Error(), e.RetryAfter)
}

func (e *RetryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

func validPolicy(policy Policy) bool {
	return policy.Version > 0 && validRatePolicy(policy.InstallationRate) &&
		validRatePolicy(policy.OperationRate) &&
		policy.SampleWindow >= 5 && policy.SampleWindow <= 100 &&
		policy.MinimumSamples >= 1 && policy.MinimumSamples <= policy.SampleWindow &&
		policy.FailureThreshold >= 1 && policy.FailureThreshold <= policy.SampleWindow &&
		policy.UnknownThreshold >= 1 && policy.UnknownThreshold <= policy.SampleWindow &&
		policy.ConsecutiveFailureThreshold >= 1 &&
		policy.ConsecutiveFailureThreshold <= policy.SampleWindow &&
		policy.OpenDuration >= time.Minute && policy.OpenDuration <= 24*time.Hour &&
		policy.OpenDuration%time.Second == 0 &&
		policy.ProbeTimeout >= time.Second && policy.ProbeTimeout <= 10*time.Minute &&
		policy.ProbeTimeout%time.Second == 0
}

func validRatePolicy(policy RatePolicy) bool {
	return policy.Window >= time.Second && policy.Window <= time.Hour &&
		policy.Window%time.Second == 0 && policy.Limit >= 1 && policy.Limit <= 100000
}

func validAdmission(input Admission) bool {
	return requestNoPattern.MatchString(input.RequestNo) && validProvider(input.Provider) &&
		operationScopePattern.MatchString(input.OperationScope)
}

func validProvider(provider string) bool {
	return provider == ProviderTencentESSBasic || provider == ProviderNuonuoInvoice
}

func validObservation(input Observation) bool {
	if !attemptNoPattern.MatchString(input.AttemptNo) || len(input.StableCode) == 0 ||
		len(input.StableCode) > 64 || input.Elapsed < 0 || input.Elapsed > time.Hour ||
		input.Elapsed%time.Millisecond != 0 {
		return false
	}
	switch input.Result {
	case ResultSucceeded, ResultDefinitiveFailure, ResultAvailabilityFailed, ResultUnknown:
		return true
	default:
		return false
	}
}
