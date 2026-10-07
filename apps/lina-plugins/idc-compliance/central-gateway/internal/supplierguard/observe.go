// This file records one supplier outcome and advances only the matching
// PostgreSQL circuit generation.

package supplierguard

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"
)

type attemptRecord struct {
	AttemptNo      string
	RequestNo      string
	InstallationID string
	Provider       string
	OperationScope string
	CircuitVersion int64
	Probe          bool
	Status         string
	Result         sql.NullString
	StableDigest   []byte
	ElapsedMS      sql.NullInt64
	RouteNo        string
}

type sampleCounts struct {
	Total               int
	Failures            int
	Unknowns            int
	ConsecutiveFailures int
}

// Observe commits exactly one result for an admitted supplier write. Exact
// repeats recover; changed observations conflict and never alter stored fact.
func (s *Service) Observe(ctx context.Context, input Observation) (lease Lease, err error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || !validObservation(input) {
		return Lease{}, ErrInvalid
	}
	databaseContext, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	route, err := s.readAttemptRoute(databaseContext, input.AttemptNo)
	if err != nil {
		return Lease{}, err
	}
	transaction, err := s.database.BeginTx(databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Lease{}, ErrRejected
	}
	defer finishTransaction(transaction, &err)
	databaseNow, err := transactionTime(databaseContext, transaction)
	if err != nil {
		return Lease{}, ErrRejected
	}
	requestStatus, requestScope, err := lockBusinessRequest(
		databaseContext, transaction, route.RequestNo, s.installationID,
	)
	if err != nil || requestScope != route.OperationScope {
		return Lease{}, ErrConflict
	}
	circuit, err := s.lockCircuit(
		databaseContext, transaction, route.Provider, route.OperationScope,
	)
	if err != nil {
		return Lease{}, err
	}
	if err = s.validateCircuitPolicy(circuit); err != nil {
		return Lease{}, err
	}
	attempt, err := lockAttempt(databaseContext, transaction, input.AttemptNo)
	if err != nil || !sameAttemptRoute(route, attempt) {
		return Lease{}, ErrConflict
	}
	digest := stableCodeDigest(input.StableCode)
	elapsedMilliseconds := input.Elapsed.Milliseconds()
	if attempt.Status == "observed" {
		if !attempt.Result.Valid || ResultClass(attempt.Result.String) != input.Result ||
			!bytes.Equal(attempt.StableDigest, digest[:]) ||
			!attempt.ElapsedMS.Valid || attempt.ElapsedMS.Int64 != elapsedMilliseconds {
			return Lease{}, ErrConflict
		}
		if err = transaction.Commit(); err != nil {
			return Lease{}, ErrRejected
		}
		return leaseFromAttempt(attempt, true), nil
	}
	if attempt.Status != "admitted" || requestStatus != "accepted" {
		return Lease{}, ErrConflict
	}
	result, err := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_write_attempt
SET status_code = 'observed', result_class = $2, stable_code_sha256 = $3,
    elapsed_milliseconds = $4, observed_at = $5
WHERE attempt_no = $1 AND status_code = 'admitted'`,
		input.AttemptNo, string(input.Result), digest[:], elapsedMilliseconds, databaseNow)
	if err != nil || rowsAffected(result) != 1 {
		return Lease{}, ErrRejected
	}
	businessStatus := businessStatusFor(input.Result)
	result, err = transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_business_request
SET status_code = $3
WHERE request_no = $1 AND installation_id = $2 AND status_code = 'accepted'`,
		attempt.RequestNo, s.installationID, businessStatus)
	if err != nil || rowsAffected(result) != 1 {
		return Lease{}, ErrRejected
	}
	healthResult := healthResultFor(input.Result)
	result, err = transaction.ExecContext(databaseContext, `
INSERT INTO idc_gateway_supplier_health_sample
    (attempt_no, provider_code, operation_scope, circuit_version,
     result_class, stable_code_sha256, elapsed_milliseconds, observed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		attempt.AttemptNo, attempt.Provider, attempt.OperationScope,
		attempt.CircuitVersion, healthResult, digest[:], elapsedMilliseconds, databaseNow)
	if err != nil || rowsAffected(result) != 1 {
		return Lease{}, ErrRejected
	}
	if err = pruneSamples(
		databaseContext, transaction, attempt.Provider, attempt.OperationScope,
		s.policy.SampleWindow,
	); err != nil {
		return Lease{}, err
	}
	counts, err := loadSampleCounts(
		databaseContext, transaction, attempt.Provider, attempt.OperationScope,
		attempt.CircuitVersion, s.policy.SampleWindow,
	)
	if err != nil {
		return Lease{}, err
	}
	if err = s.advanceCircuit(
		databaseContext, transaction, databaseNow, circuit, attempt, input.Result, counts,
	); err != nil {
		return Lease{}, err
	}
	if err = transaction.Commit(); err != nil {
		return Lease{}, ErrRejected
	}
	attempt.Status = "observed"
	attempt.Result = sql.NullString{String: string(input.Result), Valid: true}
	attempt.StableDigest = append([]byte(nil), digest[:]...)
	attempt.ElapsedMS = sql.NullInt64{Int64: elapsedMilliseconds, Valid: true}
	return leaseFromAttempt(attempt, false), nil
}

func (s *Service) readAttemptRoute(ctx context.Context, attemptNo string) (attemptRecord, error) {
	var result attemptRecord
	err := s.database.QueryRowContext(ctx, `
SELECT a.attempt_no, a.request_no, a.installation_id, a.provider_code, a.operation_scope,
       a.circuit_version, a.probe, a.status_code, a.result_class,
       a.stable_code_sha256, a.elapsed_milliseconds, r.route_no
FROM idc_gateway_local_supplier_write_attempt a
JOIN idc_gateway_local_supplier_route r ON r.attempt_no = a.attempt_no
WHERE a.attempt_no = $1`, attemptNo).Scan(
		&result.AttemptNo, &result.RequestNo, &result.InstallationID,
		&result.Provider, &result.OperationScope, &result.CircuitVersion,
		&result.Probe, &result.Status, &result.Result, &result.StableDigest,
		&result.ElapsedMS, &result.RouteNo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return attemptRecord{}, ErrConflict
	}
	if err != nil || result.InstallationID != s.installationID {
		return attemptRecord{}, ErrRejected
	}
	return result, nil
}

func lockAttempt(ctx context.Context, transaction *sql.Tx, attemptNo string) (attemptRecord, error) {
	var result attemptRecord
	err := transaction.QueryRowContext(ctx, `
SELECT a.attempt_no, a.request_no, a.installation_id, a.provider_code, a.operation_scope,
       a.circuit_version, a.probe, a.status_code, a.result_class,
       a.stable_code_sha256, a.elapsed_milliseconds, r.route_no
FROM idc_gateway_local_supplier_write_attempt a
JOIN idc_gateway_local_supplier_route r ON r.attempt_no = a.attempt_no
WHERE a.attempt_no = $1
FOR UPDATE`, attemptNo).Scan(
		&result.AttemptNo, &result.RequestNo, &result.InstallationID,
		&result.Provider, &result.OperationScope, &result.CircuitVersion,
		&result.Probe, &result.Status, &result.Result, &result.StableDigest,
		&result.ElapsedMS, &result.RouteNo,
	)
	return result, err
}

func sameAttemptRoute(left attemptRecord, right attemptRecord) bool {
	return left.AttemptNo == right.AttemptNo && left.RequestNo == right.RequestNo &&
		left.InstallationID == right.InstallationID && left.Provider == right.Provider &&
		left.OperationScope == right.OperationScope &&
		left.CircuitVersion == right.CircuitVersion && left.Probe == right.Probe &&
		left.RouteNo == right.RouteNo
}

func businessStatusFor(result ResultClass) string {
	switch result {
	case ResultSucceeded:
		return "succeeded"
	case ResultUnknown:
		return "result_unknown"
	default:
		return "failed"
	}
}

func healthResultFor(result ResultClass) string {
	switch result {
	case ResultAvailabilityFailed:
		return string(ResultAvailabilityFailed)
	case ResultUnknown:
		return string(ResultUnknown)
	default:
		return string(ResultSucceeded)
	}
}

func pruneSamples(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
	operationScope string,
	maximum int,
) error {
	_, err := transaction.ExecContext(ctx, `
DELETE FROM idc_gateway_supplier_health_sample
WHERE sample_no IN (
    SELECT sample_no
    FROM idc_gateway_supplier_health_sample
    WHERE provider_code = $1 AND operation_scope = $2
    ORDER BY sample_no DESC
    OFFSET $3
)`, provider, operationScope, maximum)
	if err != nil {
		return ErrRejected
	}
	return nil
}

func loadSampleCounts(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
	operationScope string,
	circuitVersion int64,
	maximum int,
) (sampleCounts, error) {
	rows, err := transaction.QueryContext(ctx, `
SELECT result_class
FROM idc_gateway_supplier_health_sample
WHERE provider_code = $1 AND operation_scope = $2 AND circuit_version = $3
ORDER BY sample_no DESC
LIMIT $4`, provider, operationScope, circuitVersion, maximum)
	if err != nil {
		return sampleCounts{}, ErrRejected
	}
	defer rows.Close()
	result := sampleCounts{}
	countingConsecutive := true
	for rows.Next() {
		var class string
		if scanErr := rows.Scan(&class); scanErr != nil {
			return sampleCounts{}, ErrRejected
		}
		result.Total++
		switch class {
		case string(ResultAvailabilityFailed):
			result.Failures++
			if countingConsecutive {
				result.ConsecutiveFailures++
			}
		case string(ResultUnknown):
			result.Unknowns++
			countingConsecutive = false
		case string(ResultSucceeded):
			countingConsecutive = false
		default:
			return sampleCounts{}, ErrRejected
		}
	}
	if err = rows.Err(); err != nil {
		return sampleCounts{}, ErrRejected
	}
	return result, nil
}

func (s *Service) advanceCircuit(
	ctx context.Context,
	transaction *sql.Tx,
	databaseNow time.Time,
	circuit circuitRecord,
	attempt attemptRecord,
	resultClass ResultClass,
	counts sampleCounts,
) error {
	if attempt.Probe && circuit.Status == "half_open" &&
		circuit.ProbeAttemptNo.Valid && circuit.ProbeAttemptNo.String == attempt.AttemptNo &&
		circuit.Version == attempt.CircuitVersion {
		if resultClass == ResultSucceeded || resultClass == ResultDefinitiveFailure {
			return updateCircuitState(
				ctx, transaction, circuit, "closed", databaseNow, circuit.Version+1,
				sampleCounts{}, sql.NullTime{},
			)
		}
		return updateCircuitState(
			ctx, transaction, circuit, "open", databaseNow, circuit.Version+1,
			counts, sql.NullTime{Time: databaseNow.Add(s.policy.OpenDuration), Valid: true},
		)
	}
	if attempt.Probe || circuit.Status != "closed" || circuit.Version != attempt.CircuitVersion {
		return nil
	}
	if counts.Total >= s.policy.MinimumSamples &&
		(counts.Failures >= s.policy.FailureThreshold ||
			counts.Unknowns >= s.policy.UnknownThreshold ||
			counts.ConsecutiveFailures >= s.policy.ConsecutiveFailureThreshold) {
		return updateCircuitState(
			ctx, transaction, circuit, "open", databaseNow, circuit.Version+1,
			counts, sql.NullTime{Time: databaseNow.Add(s.policy.OpenDuration), Valid: true},
		)
	}
	return updateCircuitState(
		ctx, transaction, circuit, "closed", databaseNow, circuit.Version,
		counts, sql.NullTime{},
	)
}

func updateCircuitState(
	ctx context.Context,
	transaction *sql.Tx,
	old circuitRecord,
	status string,
	databaseNow time.Time,
	version int64,
	counts sampleCounts,
	openUntil sql.NullTime,
) error {
	result, err := transaction.ExecContext(ctx, `
UPDATE idc_gateway_supplier_circuit
SET status_code = $3, total_samples = $4, failure_samples = $5,
    unknown_samples = $6, consecutive_failures = $7, open_until = $8,
    probe_attempt_no = NULL, probe_expires_at = NULL, updated_at = $9, version = $10
WHERE provider_code = $1 AND operation_scope = $2 AND version = $11`,
		old.Provider, old.OperationScope, status, counts.Total, counts.Failures,
		counts.Unknowns, counts.ConsecutiveFailures, nullableTime(openUntil),
		databaseNow, version, old.Version)
	if err != nil || rowsAffected(result) != 1 {
		return ErrRejected
	}
	return nil
}

func nullableTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC()
}

func leaseFromAttempt(attempt attemptRecord, recovered bool) Lease {
	result := ResultClass("")
	if attempt.Result.Valid {
		result = ResultClass(attempt.Result.String)
	}
	return Lease{
		AttemptNo: attempt.AttemptNo, RouteNo: attempt.RouteNo, Provider: attempt.Provider,
		OperationScope: attempt.OperationScope, Recovered: recovered,
		Probe: attempt.Probe, CircuitVersion: attempt.CircuitVersion,
		Status: attempt.Status, Result: result,
	}
}

func (s *Service) expireAbandonedProbe(
	ctx context.Context,
	provider string,
	operationScope string,
) (retryAfter time.Duration, expired bool, err error) {
	databaseContext, cancel := context.WithTimeout(ctx, databaseOperationTimeout)
	defer cancel()
	var (
		probeAttemptNo string
		probeExpiresAt time.Time
		requestNo      string
	)
	err = s.database.QueryRowContext(databaseContext, `
SELECT c.probe_attempt_no, c.probe_expires_at, a.request_no
FROM idc_gateway_supplier_circuit c
JOIN idc_gateway_local_supplier_write_attempt a
  ON a.attempt_no = c.probe_attempt_no
WHERE c.provider_code = $1 AND c.operation_scope = $2
  AND c.status_code = 'half_open'`, provider, operationScope).Scan(
		&probeAttemptNo, &probeExpiresAt, &requestNo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, ErrRejected
	}
	transaction, err := s.database.BeginTx(databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, false, ErrRejected
	}
	defer finishTransaction(transaction, &err)
	databaseNow, err := transactionTime(databaseContext, transaction)
	if err != nil {
		return 0, false, ErrRejected
	}
	if databaseNow.Before(probeExpiresAt.UTC()) {
		return 0, false, nil
	}
	requestStatus, _, err := lockBusinessRequest(
		databaseContext, transaction, requestNo, s.installationID,
	)
	if err != nil || requestStatus != "accepted" {
		return 0, false, ErrRejected
	}
	circuit, err := s.lockCircuit(databaseContext, transaction, provider, operationScope)
	if err != nil {
		return 0, false, err
	}
	if err = s.validateCircuitPolicy(circuit); err != nil {
		return 0, false, err
	}
	if circuit.Status != "half_open" || !circuit.ProbeAttemptNo.Valid ||
		circuit.ProbeAttemptNo.String != probeAttemptNo ||
		!circuit.ProbeExpiresAt.Valid || databaseNow.Before(circuit.ProbeExpiresAt.Time) {
		if err = transaction.Commit(); err != nil {
			return 0, false, ErrRejected
		}
		return 0, false, nil
	}
	attempt, err := lockAttempt(databaseContext, transaction, probeAttemptNo)
	if err != nil || attempt.Status != "admitted" || !attempt.Probe ||
		attempt.CircuitVersion != circuit.Version {
		return 0, false, ErrRejected
	}
	digest := stableCodeDigest("probe_timeout")
	elapsedMilliseconds := int64(s.policy.ProbeTimeout / time.Millisecond)
	result, err := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_write_attempt
SET status_code = 'observed', result_class = 'result_unknown',
    stable_code_sha256 = $2, elapsed_milliseconds = $3, observed_at = $4
WHERE attempt_no = $1 AND status_code = 'admitted'`,
		probeAttemptNo, digest[:], elapsedMilliseconds, databaseNow)
	if err != nil || rowsAffected(result) != 1 {
		return 0, false, ErrRejected
	}
	result, err = transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_business_request
