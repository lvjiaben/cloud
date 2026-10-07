// This file configures local policies and executes bounded retention dispositions.

package datalifecycle

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// LifecyclePolicyInput defines one explicit merchant-approved data-class policy.
type LifecyclePolicyInput struct {
	DataClassCode       string
	RetentionDays       int
	DeletionMode        string
	LegalBasisCode      string
	EmployeePrincipalNo string
}

// RetentionInput bounds one local retention execution.
type RetentionInput struct {
	DataClassCode       string
	EmployeePrincipalNo string
	MaximumRows         int
}

// RetentionResult reports one immutable local disposition.
type RetentionResult struct {
	DispositionNo  string
	DataClassCode  string
	OutcomeCode    string
	DeletedRows    int
	CutoffAt       time.Time
	SelectedSHA256 [sha256.Size]byte
}

type retentionTarget struct {
	table      string
	idColumn   string
	timeColumn string
}

var retentionTargets = map[string][]retentionTarget{
	"supplier_usage": {{
		table: "idc_gateway_supplier_usage_snapshot", idColumn: "usage_snapshot_no",
		timeColumn: "created_at",
	}},
	"supplier_cost": {
		{table: "idc_gateway_supplier_budget_alert", idColumn: "alert_no", timeColumn: "created_at"},
		{table: "idc_gateway_supplier_reconciliation", idColumn: "reconciliation_no", timeColumn: "created_at"},
	},
	"supplier_sla": {{
		table: "idc_gateway_supplier_sla_snapshot", idColumn: "sla_snapshot_no",
		timeColumn: "created_at",
	}},
	"process_nonce": {{
		table: "idc_gateway_process_nonce", idColumn: "encode(nonce_sha256, 'hex')",
		timeColumn: "expires_at",
	}},
}

// ConfigurePolicy writes the current merchant's explicit, versioned class policy.
func (s *Service) ConfigurePolicy(ctx context.Context, input LifecyclePolicyInput) error {
	input.DataClassCode = strings.TrimSpace(input.DataClassCode)
	input.DeletionMode = strings.TrimSpace(input.DeletionMode)
	input.LegalBasisCode = strings.TrimSpace(input.LegalBasisCode)
	input.EmployeePrincipalNo = strings.TrimSpace(input.EmployeePrincipalNo)
	if !s.validContext(ctx) || !validDataClass(input.DataClassCode) ||
		input.RetentionDays < 1 || input.RetentionDays > 36500 ||
		!validDeletionMode(input.DeletionMode) || !stableCodePattern.MatchString(input.LegalBasisCode) ||
		!validEmployee(input.EmployeePrincipalNo) ||
		(isStatutoryClass(input.DataClassCode) && input.DeletionMode != "retain_digest") {
		return ErrInvalid
	}
	result, err := s.database.ExecContext(ctx, `
INSERT INTO idc_gateway_data_lifecycle_policy
    (data_class_code, retention_days, deletion_mode, legal_basis_code,
     enabled, updated_by_employee_no)
VALUES ($1, $2, $3, $4, TRUE, $5)
ON CONFLICT (data_class_code) DO UPDATE
SET retention_days = EXCLUDED.retention_days,
    deletion_mode = EXCLUDED.deletion_mode,
    legal_basis_code = EXCLUDED.legal_basis_code,
    enabled = TRUE,
    updated_by_employee_no = EXCLUDED.updated_by_employee_no,
    updated_at = CURRENT_TIMESTAMP,
    version = idc_gateway_data_lifecycle_policy.version + 1`,
		input.DataClassCode, input.RetentionDays, input.DeletionMode,
		input.LegalBasisCode, input.EmployeePrincipalNo)
	if err != nil || rowsAffected(result) != 1 {
		return classifyDatabaseError(err)
	}
	return nil
}

