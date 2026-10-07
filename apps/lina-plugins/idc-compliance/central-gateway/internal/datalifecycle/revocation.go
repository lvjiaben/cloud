// This file governs stop-first supplier authorization revocation and explicit reopening.

package datalifecycle

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
)

// SupplierAuthorizationRevocation carries one stop-first local revocation.
type SupplierAuthorizationRevocation struct {
	Provider            string
	BindingNo           string
	EmployeePrincipalNo string
	EvidenceSetSHA256   [sha256.Size]byte
	IdempotencyKey      []byte
}

// SupplierAuthorizationReopening carries one separately authorized supplier reopening.
type SupplierAuthorizationReopening struct {
	Provider            string
	PreviousBindingNo   string
	ReasonCode          string
	Subject             []byte
	EmployeePrincipalNo string
	EvidenceSetSHA256   [sha256.Size]byte
	IdempotencyKey      []byte
}

// RevokeSupplierAuthorization stops new local work before revoking the opaque
// subject. A retry with the same identity idempotency key completes a partial
// stop/revoke sequence without calling a supplier or deleting retained data.
func (s *Service) RevokeSupplierAuthorization(
	ctx context.Context,
	input SupplierAuthorizationRevocation,
) (supplieridentity.Result, error) {
	input.Provider = strings.TrimSpace(input.Provider)
	input.BindingNo = strings.TrimSpace(input.BindingNo)
	input.EmployeePrincipalNo = strings.TrimSpace(input.EmployeePrincipalNo)
	if !s.validContext(ctx) || !validProvider(input.Provider) ||
		!bindingPattern.MatchString(input.BindingNo) || !validEmployee(input.EmployeePrincipalNo) ||
		len(input.IdempotencyKey) != supplieridentity.SecretBytes {
		clear(input.IdempotencyKey)
		return supplieridentity.Result{}, ErrInvalid
	}
	operationDigest := digestFrame(
		"idc-supplier-authorization-revoke-v1", input.Provider,
		input.BindingNo, input.EmployeePrincipalNo,
		hex.EncodeToString(input.EvidenceSetSHA256[:]),
	)
	if err := s.stopSupplierProcessing(ctx, input, operationDigest); err != nil {
		clear(input.IdempotencyKey)
		return supplieridentity.Result{}, err
	}
	result, err := s.supplierIdentity.Revoke(ctx, supplieridentity.RevokeInput{
		BindingNo: input.BindingNo, ReasonCode: supplieridentity.ReasonAuthorizationLost,
		IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil {
		return supplieridentity.Result{}, err
	}
	if err = s.recordSupplierRevoked(ctx, input, operationDigest); err != nil {
		return supplieridentity.Result{}, err
	}
	return result, nil
}

// ReopenSupplierAuthorization creates a new opaque subject and only then resumes
// local supplier processing. Stable retries complete a partially resumed operation.
func (s *Service) ReopenSupplierAuthorization(
	ctx context.Context,
	input SupplierAuthorizationReopening,
) (supplieridentity.Result, error) {
	input.Provider = strings.TrimSpace(input.Provider)
	input.PreviousBindingNo = strings.TrimSpace(input.PreviousBindingNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.EmployeePrincipalNo = strings.TrimSpace(input.EmployeePrincipalNo)
	if !s.validContext(ctx) || !validProvider(input.Provider) ||
		!bindingPattern.MatchString(input.PreviousBindingNo) ||
		input.ReasonCode != supplieridentity.ReasonSupplierReauthorized ||
		!validEmployee(input.EmployeePrincipalNo) || len(input.Subject) == 0 ||
		len(input.IdempotencyKey) != supplieridentity.SecretBytes {
		clear(input.Subject)
		clear(input.IdempotencyKey)
		return supplieridentity.Result{}, ErrInvalid
	}
	result, err := s.supplierIdentity.Reopen(ctx, supplieridentity.ReopenInput{
		PreviousBindingNo: input.PreviousBindingNo, Provider: input.Provider,
		Subject: input.Subject, ReasonCode: input.ReasonCode,
		IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil {
		return supplieridentity.Result{}, err
	}
	operationDigest := digestFrame(
		"idc-supplier-authorization-reopen-v1", input.Provider,
		input.PreviousBindingNo, result.BindingNo, input.EmployeePrincipalNo,
		hex.EncodeToString(input.EvidenceSetSHA256[:]),
	)
	if err = s.resumeSupplierProcessing(ctx, input, result.BindingNo, operationDigest); err != nil {
		return supplieridentity.Result{}, err
	}
	return result, nil
}

func (s *Service) stopSupplierProcessing(
	ctx context.Context,
	input SupplierAuthorizationRevocation,
	operationDigest [sha256.Size]byte,
) (returnError error) {
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ErrUnavailable
	}
	defer finishTransaction(transaction, &returnError)
	var boundProvider string
	if err = transaction.QueryRowContext(ctx, `
SELECT provider_code
FROM idc_gateway_local_supplier_subject
WHERE binding_no = $1 AND installation_id = $2
FOR SHARE`, input.BindingNo, s.installationID).Scan(&boundProvider); err != nil ||
		boundProvider != input.Provider {
		return ErrConflict
	}
	var status string
	var existingDigest []byte
	err = transaction.QueryRowContext(ctx, `
SELECT status_code, operation_sha256
FROM idc_gateway_supplier_processing_state
WHERE provider_code = $1
FOR UPDATE`, input.Provider).Scan(&status, &existingDigest)
	if err != nil {
		return ErrUnavailable
	}
	switch status {
	case "allowed":
		result, updateErr := transaction.ExecContext(ctx, `
UPDATE idc_gateway_supplier_processing_state
SET status_code = 'stopped', operation_sha256 = $2,
    updated_by_employee_no = $3, updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE provider_code = $1 AND status_code = 'allowed'`,
			input.Provider, operationDigest[:], input.EmployeePrincipalNo)
		if updateErr != nil || rowsAffected(result) != 1 {
			return ErrUnavailable
		}
	case "stopped":
		if len(existingDigest) != sha256.Size ||
			!equalDigest(existingDigest, operationDigest[:]) {
			return ErrConflict
		}
	default:
		return ErrUnavailable
	}
	if _, err = transaction.ExecContext(ctx, `
UPDATE idc_gateway_supplier_collection_state
SET status_code = 'disabled', next_collect_at = NULL,
    lease_sha256 = NULL, lease_expires_at = NULL,
    updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE provider_code = $1 AND status_code <> 'disabled'`, input.Provider); err != nil {
		return ErrUnavailable
	}
	if err = insertAuthorizationEvent(
		ctx, transaction, s.random, input.Provider, input.EmployeePrincipalNo,
		input.EvidenceSetSHA256, "processing_stopped", operationDigest,
	); err != nil {
		return err
	}
	if err = transaction.Commit(); err != nil {
		return classifyDatabaseError(err)
	}
	return nil
}

func (s *Service) resumeSupplierProcessing(
	ctx context.Context,
	input SupplierAuthorizationReopening,
	newBindingNo string,
	operationDigest [sha256.Size]byte,
) (returnError error) {
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ErrUnavailable
	}
	defer finishTransaction(transaction, &returnError)
	var previousStatus, newStatus, previousProvider, newProvider string
	err = transaction.QueryRowContext(ctx, `
SELECT previous.status_code, current.status_code,
       previous.provider_code, current.provider_code
FROM idc_gateway_local_supplier_subject previous
JOIN idc_gateway_local_supplier_subject current
  ON current.installation_id = previous.installation_id
WHERE previous.binding_no = $1 AND current.binding_no = $2
  AND previous.installation_id = $3
FOR SHARE`, input.PreviousBindingNo, newBindingNo, s.installationID).Scan(
		&previousStatus, &newStatus, &previousProvider, &newProvider,
	)
	if err != nil || previousStatus != "revoked" || newStatus != "active" ||
		previousProvider != input.Provider || newProvider != input.Provider {
		return ErrConflict
	}
	var processingStatus string
	if err = transaction.QueryRowContext(ctx, `
SELECT status_code
FROM idc_gateway_supplier_processing_state
WHERE provider_code = $1
FOR UPDATE`, input.Provider).Scan(&processingStatus); err != nil {
		return ErrUnavailable
	}
	switch processingStatus {
	case "stopped":
		result, updateErr := transaction.ExecContext(ctx, `
UPDATE idc_gateway_supplier_processing_state
SET status_code = 'allowed', operation_sha256 = NULL,
    updated_by_employee_no = $2, updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE provider_code = $1 AND status_code = 'stopped'`,
			input.Provider, input.EmployeePrincipalNo)
		if updateErr != nil || rowsAffected(result) != 1 {
			return classifyDatabaseError(updateErr)
		}
	case "allowed":
		var replayed bool
		if err = transaction.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM idc_gateway_supplier_authorization_event
    WHERE provider_code = $1 AND action_code = 'processing_resumed'
      AND operation_sha256 = $2
)`, input.Provider, operationDigest[:]).Scan(&replayed); err != nil || !replayed {
			return ErrConflict
		}
	default:
		return ErrUnavailable
	}
	if err = insertAuthorizationEvent(
		ctx, transaction, s.random, input.Provider, input.EmployeePrincipalNo,
		input.EvidenceSetSHA256, "processing_resumed", operationDigest,
	); err != nil {
		return err
	}
	if err = transaction.Commit(); err != nil {
		return classifyDatabaseError(err)
	}
	return nil
}

func (s *Service) recordSupplierRevoked(
	ctx context.Context,
	input SupplierAuthorizationRevocation,
	operationDigest [sha256.Size]byte,
) (returnError error) {
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ErrUnavailable
	}
	defer finishTransaction(transaction, &returnError)
	var status string
	var storedDigest []byte
	if err = transaction.QueryRowContext(ctx, `
SELECT status_code, operation_sha256
FROM idc_gateway_supplier_processing_state
WHERE provider_code = $1
FOR SHARE`, input.Provider).Scan(&status, &storedDigest); err != nil ||
		status != "stopped" || !equalDigest(storedDigest, operationDigest[:]) {
		return ErrConflict
	}
	if err = insertAuthorizationEvent(
		ctx, transaction, s.random, input.Provider, input.EmployeePrincipalNo,
		input.EvidenceSetSHA256, "revoked", operationDigest,
	); err != nil {
		return err
	}
	if err = transaction.Commit(); err != nil {
		return classifyDatabaseError(err)
	}
	return nil
}

func insertAuthorizationEvent(
	ctx context.Context,
	transaction *sql.Tx,
	randomSource interface{ Read([]byte) (int, error) },
	provider string,
	employeePrincipalNo string,
	evidenceSetSHA256 [sha256.Size]byte,
	action string,
	operationDigest [sha256.Size]byte,
) error {
	var exists bool
	if err := transaction.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM idc_gateway_supplier_authorization_event
    WHERE provider_code = $1 AND action_code = $2 AND operation_sha256 = $3
)`, provider, action, operationDigest[:]).Scan(&exists); err != nil {
		return ErrUnavailable
	}
	if exists {
		return nil
	}
	eventNo, err := randomIdentifier(randomSource, "lae_")
	if err != nil {
		return ErrUnavailable
	}
	_, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_authorization_event
    (authorization_event_no, provider_code, action_code,
     employee_principal_no, evidence_set_sha256, occurred_at, operation_sha256)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, $6)`,
		eventNo, provider, action, employeePrincipalNo,
		evidenceSetSHA256[:], operationDigest[:])
	if err != nil {
		return classifyDatabaseError(err)
	}
	return nil
}

func equalDigest(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare(left, right) == 1
}
