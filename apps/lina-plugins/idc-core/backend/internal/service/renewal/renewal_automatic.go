// This file executes bounded automatic renewal attempts with database-time retry fences.

package renewal

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumAutomaticRenewalBatch = 64

const automaticRenewalActor = "renewal-scheduler"

type automaticTargetRow struct {
	EntitlementNo        string    `orm:"entitlement_no"`
	CustomerNo           string    `orm:"customer_no"`
	CurrencyCode         string    `orm:"currency_code"`
	EntitlementExpiresAt time.Time `orm:"entitlement_expires_at"`
	PolicyNo             string    `orm:"policy_no"`
	PolicyVersion        int64     `orm:"policy_version"`
	RenewalCycles        int       `orm:"renewal_cycles"`
	MaximumAttempts      int       `orm:"maximum_attempts"`
	RetryIntervalSeconds int64     `orm:"retry_interval_seconds"`
	PolicyContentDigest  string    `orm:"policy_content_digest"`
	Due                  bool      `orm:"due"`
}

type automaticAttemptOwnerRow struct {
	OperationNo          string     `orm:"operation_no"`
	EntitlementNo        string     `orm:"entitlement_no"`
	CustomerNo           string     `orm:"customer_no"`
	CurrencyCode         string     `orm:"currency_code"`
	InvoiceNo            string     `orm:"invoice_no"`
	PolicyNo             string     `orm:"policy_no"`
	PolicyVersion        int64      `orm:"policy_version"`
	MaximumAttempts      int        `orm:"maximum_attempts"`
	RetryIntervalSeconds int64      `orm:"retry_interval_seconds"`
	PaymentDueAt         time.Time  `orm:"payment_due_at"`
	CapabilityExpiresAt  *time.Time `orm:"capability_expires_at"`
	LatestAttemptNo      *string    `orm:"latest_attempt_no"`
	LatestSequence       *int       `orm:"latest_sequence"`
	LatestOutcome        *string    `orm:"latest_outcome"`
	LatestLedgerNo       *string    `orm:"latest_ledger_no"`
	LatestFailureCode    *string    `orm:"latest_failure_code"`
	LatestNextAttemptAt  *time.Time `orm:"latest_next_attempt_at"`
}

