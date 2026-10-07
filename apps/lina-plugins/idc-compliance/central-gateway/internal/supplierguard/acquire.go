// This file acquires the only supplier write lease for an authenticated local
// business request under database-authoritative rate and circuit state.

package supplierguard

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const databaseOperationTimeout = 3 * time.Second

// Service owns no cache; every decision is made from the installation's PostgreSQL database.
type Service struct {
	database       *sql.DB
	installationID string
	policy         Policy
}

// NewService constructs one installation-local guard with reviewed deployment policy.
func NewService(database *sql.DB, installationID string, policy Policy) (*Service, error) {
	if database == nil || !installationIDPattern.MatchString(installationID) || !validPolicy(policy) {
		return nil, ErrInvalid
	}
	return &Service{database: database, installationID: installationID, policy: policy}, nil
}

// Acquire creates or recovers the only supplier write attempt for one business request.
func (s *Service) Acquire(ctx context.Context, input Admission) (lease Lease, err error) {
	if !s.canAcquire(ctx, input) {
		return Lease{}, ErrInvalid
	}
	if retryAfter, expired, expireErr := s.expireAbandonedProbe(
		ctx, input.Provider, input.OperationScope,
	); expireErr != nil {
		return Lease{}, expireErr
	} else if expired {
		return Lease{}, retryError(ErrCircuitOpen, retryAfter)
	}
	databaseContext, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Lease{}, ErrRejected
	}
	defer finishTransaction(transaction, &err)
	databaseNow, err := transactionTime(databaseContext, transaction)
	if err != nil {
		return Lease{}, ErrRejected
	}
	if err = lockActiveInstallation(
		databaseContext, transaction, s.installationID, input.Provider,
	); err != nil {
		return Lease{}, err
	}
	requestStatus, requestScope, err := lockBusinessRequest(
		databaseContext, transaction, input.RequestNo, s.installationID,
	)
	if err != nil || requestScope != input.OperationScope {
		return Lease{}, ErrConflict
	}
	existing, found, err := findAttemptForRequest(databaseContext, transaction, input.RequestNo)
	if err != nil {
		return Lease{}, ErrRejected
	}
	if found {
		if existing.Provider != input.Provider || existing.OperationScope != input.OperationScope ||
			existing.AttemptNo == "" {
			return Lease{}, ErrConflict
		}
		existing.CallSupplier = false
		existing.Recovered = true
		if err = transaction.Commit(); err != nil {
			return Lease{}, ErrRejected
		}
		return existing, nil
	}
	if requestStatus != "registered" {
		return Lease{}, ErrConflict
	}
	circuit, err := s.lockCircuit(databaseContext, transaction, input.Provider, input.OperationScope)
	if err != nil {
		return Lease{}, err
	}
	if err = s.validateCircuitPolicy(circuit); err != nil {
		return Lease{}, err
	}
	if circuit.Status == "half_open" {
		return Lease{}, retryError(ErrCircuitOpen, circuit.ProbeExpiresAt.Time.Sub(databaseNow))
	}
	probe := false
	if circuit.Status == "open" {
		if databaseNow.Before(circuit.OpenUntil.Time) {
			return Lease{}, retryError(ErrCircuitOpen, circuit.OpenUntil.Time.Sub(databaseNow))
		}
		probe = true
	}
	if err = consumeRateWindow(
		databaseContext, transaction, databaseNow, "installation", s.installationID, "",
		s.policy.Version, s.policy.InstallationRate,
	); err != nil {
		return Lease{}, err
	}
	if err = consumeRateWindow(
		databaseContext, transaction, databaseNow, "operation", s.installationID,
		input.OperationScope, s.policy.Version, s.policy.OperationRate,
	); err != nil {
		return Lease{}, err
	}
	attemptNo, err := generateAttemptNo()
	if err != nil {
		return Lease{}, ErrRejected
	}
	routeNo, err := generateRouteNo()
	if err != nil {
		return Lease{}, ErrRejected
	}
	circuitVersion := circuit.Version
	if probe {
		circuitVersion++
		result, updateErr := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_supplier_circuit
SET status_code = 'half_open', open_until = NULL, probe_attempt_no = $3,
    probe_expires_at = $4, updated_at = $5, version = $6
WHERE provider_code = $1 AND operation_scope = $2
  AND status_code = 'open' AND version = $7`,
			input.Provider, input.OperationScope, attemptNo,
			databaseNow.Add(s.policy.ProbeTimeout), databaseNow, circuitVersion, circuit.Version)
		if updateErr != nil || rowsAffected(result) != 1 {
			return Lease{}, ErrRejected
		}
	}
	result, err := transaction.ExecContext(databaseContext, `
INSERT INTO idc_gateway_local_supplier_write_attempt
    (attempt_no, request_no, installation_id, provider_code, operation_scope,
     circuit_version, probe)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		attemptNo, input.RequestNo, s.installationID, input.Provider,
		input.OperationScope, circuitVersion, probe)
	if err != nil || rowsAffected(result) != 1 {
		return Lease{}, ErrRejected
	}
	result, err = transaction.ExecContext(databaseContext, `
INSERT INTO idc_gateway_local_supplier_route
    (route_no, attempt_no, request_no, installation_id, provider_code, operation_scope)
VALUES ($1, $2, $3, $4, $5, $6)`,
		routeNo, attemptNo, input.RequestNo, s.installationID,
		input.Provider, input.OperationScope)
	if err != nil || rowsAffected(result) != 1 {
		return Lease{}, ErrRejected
	}
	result, err = transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_business_request
