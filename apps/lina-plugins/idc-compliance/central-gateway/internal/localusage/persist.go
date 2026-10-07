// This file atomically seals and persists one provider-neutral collection,
// including configured budget alerts, exact reconciliation, and local SLA.

package localusage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

type budgetPolicy struct {
	Present  bool
	Warning  int64
	Critical int64
}

func (s *CollectionService) persist(
	ctx context.Context,
	lease collectionLease,
	input Collection,
) (result CollectionResult, returnError error) {
	if s == nil || ctx == nil || ctx.Err() != nil || lease.Provider != input.ProviderCode ||
		len(lease.Token) != sha256.Size || !validCollection(input) {
		return CollectionResult{}, ErrCollectionRejected
	}
	eventDigests, err := s.keyring.EventDigests(input.ProviderCode, input.EventID)
	if err != nil {
		return CollectionResult{}, ErrCollectionRejected
	}
	activeEvent, ok := activeCollectionDigest(s.keyring.ActiveEventVersion(), eventDigests)
	if !ok {
		return CollectionResult{}, ErrCollectionRejected
	}
	planDigests, err := s.keyring.ReferenceDigests(input.ProviderCode, input.Plan.Reference)
	if err != nil {
		return CollectionResult{}, ErrCollectionRejected
	}
	activePlan, ok := activeCollectionDigest(s.keyring.ActiveReferenceLookupVersion(), planDigests)
	if !ok {
		return CollectionResult{}, ErrCollectionRejected
	}
	requestDigest, err := collectionRequestDigest(input)
	if err != nil {
		return CollectionResult{}, ErrCollectionRejected
	}
	operationNo := stableCollectionIdentifier("lop_", activeEvent.Sum[:], "operation")
	evidenceNo := stableCollectionIdentifier("lev_", activeEvent.Sum[:], "evidence")
	sealed, err := s.keyring.Seal(
		s.random, evidenceNo, operationNo, input.ProviderCode, "usage_collection", input.EvidenceFrame,
	)
	if err != nil {
		return CollectionResult{}, ErrCollectionRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	defer clear(sealed.PlainSHA256[:])
	packedEvidence, err := packCollectionEvidence(sealed)
	if err != nil {
		return CollectionResult{}, ErrCollectionRejected
	}
	defer clear(packedEvidence)
	evidenceSetDigest := collectionEvidenceSetDigest(evidenceNo, sealed.PlainSHA256)
	leaseDigest := sha256.Sum256(lease.Token)
	databaseContext, cancel := context.WithTimeout(ctx, collectionDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return CollectionResult{}, ErrCollectionRejected
	}
	defer finishCollectionTransaction(transaction, &returnError)
	databaseNow, err := collectionDatabaseTime(databaseContext, transaction)
	if err != nil || input.ObservedAt.UTC().After(databaseNow.Add(30*time.Second)) {
		return CollectionResult{}, ErrCollectionRejected
	}
	if err = lockCollectionLease(
		databaseContext, transaction, lease.Provider, leaseDigest, databaseNow,
	); err != nil {
		return CollectionResult{}, err
	}
	if replayed, found, replayError := findCollectionReplay(
		databaseContext, transaction, input.ProviderCode, eventDigests,
		requestDigest, sealed.PlainSHA256, evidenceSetDigest,
	); replayError != nil {
		return CollectionResult{}, replayError
	} else if found {
		if err = completeCollectionLease(databaseContext, transaction, lease, leaseDigest, databaseNow); err != nil {
			return CollectionResult{}, err
		}
		if err = transaction.Commit(); err != nil {
			return CollectionResult{}, ErrCollectionRejected
		}
		replayed.Replayed = true
		return replayed, nil
	}
	if err = insertCollectionEvidence(
		databaseContext, transaction, operationNo, evidenceNo, input,
		activeEvent, requestDigest, sealed, packedEvidence,
	); err != nil {
		return CollectionResult{}, err
	}
	planSnapshotNo := stableCollectionIdentifier("lps_", activeEvent.Sum[:], "plan")
	if err = insertPlanSnapshot(
		databaseContext, transaction, planSnapshotNo, evidenceNo,
		input, activePlan.Sum[:],
	); err != nil {
		return CollectionResult{}, err
	}
	for index, item := range input.Usage {
		usageNo := stableCollectionIdentifier(
			"lus_", activeEvent.Sum[:], fmt.Sprintf("usage:%06d", index),
		)
		if err = insertUsageSnapshot(
			databaseContext, transaction, usageNo, evidenceNo, input, item,
		); err != nil {
			return CollectionResult{}, err
		}
	}
	policies, err := loadBudgetPolicies(databaseContext, transaction, input.ProviderCode)
	if err != nil {
		return CollectionResult{}, err
	}
	budgetAlerts := 0
	for index, item := range input.Reconciliations {
		reconciliationNo := stableCollectionIdentifier(
			"lrc_", activeEvent.Sum[:], fmt.Sprintf("reconciliation:%06d", index),
		)
		if err = insertReconciliation(
			databaseContext, transaction, reconciliationNo, input,
			item, evidenceSetDigest,
		); err != nil {
			return CollectionResult{}, err
		}
		created, alertError := insertBudgetAlert(
			databaseContext, transaction, input, item, policies[item.CurrencyCode],
			evidenceSetDigest, activeEvent.Sum[:], index,
		)
		if alertError != nil {
			return CollectionResult{}, alertError
		}
		if created {
			budgetAlerts++
		}
	}
	slaSnapshotNo := stableCollectionIdentifier("lsl_", activeEvent.Sum[:], "sla")
	if err = insertSLASnapshot(
		databaseContext, transaction, slaSnapshotNo, input, evidenceSetDigest,
	); err != nil {
		return CollectionResult{}, err
	}
	if err = completeCollectionLease(
		databaseContext, transaction, lease, leaseDigest, databaseNow,
	); err != nil {
		return CollectionResult{}, err
	}
	if err = transaction.Commit(); err != nil {
		return CollectionResult{}, ErrCollectionRejected
	}
	return CollectionResult{
		ProviderCode: input.ProviderCode, EvidenceNo: evidenceNo,
		PlanSnapshotNo: planSnapshotNo, UsageRows: len(input.Usage),
		BudgetAlerts: budgetAlerts, Reconciliations: len(input.Reconciliations),
		SLASnapshotNo: slaSnapshotNo,
	}, nil
}

func lockCollectionLease(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
	digest [sha256.Size]byte,
	databaseNow time.Time,
) error {
	var storedDigest []byte
	var expiresAt time.Time
	var status string
	err := transaction.QueryRowContext(ctx, `
SELECT status_code, lease_sha256, lease_expires_at
FROM idc_gateway_supplier_collection_state
WHERE provider_code = $1
FOR UPDATE`, provider).Scan(&status, &storedDigest, &expiresAt)
	if err != nil || status != "running" || !bytes.Equal(storedDigest, digest[:]) ||
		expiresAt.UTC().Before(databaseNow) {
		return ErrCollectionLeaseLost
	}
	return nil
}

func findCollectionReplay(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
	digests []callbacksecurity.EventDigest,
	requestDigest [sha256.Size]byte,
	payloadDigest [sha256.Size]byte,
	evidenceSetDigest [sha256.Size]byte,
) (CollectionResult, bool, error) {
	arguments := make([]any, 0, len(digests)+1)
	arguments = append(arguments, provider)
	parameters := make([]string, 0, len(digests))
	for _, digest := range digests {
		arguments = append(arguments, digest.Sum[:])
		parameters = append(parameters, fmt.Sprintf("$%d", len(arguments)))
	}
	rows, err := transaction.QueryContext(ctx, `
SELECT evidence.evidence_no, operation.request_sha256, evidence.payload_sha256
FROM idc_gateway_supplier_evidence evidence
JOIN idc_gateway_supplier_operation operation
  ON operation.operation_no = evidence.operation_no
WHERE operation.provider_code = $1
  AND evidence.event_hmac IN (`+strings.Join(parameters, ", ")+`)
ORDER BY evidence.evidence_no
LIMIT 2
FOR SHARE`, arguments...)
	if err != nil {
		return CollectionResult{}, false, ErrCollectionRejected
	}
	defer rows.Close()
	var evidenceNo string
	var storedRequest []byte
	var storedPayload []byte
	matchCount := 0
	for rows.Next() {
		matchCount++
		if err = rows.Scan(&evidenceNo, &storedRequest, &storedPayload); err != nil {
			return CollectionResult{}, false, ErrCollectionRejected
		}
	}
	if err = rows.Err(); err != nil {
		return CollectionResult{}, false, ErrCollectionRejected
	}
	if matchCount == 0 {
		return CollectionResult{}, false, nil
	}
	if matchCount != 1 || !bytes.Equal(storedRequest, requestDigest[:]) ||
		!bytes.Equal(storedPayload, payloadDigest[:]) {
		return CollectionResult{}, false, errors.Join(ErrCollectionRejected, errCollectionConflict)
	}
	result := CollectionResult{ProviderCode: provider, EvidenceNo: evidenceNo}
	if err = transaction.QueryRowContext(ctx, `
SELECT plan_snapshot_no
FROM idc_gateway_supplier_plan_snapshot
WHERE evidence_no = $1`, evidenceNo).Scan(&result.PlanSnapshotNo); err != nil {
		return CollectionResult{}, false, ErrCollectionRejected
	}
	if err = transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM idc_gateway_supplier_usage_snapshot
WHERE evidence_no = $1`, evidenceNo).Scan(&result.UsageRows); err != nil {
		return CollectionResult{}, false, ErrCollectionRejected
	}
	if err = transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM idc_gateway_supplier_budget_alert
WHERE evidence_set_sha256 = $1`, evidenceSetDigest[:]).Scan(&result.BudgetAlerts); err != nil {
		return CollectionResult{}, false, ErrCollectionRejected
	}
	if err = transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM idc_gateway_supplier_reconciliation
WHERE evidence_set_sha256 = $1`, evidenceSetDigest[:]).Scan(&result.Reconciliations); err != nil {
		return CollectionResult{}, false, ErrCollectionRejected
	}
	if err = transaction.QueryRowContext(ctx, `
SELECT sla_snapshot_no
FROM idc_gateway_supplier_sla_snapshot
WHERE evidence_set_sha256 = $1`, evidenceSetDigest[:]).Scan(&result.SLASnapshotNo); err != nil {
		return CollectionResult{}, false, ErrCollectionRejected
	}
	return result, true, nil
}

func insertCollectionEvidence(
	ctx context.Context,
	transaction *sql.Tx,
	operationNo string,
	evidenceNo string,
	input Collection,
	eventDigest callbacksecurity.EventDigest,
	requestDigest [sha256.Size]byte,
	sealed callbacksecurity.SealedEvidence,
	packedEvidence []byte,
) error {
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_operation
    (operation_no, provider_code, operation_kind, idempotency_hmac,
     request_sha256, status_code, attempt_count, last_stable_code, observed_at)
VALUES ($1, $2, 'usage.collect', $3, $4, 'succeeded', 1, 'collected', $5)`,
		operationNo, input.ProviderCode, eventDigest.Sum[:], requestDigest[:],
		input.ObservedAt.UTC())
	if err != nil || collectionRowsAffected(result) != 1 {
		return ErrCollectionRejected
	}
	result, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_evidence
    (evidence_no, operation_no, evidence_kind, event_hmac, payload_sha256,
     payload_ciphertext, key_version, observed_at)