SET status_code = 'result_unknown'
WHERE request_no = $1 AND installation_id = $2 AND status_code = 'accepted'`,
		requestNo, s.installationID)
	if err != nil || rowsAffected(result) != 1 {
		return 0, false, ErrRejected
	}
	result, err = transaction.ExecContext(databaseContext, `
INSERT INTO idc_gateway_supplier_health_sample
    (attempt_no, provider_code, operation_scope, circuit_version,
     result_class, stable_code_sha256, elapsed_milliseconds, observed_at)
VALUES ($1, $2, $3, $4, 'result_unknown', $5, $6, $7)`,
		attempt.AttemptNo, attempt.Provider, attempt.OperationScope,
		attempt.CircuitVersion, digest[:], elapsedMilliseconds, databaseNow)
	if err != nil || rowsAffected(result) != 1 {
		return 0, false, ErrRejected
	}
	if err = pruneSamples(
		databaseContext, transaction, provider, operationScope, s.policy.SampleWindow,
	); err != nil {
		return 0, false, err
	}
	counts, err := loadSampleCounts(
		databaseContext, transaction, provider, operationScope,
		attempt.CircuitVersion, s.policy.SampleWindow,
	)
	if err != nil {
		return 0, false, err
	}
	if err = updateCircuitState(
		databaseContext, transaction, circuit, "open", databaseNow,
		circuit.Version+1, counts,
		sql.NullTime{Time: databaseNow.Add(s.policy.OpenDuration), Valid: true},
	); err != nil {
		return 0, false, err
	}
	if err = transaction.Commit(); err != nil {
		return 0, false, ErrRejected
	}
	return s.policy.OpenDuration, true, nil
}