// RunRetention removes only approved non-statutory projections. Any hold,
// incomplete work, active supplier processing, or crypto-erasure policy fails closed.
func (s *Service) RunRetention(
	ctx context.Context,
	input RetentionInput,
) (retentionResult RetentionResult, returnError error) {
	input.DataClassCode = strings.TrimSpace(input.DataClassCode)
	input.EmployeePrincipalNo = strings.TrimSpace(input.EmployeePrincipalNo)
	if !s.validContext(ctx) || !validDataClass(input.DataClassCode) ||
		!validEmployee(input.EmployeePrincipalNo) || input.MaximumRows < 1 ||
		input.MaximumRows > MaximumRetentionRows {
		return RetentionResult{}, ErrInvalid
	}
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return RetentionResult{}, ErrUnavailable
	}
	defer finishTransaction(transaction, &returnError)
	var retentionDays int
	var deletionMode string
	var enabled bool
	if err = transaction.QueryRowContext(ctx, `
SELECT retention_days, deletion_mode, enabled
FROM idc_gateway_data_lifecycle_policy
WHERE data_class_code = $1
FOR UPDATE`, input.DataClassCode).Scan(&retentionDays, &deletionMode, &enabled); err != nil || !enabled {
		return RetentionResult{}, ErrConflict
	}
	var cutoff time.Time
	if err = transaction.QueryRowContext(ctx, `
SELECT CURRENT_TIMESTAMP - $1::bigint * INTERVAL '1 day'`, retentionDays).Scan(&cutoff); err != nil {
		return RetentionResult{}, ErrUnavailable
	}
	if held, holdErr := hasActiveHold(ctx, transaction, input.DataClassCode); holdErr != nil {
		return RetentionResult{}, holdErr
	} else if held {
		return s.recordRetentionOutcome(
			ctx, transaction, input, deletionMode, "legal_hold", cutoff, nil, ErrLegalHold,
		)
	}
	if isStatutoryClass(input.DataClassCode) {
		return s.recordRetentionOutcome(
			ctx, transaction, input, deletionMode, "statutory_retained", cutoff, nil,
			ErrStatutoryRetention,
		)
	}
	if deletionMode == "crypto_erase" {
		return s.recordRetentionOutcome(
			ctx, transaction, input, deletionMode, "crypto_erase_required", cutoff, nil,
			ErrCryptoEraseRequired,
		)
	}
	if input.DataClassCode != "process_nonce" {
		if incomplete, checkErr := hasIncompleteSupplierWork(ctx, transaction); checkErr != nil {
			return RetentionResult{}, checkErr
		} else if incomplete {
			return s.recordRetentionOutcome(
				ctx, transaction, input, deletionMode, "incomplete_operation", cutoff, nil,
				ErrIncompleteOperation,
			)
		}
		if active, checkErr := hasActiveSupplierProcessing(ctx, transaction); checkErr != nil {
			return RetentionResult{}, checkErr
		} else if active {
			return s.recordRetentionOutcome(
				ctx, transaction, input, deletionMode, "supplier_active", cutoff, nil,
				ErrSupplierActive,
			)
		}
	}
	if _, err = transaction.ExecContext(ctx, `
SELECT set_config('linapro.lifecycle_executor', 'retention-v1', TRUE)`); err != nil {
		return RetentionResult{}, ErrUnavailable
	}
	selected := make([]string, 0, input.MaximumRows)
	remaining := input.MaximumRows
	for _, target := range retentionTargets[input.DataClassCode] {
		if remaining == 0 {
			break
		}
		identifiers, selectErr := selectRetentionIdentifiers(
			ctx, transaction, target, s.installationID, cutoff, remaining,
		)
		if selectErr != nil {
			return RetentionResult{}, selectErr
		}
		if len(identifiers) == 0 {
			continue
		}
		if deleteErr := deleteRetentionIdentifiers(
			ctx, transaction, target, s.installationID, identifiers,
		); deleteErr != nil {
			return RetentionResult{}, deleteErr
		}
		for _, identifier := range identifiers {
			selected = append(selected, target.table+"/"+identifier)
		}
		remaining -= len(identifiers)
	}
	outcome := "deleted"
	if len(selected) == 0 {
		outcome = "no_due_records"
	}
	return s.recordRetentionOutcome(
		ctx, transaction, input, deletionMode, outcome, cutoff, selected, nil,
	)
}