// AttemptDueService creates or reuses the current system quote and renewal
// invoice, then attempts only the exact same-currency customer balance. A
// balance shortage remains a normal append-only attempt with a future retry.
func (s *serviceImpl) AttemptDueService(
	ctx context.Context,
	entitlementNo string,
) (*AutomaticAttempt, error) {
	if entitlementNo != strings.TrimSpace(entitlementNo) || !validReference(entitlementNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	target, err := loadAutomaticTarget(ctx, s.database, entitlementNo)
	if err != nil {
		return nil, normalizeError(err)
	}
	if target == nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if !target.Due {
		existing, loadErr := loadLatestAutomaticAttempt(
			ctx, s.database, target.EntitlementNo, target.PolicyNo, target.PolicyVersion,
		)
		if loadErr != nil {
			return nil, normalizeError(loadErr)
		}
		if existing == nil ||
			(existing.Outcome != "balance_settled" && existing.Outcome != "zero_settled" &&
				existing.Outcome != "exhausted") {
			return nil, bizerr.NewCode(CodeConflict)
		}
		existing.Reused = true
		return existing, nil
	}
	seed, err := contentDigest(struct {
		EntitlementNo, CustomerNo, CurrencyCode, PolicyNo, PolicyDigest string
		PolicyVersion, ExpiryUnixMicro                                  int64
		RenewalCycles                                                   int
	}{target.EntitlementNo, target.CustomerNo, target.CurrencyCode,
		target.PolicyNo, target.PolicyContentDigest, target.PolicyVersion,
		target.EntitlementExpiresAt.UTC().UnixMicro(), target.RenewalCycles})
	if err != nil {
		return nil, normalizeError(err)
	}
	quote, err := s.quoteOwned(ctx, target.CustomerNo, RequestSourceSystem, QuoteInput{
		EntitlementNo: target.EntitlementNo, RenewalCycles: target.RenewalCycles,
		IdempotencyKey: "autoquote." + seed,
	})
	if err != nil {
		return nil, err
	}
	operation, err := s.confirmOwned(ctx, target.CustomerNo, target.CustomerNo, ConfirmInput{
		QuoteNo: quote.QuoteNo, IdempotencyKey: "autoconfirm." + seed,
	}, "system", automaticRenewalActor)
	if err != nil {
		return nil, err
	}
	if quote.TotalAmountMinor == 0 {
		if operation.Status != "ready" && operation.Status != "succeeded" {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return s.recordAutomaticAttempt(ctx, target, operation, "zero_settled", "", "")
	}
	settlement, err := s.payOwned(
		ctx, target.CustomerNo, target.CustomerNo, PayInput{OperationNo: operation.OperationNo},
		"system", automaticRenewalActor,
	)
	if err != nil {
		if bizerr.Is(err, CodeBalanceInsufficient) {
			return s.recordAutomaticAttempt(
				ctx, target, operation, "insufficient_balance", "", "balance.insufficient",
			)
		}
		if !bizerr.Is(err, CodeUnavailable) {
			return nil, err
		}
		attempt, recordErr := s.recordAutomaticAttempt(
			ctx, target, operation, "retryable_failure", "", "renewal.retryable",
		)
		if recordErr != nil {
			return nil, recordErr
		}
		return attempt, nil
	}
	return s.recordAutomaticAttempt(
		ctx, target, settlement.Operation, "balance_settled",
		settlement.LedgerTransactionNo, "",
	)
}

// RunAutomaticRenewals processes one bounded primary-worker batch. The query
// excludes terminal or not-yet-due attempts for the service's exact current
// expiry and policy revision, while AttemptDueService remains the final
// idempotency and locking boundary.
func (s *serviceImpl) RunAutomaticRenewals(
	ctx context.Context,
	limit int,
) (*AutomaticBatch, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		limit <= 0 || limit > maximumAutomaticRenewalBatch {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	serviceRows := make([]struct {
		EntitlementNo string `orm:"entitlement_no"`
	}, 0, limit)
	if err := s.database.GetScan(ctx, &serviceRows, `
SELECT service.entitlement_no AS entitlement_no
FROM plugin_idc_core_service_entitlement service
INNER JOIN plugin_idc_core_customer_subject customer
    ON customer.tenant_id = service.tenant_id
   AND customer.customer_no = service.customer_no
   AND customer.status_code = 'active'
INNER JOIN plugin_idc_core_service_renewal_policy policy
    ON policy.tenant_id = service.tenant_id
   AND policy.policy_no = service.current_renewal_policy_no
   AND policy.policy_version = service.current_renewal_policy_version
   AND policy.entitlement_no = service.entitlement_no
   AND policy.auto_renew_enabled = TRUE
LEFT JOIN LATERAL (
    SELECT attempt.outcome_code, attempt.next_attempt_at
    FROM plugin_idc_core_service_renewal_attempt attempt
    INNER JOIN plugin_idc_core_business_operation operation
        ON operation.tenant_id = attempt.tenant_id
       AND operation.operation_no = attempt.operation_no
    INNER JOIN plugin_idc_core_service_price_snapshot price
        ON price.tenant_id = attempt.tenant_id
       AND price.price_snapshot_no = operation.price_snapshot_no
       AND price.subject_no = service.entitlement_no
       AND price.original_expires_at = service.expires_at
    WHERE attempt.tenant_id = service.tenant_id
      AND attempt.entitlement_no = service.entitlement_no
      AND attempt.policy_no = policy.policy_no
      AND attempt.policy_version = policy.policy_version
    ORDER BY attempt.attempt_sequence DESC, attempt.id DESC
    LIMIT 1
) latest ON TRUE
WHERE service.tenant_id = 0
  AND service.status_code IN ('active', 'expired')
  AND service.expires_at IS NOT NULL
  AND service.renewal_mode_code IN ('provider_confirmed','local_entitlement')
  AND service.expires_at <=
      CURRENT_TIMESTAMP + policy.lead_seconds * INTERVAL '1 second'
  AND (latest.outcome_code IS NULL OR (
      latest.outcome_code NOT IN ('balance_settled', 'zero_settled', 'exhausted')
      AND (latest.next_attempt_at IS NULL OR latest.next_attempt_at <= CURRENT_TIMESTAMP)
  ))
ORDER BY service.expires_at ASC, service.id ASC
LIMIT ?`, limit); err != nil {
		return nil, normalizeError(gerror.Wrap(err, "list due automatic renewals"))
	}
	report := &AutomaticBatch{Selected: len(serviceRows)}
	for _, serviceRow := range serviceRows {
		if ctx.Err() != nil {
			return report, normalizeError(ctx.Err())
		}
		attempt, err := s.AttemptDueService(ctx, serviceRow.EntitlementNo)
		if err != nil || attempt == nil {
			report.Failures++
			continue
		}
		report.Processed++
		switch attempt.Outcome {
		case "insufficient_balance":
			report.InsufficientBalance++
		case "balance_settled", "zero_settled":
			report.Settled++
		case "exhausted":
			report.Exhausted++
		}
	}
	if report.Failures != 0 {
		return report, bizerr.NewCode(CodeUnavailable)
	}
	return report, nil
}

func loadAutomaticTarget(
	ctx context.Context,
	database gdb.DB,
	entitlementNo string,
) (*automaticTargetRow, error) {
	var row *automaticTargetRow
	if err := database.GetScan(ctx, &row, `
SELECT service.entitlement_no AS entitlement_no, service.customer_no, service.currency_code,
       service.expires_at AS entitlement_expires_at,
       policy.policy_no, policy.policy_version, policy.renewal_cycles,
       policy.maximum_attempts, policy.retry_interval_seconds,
       policy.content_digest AS policy_content_digest,
       (service.expires_at <=
        CURRENT_TIMESTAMP + policy.lead_seconds * INTERVAL '1 second') AS due
FROM plugin_idc_core_service_entitlement service
INNER JOIN plugin_idc_core_customer_subject customer
    ON customer.tenant_id = service.tenant_id
   AND customer.customer_no = service.customer_no
INNER JOIN plugin_idc_core_service_renewal_policy policy
    ON policy.tenant_id = service.tenant_id
   AND policy.policy_no = service.current_renewal_policy_no
   AND policy.policy_version = service.current_renewal_policy_version
   AND policy.entitlement_no = service.entitlement_no
WHERE service.tenant_id = 0 AND service.entitlement_no = ?
  AND customer.status_code = 'active'
  AND service.status_code IN ('active', 'expired')
  AND service.expires_at IS NOT NULL
  AND service.renewal_mode_code IN ('provider_confirmed','local_entitlement')
  AND policy.auto_renew_enabled = TRUE`, entitlementNo); err != nil {
		return nil, gerror.Wrap(err, "load due automatic renewal target")
	}
	return row, nil
}

func loadLatestAutomaticAttempt(
	ctx context.Context,
	database gdb.DB,
	entitlementNo string,
	policyNo string,
	policyVersion int64,
) (*AutomaticAttempt, error) {
	var row struct {
		AttemptNo           string     `orm:"attempt_no"`
		OperationNo         string     `orm:"operation_no"`
		InvoiceNo           string     `orm:"invoice_no"`
		EntitlementNo       string     `orm:"entitlement_no"`
		CustomerNo          string     `orm:"customer_no"`
		CurrencyCode        string     `orm:"currency_code"`
		PolicyNo            string     `orm:"policy_no"`
		PolicyVersion       int64      `orm:"policy_version"`
		Sequence            int        `orm:"attempt_sequence"`
		Outcome             string     `orm:"outcome_code"`
		LedgerTransactionNo *string    `orm:"ledger_transaction_no"`
		FailureCode         string     `orm:"failure_code"`
		NextAttemptAt       *time.Time `orm:"next_attempt_at"`
	}
	if err := database.GetScan(ctx, &row, `
SELECT attempt.attempt_no, attempt.operation_no, attempt.invoice_no,
       attempt.entitlement_no, attempt.customer_no, attempt.currency_code,
       attempt.policy_no, attempt.policy_version, attempt.attempt_sequence,
       attempt.outcome_code, attempt.ledger_transaction_no,
       attempt.failure_code, attempt.next_attempt_at
FROM plugin_idc_core_service_renewal_attempt attempt
WHERE attempt.tenant_id = 0 AND attempt.entitlement_no = ?
  AND attempt.policy_no = ? AND attempt.policy_version = ?
ORDER BY attempt.created_at DESC, attempt.id DESC
LIMIT 1`, entitlementNo, policyNo, policyVersion); err != nil {
		return nil, gerror.Wrap(err, "load latest automatic renewal attempt")
	}
	if row.AttemptNo == "" {
		return nil, nil
	}
	result := &AutomaticAttempt{
		AttemptNo: row.AttemptNo, OperationNo: row.OperationNo,
		InvoiceNo: row.InvoiceNo, EntitlementNo: row.EntitlementNo,
		CustomerNo: row.CustomerNo, CurrencyCode: row.CurrencyCode,
		PolicyNo: row.PolicyNo, PolicyVersion: row.PolicyVersion,
		Sequence: row.Sequence, Outcome: row.Outcome, FailureCode: row.FailureCode,
		NextAttemptAt: cloneTime(row.NextAttemptAt),
	}
	if row.LedgerTransactionNo != nil {
		result.LedgerTransactionNo = *row.LedgerTransactionNo
	}
	return result, nil
}

func (s *serviceImpl) recordAutomaticAttempt(
	ctx context.Context,
	target *automaticTargetRow,
	operation *Operation,
	requestedOutcome string,
	ledgerTransactionNo string,
	failureCode string,
) (*AutomaticAttempt, error) {
	if target == nil || operation == nil || operation.EntitlementNo != target.EntitlementNo ||
		operation.CurrencyCode != target.CurrencyCode ||
		(requestedOutcome != "balance_settled" && requestedOutcome != "zero_settled" &&
			requestedOutcome != "insufficient_balance" && requestedOutcome != "retryable_failure") {
		return nil, bizerr.NewCode(CodeConflict)
	}
	var result *AutomaticAttempt
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var recordErr error
		result, recordErr = s.recordAutomaticAttemptLocked(
			ctx, target, operation, requestedOutcome, ledgerTransactionNo, failureCode,
		)
		return recordErr
	})
	return result, normalizeError(err)
}

func (s *serviceImpl) recordAutomaticAttemptLocked(
	ctx context.Context,
	target *automaticTargetRow,
	operation *Operation,
	requestedOutcome string,
	ledgerTransactionNo string,
	failureCode string,
) (*AutomaticAttempt, error) {
	owner, err := lockAutomaticAttemptOwner(
		ctx, s.database, operation.OperationNo, target.PolicyNo, target.PolicyVersion,
	)
	if err != nil {
		return nil, err
	}
	if owner == nil || owner.EntitlementNo != target.EntitlementNo ||
		owner.CustomerNo != target.CustomerNo || owner.CurrencyCode != target.CurrencyCode ||
		owner.InvoiceNo != operation.InvoiceNo || owner.MaximumAttempts != target.MaximumAttempts ||
		owner.RetryIntervalSeconds != target.RetryIntervalSeconds ||
		owner.PaymentDueAt.IsZero() || owner.CapabilityExpiresAt == nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	now, err := renewalDatabaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	if owner.LatestAttemptNo != nil {
		if owner.LatestOutcome == nil || owner.LatestSequence == nil {
			return nil, bizerr.NewCode(CodeConflict)
		}
		terminal := *owner.LatestOutcome == "balance_settled" ||
			*owner.LatestOutcome == "zero_settled" || *owner.LatestOutcome == "exhausted"
		waiting := owner.LatestNextAttemptAt != nil && owner.LatestNextAttemptAt.After(now)
		if terminal || waiting {
			return projectAutomaticAttempt(owner, true), nil
		}
	}
	sequence := 1
	if owner.LatestSequence != nil {
		sequence = *owner.LatestSequence + 1
	}
	if sequence > owner.MaximumAttempts {
		return nil, bizerr.NewCode(CodeConflict)
	}
	outcome := requestedOutcome
	var nextAttemptAt *time.Time
	if requestedOutcome == "insufficient_balance" || requestedOutcome == "retryable_failure" {
		next, retryable := nextAutomaticRetry(now, owner, sequence)
		if !retryable {
			outcome = "exhausted"
		} else {
			nextAttemptAt = next
		}
	}
	attemptNo, err := newNumber(s.random, "RAT_")
	if err != nil {
		return nil, err
	}
	evidence, err := contentDigest(struct {
		OperationNo, InvoiceNo, PolicyNo, Outcome, LedgerNo, FailureCode string
		PolicyVersion                                                    int64
		Sequence                                                         int
		NextAttemptAt                                                    *time.Time
	}{owner.OperationNo, owner.InvoiceNo, owner.PolicyNo, outcome,
		ledgerTransactionNo, failureCode, owner.PolicyVersion, sequence, nextAttemptAt})
	if err != nil {
		return nil, err
	}
	content, err := contentDigest(struct {
		AttemptNo, Evidence string
		CreatedAt           time.Time
	}{attemptNo, evidence, now})
	if err != nil {
		return nil, err
	}
	var ledgerNo any
	var ledgerType any
	if outcome == "balance_settled" {
		ledgerNo = ledgerTransactionNo
		ledgerType = "balance_payment"
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_renewal_attempt
    (tenant_id, attempt_no, operation_no, entitlement_no, customer_no,
     currency_code, invoice_no, policy_no, policy_version, attempt_sequence,
     outcome_code, ledger_transaction_no, ledger_transaction_type_code,
     failure_code, next_attempt_at, evidence_digest, content_digest, created_at)
VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, attemptNo,
		owner.OperationNo, owner.EntitlementNo, owner.CustomerNo, owner.CurrencyCode,
		owner.InvoiceNo, owner.PolicyNo, owner.PolicyVersion, sequence, outcome,
		ledgerNo, ledgerType, failureCode, nextAttemptAt, evidence, content, now); err != nil {
		return nil, gerror.Wrap(err, "insert automatic renewal attempt")
	}
	return &AutomaticAttempt{
		AttemptNo: attemptNo, OperationNo: owner.OperationNo, InvoiceNo: owner.InvoiceNo,
		EntitlementNo: owner.EntitlementNo, CustomerNo: owner.CustomerNo,
		CurrencyCode: owner.CurrencyCode, PolicyNo: owner.PolicyNo,
		PolicyVersion: owner.PolicyVersion, Sequence: sequence, Outcome: outcome,
		LedgerTransactionNo: ledgerTransactionNo, FailureCode: failureCode,
		NextAttemptAt: cloneTime(nextAttemptAt),
	}, nil
}

func nextAutomaticRetry(
	now time.Time,
	owner *automaticAttemptOwnerRow,
	sequence int,
) (*time.Time, bool) {
	if owner == nil || owner.CapabilityExpiresAt == nil || sequence >= owner.MaximumAttempts {
		return nil, false
	}
	deadline := owner.PaymentDueAt
	if owner.CapabilityExpiresAt.Before(deadline) {
		deadline = *owner.CapabilityExpiresAt
	}
	next := now.Add(time.Duration(owner.RetryIntervalSeconds) * time.Second)
	if !next.Before(deadline) {
		return nil, false
	}
	return &next, true
}

func lockAutomaticAttemptOwner(
	ctx context.Context,
	database gdb.DB,
	operationNo string,
	policyNo string,
	policyVersion int64,
) (*automaticAttemptOwnerRow, error) {
	var row *automaticAttemptOwnerRow
	if err := database.GetScan(ctx, &row, `
SELECT operation.operation_no, operation.subject_no AS entitlement_no, operation.customer_no,
       operation.currency_code, operation.invoice_no,
       policy.policy_no, policy.policy_version, policy.maximum_attempts,
       policy.retry_interval_seconds, invoice.payment_due_at,
       invoice.payment_due_at AS capability_expires_at
FROM plugin_idc_core_business_operation operation
INNER JOIN plugin_idc_core_service_entitlement service
    ON service.tenant_id = operation.tenant_id
   AND service.entitlement_no = operation.subject_no
   AND service.customer_no = operation.customer_no
INNER JOIN plugin_idc_core_billing_invoice invoice
    ON invoice.tenant_id = operation.tenant_id
   AND invoice.invoice_no = operation.invoice_no
   AND invoice.customer_no = operation.customer_no
   AND invoice.currency_code = operation.currency_code
INNER JOIN plugin_idc_core_service_renewal_policy policy
    ON policy.tenant_id = operation.tenant_id
   AND policy.entitlement_no = operation.subject_no
   AND policy.policy_no = ? AND policy.policy_version = ?
WHERE operation.tenant_id = 0 AND operation.operation_type_code = 'renewal'
  AND operation.operation_no = ?
FOR UPDATE OF operation, policy`, policyNo, policyVersion, operationNo); err != nil {
		return nil, gerror.Wrap(err, "lock automatic renewal attempt owner")
	}
	if row == nil {
		return nil, nil
	}
	// Read the latest attempt in a second statement after acquiring the owner
	// lock. PostgreSQL fixes a statement snapshot before FOR UPDATE waits, so a
	// lateral read in the locking statement could miss the attempt committed by
	// the transaction that previously held this lock.
	var latest *automaticAttemptOwnerRow
	if err := database.GetScan(ctx, &latest, `
SELECT attempt_no AS latest_attempt_no,
       attempt_sequence AS latest_sequence,
       outcome_code AS latest_outcome,
       ledger_transaction_no AS latest_ledger_no,
       failure_code AS latest_failure_code,
       next_attempt_at AS latest_next_attempt_at
FROM plugin_idc_core_service_renewal_attempt
WHERE tenant_id = 0 AND operation_no = ?
ORDER BY attempt_sequence DESC, id DESC
LIMIT 1`, operationNo); err != nil {
		return nil, gerror.Wrap(err, "load locked automatic renewal attempt")
	}
	if latest != nil {
		row.LatestAttemptNo = latest.LatestAttemptNo
		row.LatestSequence = latest.LatestSequence
		row.LatestOutcome = latest.LatestOutcome
		row.LatestLedgerNo = latest.LatestLedgerNo
		row.LatestFailureCode = latest.LatestFailureCode
		row.LatestNextAttemptAt = latest.LatestNextAttemptAt
	}
	return row, nil
}

func projectAutomaticAttempt(row *automaticAttemptOwnerRow, reused bool) *AutomaticAttempt {
	result := &AutomaticAttempt{
		OperationNo: row.OperationNo, InvoiceNo: row.InvoiceNo,
		EntitlementNo: row.EntitlementNo, CustomerNo: row.CustomerNo,
		CurrencyCode: row.CurrencyCode, PolicyNo: row.PolicyNo,
		PolicyVersion: row.PolicyVersion, Reused: reused,
	}
	if row.LatestAttemptNo != nil {
		result.AttemptNo = *row.LatestAttemptNo
	}
	if row.LatestSequence != nil {
		result.Sequence = *row.LatestSequence
	}
	if row.LatestOutcome != nil {
		result.Outcome = *row.LatestOutcome
	}
	if row.LatestLedgerNo != nil {
		result.LedgerTransactionNo = *row.LatestLedgerNo
	}
	if row.LatestFailureCode != nil {
		result.FailureCode = *row.LatestFailureCode
	}
	result.NextAttemptAt = cloneTime(row.LatestNextAttemptAt)
	return result
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