SET status_code = 'accepted'
WHERE request_no = $1 AND installation_id = $2 AND status_code = 'registered'`,
		input.RequestNo, s.installationID)
	if err != nil || rowsAffected(result) != 1 {
		return Lease{}, ErrRejected
	}
	if err = transaction.Commit(); err != nil {
		return Lease{}, ErrRejected
	}
	return Lease{
		AttemptNo: attemptNo, RouteNo: routeNo,
		Provider: input.Provider, OperationScope: input.OperationScope,
		CallSupplier: true, Probe: probe, CircuitVersion: circuitVersion, Status: "admitted",
	}, nil
}

func (s *Service) canAcquire(ctx context.Context, input Admission) bool {
	return s != nil && s.database != nil && ctx != nil && ctx.Err() == nil && validAdmission(input)
}

type circuitRecord struct {
	Provider                    string
	OperationScope              string
	PolicyVersion               int64
	SampleWindow                int
	MinimumSamples              int
	FailureThreshold            int
	UnknownThreshold            int
	ConsecutiveFailureThreshold int
	OpenSeconds                 int
	ProbeTimeoutSeconds         int
	Status                      string
	TotalSamples                int
	FailureSamples              int
	UnknownSamples              int
	ConsecutiveFailures         int
	OpenUntil                   sql.NullTime
	ProbeAttemptNo              sql.NullString
	ProbeExpiresAt              sql.NullTime
	Version                     int64
}

func (s *Service) lockCircuit(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
	operationScope string,
) (circuitRecord, error) {
	policy := s.policy
	_, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_circuit
    (provider_code, operation_scope, policy_version, sample_window,
     minimum_samples, failure_threshold, unknown_threshold,
     consecutive_failure_threshold, open_seconds, probe_timeout_seconds)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (provider_code, operation_scope) DO NOTHING`,
		provider, operationScope, policy.Version, policy.SampleWindow,
		policy.MinimumSamples, policy.FailureThreshold, policy.UnknownThreshold,
		policy.ConsecutiveFailureThreshold, int(policy.OpenDuration/time.Second),
		int(policy.ProbeTimeout/time.Second))
	if err != nil {
		return circuitRecord{}, ErrRejected
	}
	var result circuitRecord
	err = transaction.QueryRowContext(ctx, `
SELECT provider_code, operation_scope, policy_version, sample_window,
       minimum_samples, failure_threshold, unknown_threshold,
       consecutive_failure_threshold, open_seconds, probe_timeout_seconds,
       status_code, total_samples, failure_samples, unknown_samples,
       consecutive_failures, open_until, probe_attempt_no, probe_expires_at, version
FROM idc_gateway_supplier_circuit
WHERE provider_code = $1 AND operation_scope = $2
FOR UPDATE`, provider, operationScope).Scan(
		&result.Provider, &result.OperationScope, &result.PolicyVersion,
		&result.SampleWindow, &result.MinimumSamples, &result.FailureThreshold,
		&result.UnknownThreshold, &result.ConsecutiveFailureThreshold,
		&result.OpenSeconds, &result.ProbeTimeoutSeconds, &result.Status,
		&result.TotalSamples, &result.FailureSamples, &result.UnknownSamples,
		&result.ConsecutiveFailures, &result.OpenUntil, &result.ProbeAttemptNo,
		&result.ProbeExpiresAt, &result.Version,
	)
	if err != nil {
		return circuitRecord{}, ErrRejected
	}
	return result, nil
}