func (s *Service) recordRetentionOutcome(
	ctx context.Context,
	transaction *sql.Tx,
	input RetentionInput,
	deletionMode string,
	outcome string,
	cutoff time.Time,
	selected []string,
	resultError error,
) (RetentionResult, error) {
	sort.Strings(selected)
	selectedDigest := digestFrame(
		"idc-retention-selection-v1", input.DataClassCode, outcome,
		strings.Join(selected, "\n"),
	)
	dispositionNo, err := randomIdentifier(s.random, "lrd_")
	if err != nil {
		return RetentionResult{}, ErrUnavailable
	}
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_retention_disposition
    (disposition_no, data_class_code, deletion_mode, outcome_code,
     selected_rows, selected_set_sha256, cutoff_at, employee_principal_no)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		dispositionNo, input.DataClassCode, deletionMode, outcome, len(selected),
		selectedDigest[:], cutoff.UTC(), input.EmployeePrincipalNo)
	if err != nil || rowsAffected(result) != 1 {
		return RetentionResult{}, classifyDatabaseError(err)
	}
	if err = transaction.Commit(); err != nil {
		return RetentionResult{}, classifyDatabaseError(err)
	}
	return RetentionResult{
		DispositionNo: dispositionNo, DataClassCode: input.DataClassCode,
		OutcomeCode: outcome, DeletedRows: len(selected), CutoffAt: cutoff.UTC(),
		SelectedSHA256: selectedDigest,
	}, resultError
}

func hasActiveHold(ctx context.Context, transaction *sql.Tx, dataClass string) (bool, error) {
	var held bool
	err := transaction.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM idc_gateway_legal_hold_event placed
    WHERE placed.data_class_code = $1 AND placed.action_code = 'placed'
      AND NOT EXISTS (
          SELECT 1 FROM idc_gateway_legal_hold_event released
          WHERE released.hold_no = placed.hold_no
            AND released.action_code = 'released'
      )
)`, dataClass).Scan(&held)
	if err != nil {
		return false, ErrUnavailable
	}
	return held, nil
}

func hasIncompleteSupplierWork(ctx context.Context, transaction *sql.Tx) (bool, error) {
	var incomplete bool
	err := transaction.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM idc_gateway_local_supplier_route
    WHERE projected_status IN ('running', 'result_unknown')
       OR reconciliation_status IN ('query_required', 'reconciliation_required')
)`).Scan(&incomplete)
	if err != nil {
		return false, ErrUnavailable
	}
	return incomplete, nil
}

func hasActiveSupplierProcessing(ctx context.Context, transaction *sql.Tx) (bool, error) {
	var active bool
	err := transaction.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM idc_gateway_supplier_processing_state
    WHERE status_code <> 'stopped'
)`).Scan(&active)
	if err != nil {
		return false, ErrUnavailable
	}
	return active, nil
}

func selectRetentionIdentifiers(
	ctx context.Context,
	transaction *sql.Tx,
	target retentionTarget,
	installationID string,
	cutoff time.Time,
	limit int,
) (result []string, returnError error) {
	query := fmt.Sprintf(`
SELECT %s FROM %s
WHERE %s < $1`, target.idColumn, target.table, target.timeColumn)
	arguments := []any{cutoff.UTC(), limit}
	if target.table == "idc_gateway_process_nonce" {
		query += " AND installation_id = $3"
		arguments = append(arguments, installationID)
	}
	query += fmt.Sprintf(" ORDER BY %s, %s LIMIT $2 FOR UPDATE SKIP LOCKED", target.timeColumn, target.idColumn)
	rows, err := transaction.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer func() {
		if closeError := rows.Close(); closeError != nil && returnError == nil {
			result = nil
			returnError = ErrUnavailable
		}
	}()
	result = make([]string, 0, limit)
	for rows.Next() {
		var identifier string
		if err = rows.Scan(&identifier); err != nil {
			return nil, ErrUnavailable
		}
		result = append(result, identifier)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrUnavailable
	}
	return result, nil
}

func deleteRetentionIdentifiers(
	ctx context.Context,
	transaction *sql.Tx,
	target retentionTarget,
	installationID string,
	identifiers []string,
) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ANY($1)", target.table, target.idColumn)
	arguments := []any{identifiers}
	if target.table == "idc_gateway_process_nonce" {
		query += " AND installation_id = $2"
		arguments = append(arguments, installationID)
	}
	result, err := transaction.ExecContext(ctx, query, arguments...)
	if err != nil || rowsAffected(result) != int64(len(identifiers)) {
		return ErrUnavailable
	}
	return nil
}

func validDeletionMode(value string) bool {
	return value == "delete" || value == "crypto_erase" || value == "retain_digest"
}

func isStatutoryClass(value string) bool {
	return value == "supplier_callback" || value == "supplier_evidence" || value == "audit"
}