VALUES ($1, $2, 'usage.collection', $3, $4, $5, $6, $7)`,
		evidenceNo, operationNo, eventDigest.Sum[:], sealed.PlainSHA256[:], packedEvidence,
		fmt.Sprintf("evidence:%d", sealed.KeyVersion), input.ObservedAt.UTC())
	if err != nil || collectionRowsAffected(result) != 1 {
		return ErrCollectionRejected
	}
	return nil
}

func insertPlanSnapshot(
	ctx context.Context,
	transaction *sql.Tx,
	planNo string,
	evidenceNo string,
	input Collection,
	planDigest []byte,
) error {
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_plan_snapshot
    (plan_snapshot_no, provider_code, plan_ref_hmac, status_code,
     currency_code, valid_from, valid_until, evidence_no, observed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		planNo, input.ProviderCode, planDigest, input.Plan.StatusCode,
		nullableCollectionString(input.Plan.Currency), nullableCollectionTime(input.Plan.ValidFrom),
		nullableCollectionTime(input.Plan.ValidUntil), evidenceNo, input.ObservedAt.UTC())
	if err != nil || collectionRowsAffected(result) != 1 {
		return ErrCollectionRejected
	}
	return nil
}

func insertUsageSnapshot(
	ctx context.Context,
	transaction *sql.Tx,
	usageNo string,
	evidenceNo string,
	input Collection,
	item UsageObservation,
) error {
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_usage_snapshot
    (usage_snapshot_no, provider_code, metric_code, period_started_at,
     period_ended_at, quantity_numerator, quantity_denominator,
     supplier_cost_minor, currency_code, evidence_no, observed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		usageNo, input.ProviderCode, item.MetricCode, item.PeriodStartedAt.UTC(),
		item.PeriodEndedAt.UTC(), item.QuantityNumerator, item.QuantityDenominator,
		nullableCollectionInt64(item.SupplierCostMinor), nullableCollectionString(item.CurrencyCode),
		evidenceNo, input.ObservedAt.UTC())
	if err != nil || collectionRowsAffected(result) != 1 {
		return ErrCollectionRejected
	}
	return nil
}

func loadBudgetPolicies(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
) (map[string]budgetPolicy, error) {
	rows, err := transaction.QueryContext(ctx, `