func (s *Service) validateCircuitPolicy(circuit circuitRecord) error {
	policy := s.policy
	if circuit.Provider == "" || circuit.PolicyVersion != policy.Version ||
		circuit.SampleWindow != policy.SampleWindow ||
		circuit.MinimumSamples != policy.MinimumSamples ||
		circuit.FailureThreshold != policy.FailureThreshold ||
		circuit.UnknownThreshold != policy.UnknownThreshold ||
		circuit.ConsecutiveFailureThreshold != policy.ConsecutiveFailureThreshold ||
		circuit.OpenSeconds != int(policy.OpenDuration/time.Second) ||
		circuit.ProbeTimeoutSeconds != int(policy.ProbeTimeout/time.Second) {
		return ErrRejected
	}
	return nil
}

func consumeRateWindow(
	ctx context.Context,
	transaction *sql.Tx,
	databaseNow time.Time,
	dimension string,
	installationID string,
	operationScope string,
	policyVersion int64,
	policy RatePolicy,
) error {
	windowSeconds := int(policy.Window / time.Second)
	_, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_rate_limit_window
    (dimension_kind, installation_id, operation_scope, policy_version,
     window_seconds, limit_count, window_started_at, request_count)
VALUES ($1, $2, $3, $4, $5, $6, $7, 0)
ON CONFLICT (dimension_kind, installation_id, operation_scope) DO NOTHING`,
		dimension, installationID, operationScope, policyVersion,
		windowSeconds, policy.Limit, databaseNow)
	if err != nil {
		return ErrRejected
	}
	var (
		storedVersion int64
		storedWindow  int
		storedLimit   int
		windowStarted time.Time
		requestCount  int
	)
	err = transaction.QueryRowContext(ctx, `
SELECT policy_version, window_seconds, limit_count, window_started_at, request_count
FROM idc_gateway_local_rate_limit_window
WHERE dimension_kind = $1 AND installation_id = $2 AND operation_scope = $3
FOR UPDATE`, dimension, installationID, operationScope).Scan(
		&storedVersion, &storedWindow, &storedLimit, &windowStarted, &requestCount,
	)
	if err != nil {
		return ErrRejected
	}
	if storedVersion > policyVersion ||
		(storedVersion == policyVersion &&
			(storedWindow != windowSeconds || storedLimit != policy.Limit)) {
		return ErrRejected
	}
	if storedVersion < policyVersion || !databaseNow.Before(windowStarted.Add(policy.Window)) {
		result, updateErr := transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_rate_limit_window
SET policy_version = $4, window_seconds = $5, limit_count = $6,
    window_started_at = $7, request_count = 1, updated_at = $7
WHERE dimension_kind = $1 AND installation_id = $2 AND operation_scope = $3`,
			dimension, installationID, operationScope, policyVersion,
			windowSeconds, policy.Limit, databaseNow)
		if updateErr != nil || rowsAffected(result) != 1 {
			return ErrRejected
		}
		return nil
	}
	if requestCount >= policy.Limit {
		return retryError(ErrRateLimited, windowStarted.Add(policy.Window).Sub(databaseNow))
	}
	result, err := transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_rate_limit_window
