// This file persists restore write blocks and verifies restored local evidence.

package datalifecycle

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/migration"
	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
)

// RestoreVerificationInput binds one isolated restore verification.
type RestoreVerificationInput struct {
	BackupManifestSHA256  [sha256.Size]byte
	EmployeePrincipalNo   string
	SupplierWritesBlocked bool
}

// RestoreVerificationResult reports the persisted restore checks.
type RestoreVerificationResult struct {
	VerificationNo            string
	DatabaseVerified          bool
	KeyringVerified           bool
	EvidenceIntegrityVerified bool
	SupplierWritesBlocked     bool
	VerifiedAt                time.Time
}

// VerifyRestore performs read-only database, keyring, subject, and evidence
// checks after an operator has restored into an isolated write-blocked environment.
func (s *Service) VerifyRestore(
	ctx context.Context,
	input RestoreVerificationInput,
) (RestoreVerificationResult, error) {
	input.EmployeePrincipalNo = strings.TrimSpace(input.EmployeePrincipalNo)
	if !s.validContext(ctx) || !validEmployee(input.EmployeePrincipalNo) ||
		!input.SupplierWritesBlocked || !s.supplierWritesAreStopped(ctx) {
		return RestoreVerificationResult{}, ErrInvalid
	}
	databaseVerified := migration.Verify(ctx, s.database) == nil && s.verifyDatabaseShape(ctx)
	keyringVerified := s.verifyKeyrings()
	evidenceVerified := false
	if keyringVerified {
		evidenceVerified = s.verifySubjects(ctx) && s.verifyCallbackEvidence(ctx)
	}
	verificationNo, err := randomIdentifier(s.random, "lrv_")
	if err != nil {
		return RestoreVerificationResult{}, ErrUnavailable
	}
	var verifiedAt time.Time
	result, err := s.database.ExecContext(ctx, `
INSERT INTO idc_gateway_restore_verification
    (restore_verification_no, backup_manifest_sha256, database_verified,
     keyring_verified, evidence_integrity_verified, supplier_writes_blocked,
     verified_by_employee_no, verified_at)
VALUES ($1, $2, $3, $4, $5, TRUE, $6, CURRENT_TIMESTAMP)`,
		verificationNo, input.BackupManifestSHA256[:], databaseVerified,
		keyringVerified, evidenceVerified, input.EmployeePrincipalNo)
	if err != nil || rowsAffected(result) != 1 {
		return RestoreVerificationResult{}, classifyDatabaseError(err)
	}
	if err = s.database.QueryRowContext(ctx, `
SELECT verified_at FROM idc_gateway_restore_verification
WHERE restore_verification_no = $1`, verificationNo).Scan(&verifiedAt); err != nil {
		return RestoreVerificationResult{}, ErrUnavailable
	}
	verification := RestoreVerificationResult{
		VerificationNo: verificationNo, DatabaseVerified: databaseVerified,
		KeyringVerified: keyringVerified, EvidenceIntegrityVerified: evidenceVerified,
		SupplierWritesBlocked: true, VerifiedAt: verifiedAt.UTC(),
	}
	if !databaseVerified || !keyringVerified || !evidenceVerified {
		return verification, ErrConflict
	}
	return verification, nil
}

func (s *Service) verifyDatabaseShape(ctx context.Context) bool {
	var instanceCount, policyCount, processingCount int
	err := s.database.QueryRowContext(ctx, `
SELECT
    (SELECT COUNT(*) FROM idc_gateway_local_supplier_instance
      WHERE installation_id = $1),
    (SELECT COUNT(*) FROM idc_gateway_data_lifecycle_policy),
    (SELECT COUNT(*) FROM idc_gateway_supplier_processing_state
      WHERE provider_code IN ('tencent_essbasic', 'nuonuo_invoice'))`,
		s.installationID,
	).Scan(&instanceCount, &policyCount, &processingCount)
	return err == nil && instanceCount == 1 && policyCount == 7 && processingCount == 2
}