SELECT currency_code, warning_minor, critical_minor
FROM idc_gateway_supplier_budget_policy
WHERE provider_code = $1 AND enabled
ORDER BY currency_code
FOR SHARE`, provider)
	if err != nil {
		return nil, ErrCollectionRejected
	}
	defer rows.Close()
	result := make(map[string]budgetPolicy, MaximumReconciliationRows)
	for rows.Next() {
		var currency string
		var policy budgetPolicy
		if err = rows.Scan(&currency, &policy.Warning, &policy.Critical); err != nil {
			return nil, ErrCollectionRejected
		}
		policy.Present = true
		result[currency] = policy
	}
	if err = rows.Err(); err != nil {
		return nil, ErrCollectionRejected
	}
	return result, nil
}

func insertReconciliation(
	ctx context.Context,
	transaction *sql.Tx,
	reconciliationNo string,
	input Collection,
	item ReconciliationInput,
	evidenceDigest [sha256.Size]byte,
) error {
	difference := item.SupplierCostMinor - item.LocalObservedCostMinor
	status := "mismatch"
	if !item.LocalEvidenceComplete {
		status = "insufficient_evidence"
	} else if difference == 0 {
		status = "exact"
	}
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_reconciliation
    (reconciliation_no, provider_code, period_started_at, period_ended_at,
     currency_code, supplier_cost_minor, local_observed_cost_minor,
     difference_minor, status_code, evidence_set_sha256, observed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		reconciliationNo, input.ProviderCode, item.PeriodStartedAt.UTC(),
		item.PeriodEndedAt.UTC(), item.CurrencyCode, item.SupplierCostMinor,
		item.LocalObservedCostMinor, difference, status, evidenceDigest[:],
		input.ObservedAt.UTC())
	if err != nil || collectionRowsAffected(result) != 1 {
		return ErrCollectionRejected
	}
	return nil
}

func insertBudgetAlert(
	ctx context.Context,
	transaction *sql.Tx,
	input Collection,
	item ReconciliationInput,
	policy budgetPolicy,
	evidenceDigest [sha256.Size]byte,
	seed []byte,
	index int,
) (bool, error) {
	level := ""
	threshold := int64(0)
	if !policy.Present {
		return false, nil
	}
	if item.SupplierCostMinor >= policy.Critical {
		level, threshold = "critical", policy.Critical
	} else if item.SupplierCostMinor >= policy.Warning {
		level, threshold = "warning", policy.Warning
	}
	if level == "" {
		return false, nil
	}
	alertNo := stableCollectionIdentifier(
		"lba_", seed, fmt.Sprintf("budget:%06d:%s", index, level),
	)
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_budget_alert
    (alert_no, provider_code, currency_code, level_code, observed_minor,
     threshold_minor, period_started_at, period_ended_at, evidence_set_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (provider_code, currency_code, level_code,
             period_started_at, period_ended_at) DO NOTHING`,
		alertNo, input.ProviderCode, item.CurrencyCode, level, item.SupplierCostMinor,
		threshold, item.PeriodStartedAt.UTC(), item.PeriodEndedAt.UTC(), evidenceDigest[:])
	if err != nil {
		return false, ErrCollectionRejected
	}
	return collectionRowsAffected(result) == 1, nil
}