SET request_count = request_count + 1, updated_at = $4
WHERE dimension_kind = $1 AND installation_id = $2 AND operation_scope = $3
  AND request_count < limit_count`, dimension, installationID, operationScope, databaseNow)
	if err != nil || rowsAffected(result) != 1 {
		return ErrRejected
	}
	return nil
}

func lockBusinessRequest(
	ctx context.Context,
	transaction *sql.Tx,
	requestNo string,
	installationID string,
) (status string, scope string, err error) {
	err = transaction.QueryRowContext(ctx, `
SELECT status_code, operation_scope
FROM idc_gateway_local_business_request
WHERE request_no = $1 AND installation_id = $2
FOR UPDATE`, requestNo, installationID).Scan(&status, &scope)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrConflict
	}
	return status, scope, err
}

func lockActiveInstallation(
	ctx context.Context,
	transaction *sql.Tx,
	installationID string,
	provider string,
) error {
	var status, processingStatus string
	err := transaction.QueryRowContext(ctx, `
SELECT instance.status_code, processing.status_code
FROM idc_gateway_local_supplier_instance instance
JOIN idc_gateway_supplier_processing_state processing
  ON processing.provider_code = $2
WHERE instance.installation_id = $1
FOR SHARE OF instance, processing`, installationID, provider).Scan(&status, &processingStatus)
	if err != nil || status != "active" || processingStatus != "allowed" {
		return ErrRejected
	}
	return nil
}

func findAttemptForRequest(
	ctx context.Context,
	transaction *sql.Tx,
	requestNo string,
) (Lease, bool, error) {
	var (
		lease       Lease
		resultClass sql.NullString
	)
	err := transaction.QueryRowContext(ctx, `
SELECT a.attempt_no, r.route_no, a.provider_code, a.operation_scope,
       a.probe, a.circuit_version, a.status_code, a.result_class
FROM idc_gateway_local_supplier_write_attempt a
JOIN idc_gateway_local_supplier_route r ON r.attempt_no = a.attempt_no
WHERE a.request_no = $1`, requestNo).Scan(
		&lease.AttemptNo, &lease.RouteNo, &lease.Provider, &lease.OperationScope, &lease.Probe,
		&lease.CircuitVersion, &lease.Status, &resultClass,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Lease{}, false, nil
	}
	if err != nil {
		return Lease{}, false, err
	}
	if resultClass.Valid {
		lease.Result = ResultClass(resultClass.String)
	}
	return lease, true, nil
}

func generateAttemptNo() (string, error) {
	return generateRandomIdentifier("lwa_")
}

func generateRouteNo() (string, error) {
	return generateRandomIdentifier("lwr_")
}

func generateRandomIdentifier(prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		clear(value)
		return "", err
	}
	defer clear(value)
	return prefix + hex.EncodeToString(value), nil
}

func transactionTime(ctx context.Context, transaction *sql.Tx) (time.Time, error) {
	var value time.Time
	if err := transaction.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&value); err != nil {
		return time.Time{}, err
	}
	return value.UTC(), nil
}

func finishTransaction(transaction *sql.Tx, returnError *error) {
	rollbackError := transaction.Rollback()
	if rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) && *returnError == nil {
		*returnError = fmt.Errorf("rollback local supplier write guard: %w", rollbackError)
	}
}

func rowsAffected(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return count
}

func retryError(kind error, duration time.Duration) error {
	if duration < time.Second {
		duration = time.Second
	}
	if duration > time.Hour {
		duration = time.Hour
	}
	return &RetryError{Kind: kind, RetryAfter: duration.Round(time.Second)}
}

func stableCodeDigest(code string) [sha256.Size]byte {
	return sha256.Sum256([]byte("idc-supplier-health-code-v1\x00" + code))
}