// BlockSupplierProcessingForRestore persists a database-authoritative stop for
// both providers before an isolated recovery verification. It does not revoke
// subjects, destroy keys, or enable any supplier adapter.
func (s *Service) BlockSupplierProcessingForRestore(
	ctx context.Context,
	employeePrincipalNo string,
	backupManifestSHA256 [sha256.Size]byte,
) (returnError error) {
	employeePrincipalNo = strings.TrimSpace(employeePrincipalNo)
	if !s.validContext(ctx) || !validEmployee(employeePrincipalNo) {
		return ErrInvalid
	}
	transaction, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer finishTransaction(transaction, &returnError)
	for _, provider := range []string{ProviderTencentESSBasic, ProviderNuonuoInvoice} {
		operationDigest := digestFrame(
			"idc-restore-write-block-v1", provider,
			fmt.Sprintf("%x", backupManifestSHA256[:]),
		)
		var status string
		if err = transaction.QueryRowContext(ctx, `
SELECT status_code FROM idc_gateway_supplier_processing_state
WHERE provider_code = $1 FOR UPDATE`, provider).Scan(&status); err != nil {
			return ErrUnavailable
		}
		if status != "stopped" {
			result, updateErr := transaction.ExecContext(ctx, `
UPDATE idc_gateway_supplier_processing_state
SET status_code = 'stopped', operation_sha256 = $2,
    updated_by_employee_no = $3, updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE provider_code = $1 AND status_code = 'allowed'`,
				provider, operationDigest[:], employeePrincipalNo)
			if updateErr != nil || rowsAffected(result) != 1 {
				return classifyDatabaseError(updateErr)
			}
		}
		if _, err = transaction.ExecContext(ctx, `
UPDATE idc_gateway_supplier_collection_state
SET status_code = 'disabled', next_collect_at = NULL,
    lease_sha256 = NULL, lease_expires_at = NULL,
    updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE provider_code = $1 AND status_code <> 'disabled'`, provider); err != nil {
			return ErrUnavailable
		}
		if status != "stopped" {
			if err = insertAuthorizationEvent(
				ctx, transaction, s.random, provider, employeePrincipalNo,
				backupManifestSHA256, "processing_stopped", operationDigest,
			); err != nil {
				return err
			}
		}
	}
	if err = transaction.Commit(); err != nil {
		return classifyDatabaseError(err)
	}
	return nil
}