func insertSLASnapshot(
	ctx context.Context,
	transaction *sql.Tx,
	slaNo string,
	input Collection,
	evidenceDigest [sha256.Size]byte,
) error {
	availability := 0
	if input.SLA.RequestCount > 0 {
		availability = int(input.SLA.SuccessCount * 1_000_000 / input.SLA.RequestCount)
	}
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_sla_snapshot
    (sla_snapshot_no, provider_code, period_started_at, period_ended_at,
     request_count, success_count, availability_ppm,
     latency_p95_millis, latency_p99_millis, evidence_set_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		slaNo, input.ProviderCode, input.SLA.PeriodStartedAt.UTC(),
		input.SLA.PeriodEndedAt.UTC(), input.SLA.RequestCount, input.SLA.SuccessCount,
		availability, input.SLA.LatencyP95Millis, input.SLA.LatencyP99Millis,
		evidenceDigest[:])
	if err != nil || collectionRowsAffected(result) != 1 {
		return ErrCollectionRejected
	}
	return nil
}

func completeCollectionLease(
	ctx context.Context,
	transaction *sql.Tx,
	lease collectionLease,
	digest [sha256.Size]byte,
	databaseNow time.Time,
) error {
	result, err := transaction.ExecContext(ctx, `
UPDATE idc_gateway_supplier_collection_state
SET status_code = 'ready',
    next_collect_at = $3 + $4::bigint * INTERVAL '1 millisecond',
    last_succeeded_at = $3, failure_count = 0, last_failure_code = NULL,
    lease_sha256 = NULL, lease_expires_at = NULL,
    updated_at = $3, version = version + 1
WHERE provider_code = $1 AND status_code = 'running'
  AND lease_sha256 = $2 AND lease_expires_at >= $3`,
		lease.Provider, digest[:], databaseNow, CollectionPollInterval.Milliseconds())
	if err != nil {
		return ErrCollectionRejected
	}
	if collectionRowsAffected(result) != 1 {
		return ErrCollectionLeaseLost
	}
	return nil
}