func (s *Service) supplierWritesAreStopped(ctx context.Context) bool {
	var stopped int
	err := s.database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_supplier_processing_state
WHERE provider_code IN ('tencent_essbasic', 'nuonuo_invoice')
  AND status_code = 'stopped'`).Scan(&stopped)
	return err == nil && stopped == 2
}

func (s *Service) verifyKeyrings() bool {
	return s.supplierKeyring != nil && s.evidenceKeyring != nil &&
		s.supplierKeyring.ActiveVersion(supplieridentity.PurposeSubjectEncryption) > 0 &&
		s.supplierKeyring.ActiveVersion(supplieridentity.PurposeSubjectLookup) > 0 &&
		s.supplierKeyring.ActiveVersion(supplieridentity.PurposeIdentityOperation) > 0 &&
		s.evidenceKeyring.ActiveEvidenceVersion() > 0 &&
		s.evidenceKeyring.ActiveEventVersion() > 0 &&
		s.evidenceKeyring.ActiveReferenceVersion() > 0 &&
		s.evidenceKeyring.ActiveReferenceLookupVersion() > 0 &&
		s.evidenceKeyring.ActiveTakeoverOperationVersion() > 0
}

type storedLookup struct {
	digest []byte
	status string
}

func (s *Service) verifySubjects(ctx context.Context) (verified bool) {
	lookups, err := s.readSubjectLookups(ctx)
	if err != nil {
		return false
	}
	rows, err := s.database.QueryContext(ctx, `
SELECT binding_no, provider_code, subject_ciphertext, subject_nonce,
       cipher_format_version, encryption_key_version, status_code
FROM idc_gateway_local_supplier_subject
WHERE installation_id = $1
ORDER BY binding_no`, s.installationID)
	if err != nil {
		return false
	}
	verified = true
	defer func() {
		if closeError := rows.Close(); closeError != nil {
			verified = false
		}
	}()
	for rows.Next() {
		var bindingNo, provider, status string
		var sealed supplieridentity.Ciphertext
		if err = rows.Scan(
			&bindingNo, &provider, &sealed.Payload, &sealed.Nonce,
			&sealed.FormatVersion, &sealed.KeyVersion, &status,
		); err != nil {
			return false
		}
		plaintext, openErr := s.supplierKeyring.Open(
			s.installationID, bindingNo, provider, sealed,
		)
		clear(sealed.Payload)
		clear(sealed.Nonce)
		if openErr != nil {
			return false
		}
		digests, digestErr := s.supplierKeyring.LookupDigests(provider, plaintext)
		clear(plaintext)
		if digestErr != nil || len(digests) == 0 {
			return false
		}
		for _, digest := range digests {
			key := fmt.Sprintf("%s/%d", bindingNo, digest.KeyVersion)
			stored, found := lookups[key]
			if !found || !equalDigest(stored.digest, digest.Sum[:]) || stored.status != status {
				return false
			}
			delete(lookups, key)
		}
	}
	return rows.Err() == nil && len(lookups) == 0
}

func (s *Service) readSubjectLookups(
	ctx context.Context,
) (result map[string]storedLookup, returnError error) {
	rows, err := s.database.QueryContext(ctx, `
SELECT lookup.binding_no, lookup.lookup_key_version,
       lookup.subject_lookup_hmac, lookup.status_code
FROM idc_gateway_local_subject_lookup lookup
JOIN idc_gateway_local_supplier_subject subject
  ON subject.binding_no = lookup.binding_no
WHERE subject.installation_id = $1
ORDER BY lookup.binding_no, lookup.lookup_key_version`, s.installationID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeError := rows.Close(); closeError != nil && returnError == nil {
			result = nil
			returnError = closeError
		}
	}()
	result = make(map[string]storedLookup)
	for rows.Next() {
		var bindingNo, status string
		var version uint32
		var digest []byte
		if err = rows.Scan(&bindingNo, &version, &digest, &status); err != nil {
			return nil, err
		}
		result[fmt.Sprintf("%s/%d", bindingNo, version)] = storedLookup{
			digest: append([]byte(nil), digest...), status: status,
		}
	}
	return result, rows.Err()
}

func (s *Service) verifyCallbackEvidence(ctx context.Context) (verified bool) {
	rows, err := s.database.QueryContext(ctx, `
SELECT evidence_no, route_no, provider_code, source_kind,
       payload_ciphertext, payload_nonce, payload_sha256,
       cipher_format_version, encryption_key_version
FROM idc_gateway_local_callback_evidence
WHERE installation_id = $1
ORDER BY evidence_no`, s.installationID)
	if err != nil {
		return false
	}
	verified = true
	defer func() {
		if closeError := rows.Close(); closeError != nil {
			verified = false
		}
	}()
	for rows.Next() {
		var evidenceNo, routeNo, provider, source string
		var digest []byte
		var sealed callbacksecurity.SealedEvidence
		if err = rows.Scan(
			&evidenceNo, &routeNo, &provider, &source,
			&sealed.Ciphertext, &sealed.Nonce, &digest,
			&sealed.FormatVersion, &sealed.KeyVersion,
		); err != nil || len(digest) != sha256.Size {
			return false
		}
		copy(sealed.PlainSHA256[:], digest)
		plaintext, openErr := s.evidenceKeyring.Open(
			evidenceNo, routeNo, provider, source, sealed,
		)
		clear(sealed.Ciphertext)
		clear(sealed.Nonce)
		clear(plaintext)
		if openErr != nil {
			return false
		}
	}
	return rows.Err() == nil
}