func activeCollectionDigest(
	version uint32,
	digests []callbacksecurity.EventDigest,
) (callbacksecurity.EventDigest, bool) {
	for _, digest := range digests {
		if digest.KeyVersion == version {
			return digest, true
		}
	}
	return callbacksecurity.EventDigest{}, false
}

func packCollectionEvidence(sealed callbacksecurity.SealedEvidence) ([]byte, error) {
	if sealed.FormatVersion == 0 || sealed.KeyVersion == 0 || len(sealed.Nonce) == 0 ||
		len(sealed.Nonce) > 255 || len(sealed.Ciphertext) == 0 {
		return nil, ErrCollectionRejected
	}
	result := make([]byte, 3+len(sealed.Nonce)+len(sealed.Ciphertext))
	binary.BigEndian.PutUint16(result[:2], sealed.FormatVersion)
	result[2] = byte(len(sealed.Nonce))
	copy(result[3:], sealed.Nonce)
	copy(result[3+len(sealed.Nonce):], sealed.Ciphertext)
	return result, nil
}

func collectionEvidenceSetDigest(
	evidenceNo string,
	payload [sha256.Size]byte,
) [sha256.Size]byte {
	frame := make([]byte, 0, 96+len(evidenceNo))
	frame = append(frame, "idc-compliance/local-usage-evidence-set/v1\x00"...)
	frame = append(frame, evidenceNo...)
	frame = append(frame, 0)
	frame = append(frame, payload[:]...)
	result := sha256.Sum256(frame)
	clear(frame)
	return result
}

func collectionRequestDigest(input Collection) ([sha256.Size]byte, error) {
	frame := struct {
		SchemaVersion uint16     `json:"schema_version"`
		Collection    Collection `json:"collection"`
	}{SchemaVersion: 1, Collection: input}
	encoded, err := json.Marshal(frame)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer clear(encoded)
	return sha256.Sum256(encoded), nil
}

func stableCollectionIdentifier(prefix string, seed []byte, discriminator string) string {
	frame := make([]byte, 0, 80+len(prefix)+len(seed)+len(discriminator))
	frame = append(frame, "idc-compliance/local-usage-id/v1\x00"...)
	frame = append(frame, prefix...)
	frame = append(frame, 0)
	frame = append(frame, seed...)
	frame = append(frame, 0)
	frame = append(frame, discriminator...)
	digest := sha256.Sum256(frame)
	clear(frame)
	return prefix + hex.EncodeToString(digest[:16])
}

func collectionDatabaseTime(ctx context.Context, transaction *sql.Tx) (time.Time, error) {
	var value time.Time
	if err := transaction.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&value); err != nil {
		return time.Time{}, ErrCollectionRejected
	}
	return value.UTC(), nil
}

func nullableCollectionString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableCollectionTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableCollectionInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
