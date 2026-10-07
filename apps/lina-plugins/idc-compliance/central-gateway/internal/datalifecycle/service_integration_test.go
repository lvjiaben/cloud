// This file verifies the complete local lifecycle against isolated PostgreSQL schemas.

package datalifecycle

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/migration"
	"lina-platform/idc-compliance-gateway/internal/supplierguard"
	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
)

const lifecycleIntegrationDSN = "LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN"

type exactCanonicalizer struct{}

func (exactCanonicalizer) Canonicalize(_ context.Context, value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

func TestLocalLifecyclePostgreSQLExecutionAndRestoreDrill(t *testing.T) {
	ctx := context.Background()
	database := lifecycleTestDatabase(t, ctx)
	installationID := "linst_" + strings.Repeat("1", 64)
	supplierFile, err := supplieridentity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate supplier keyring: %v", err)
	}
	supplierRing, err := supplieridentity.New(supplierFile)
	if err != nil {
		t.Fatalf("open supplier keyring: %v", err)
	}
	t.Cleanup(supplierRing.Clear)
	evidenceFile, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate evidence keyring: %v", err)
	}
	evidenceRing, err := callbacksecurity.New(evidenceFile)
	if err != nil {
		t.Fatalf("open evidence keyring: %v", err)
	}
	t.Cleanup(evidenceRing.Clear)
	identity, err := supplieridentity.NewService(
		database, installationID, supplierRing,
		map[string]supplieridentity.Canonicalizer{ProviderTencentESSBasic: exactCanonicalizer{}},
	)
	if err != nil {
		t.Fatalf("new supplier identity service: %v", err)
	}
	if err = identity.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare supplier identity: %v", err)
	}
	service, err := NewService(database, installationID, identity, supplierRing, evidenceRing)
	if err != nil {
		t.Fatalf("new lifecycle service: %v", err)
	}
	if err = service.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare lifecycle service: %v", err)
	}
	configureLifecyclePolicies(t, ctx, service)

	bindKey := bytes.Repeat([]byte{0x31}, supplieridentity.SecretBytes)
	bound, err := identity.Bind(ctx, supplieridentity.BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("merchant-subject"),
		IdempotencyKey: bindKey,
	})
	if err != nil {
		t.Fatalf("bind supplier subject: %v", err)
	}
	if !allZero(bindKey) {
		t.Fatal("supplier bind idempotency key was not cleared")
	}
	wrongProviderKey := bytes.Repeat([]byte{0x40}, supplieridentity.SecretBytes)
	if _, err = service.RevokeSupplierAuthorization(ctx, SupplierAuthorizationRevocation{
		Provider: ProviderNuonuoInvoice, BindingNo: bound.BindingNo,
		EmployeePrincipalNo: "staff:42",
		EvidenceSetSHA256:   sha256.Sum256([]byte("wrong-provider")),
		IdempotencyKey:      wrongProviderKey,
	}); !errors.Is(err, ErrConflict) || !allZero(wrongProviderKey) {
		t.Fatalf("cross-provider revocation err=%v keyCleared=%v", err, allZero(wrongProviderKey))
	}
	revocationEvidence := sha256.Sum256([]byte("reviewed-local-revocation-evidence"))
	revokeKey := bytes.Repeat([]byte{0x42}, supplieridentity.SecretBytes)
	revoked, err := service.RevokeSupplierAuthorization(ctx, SupplierAuthorizationRevocation{
		Provider: ProviderTencentESSBasic, BindingNo: bound.BindingNo,
		EmployeePrincipalNo: "staff:42", EvidenceSetSHA256: revocationEvidence,
		IdempotencyKey: revokeKey,
	})
	if err != nil || revoked.BindingNo != bound.BindingNo || !allZero(revokeKey) {
		t.Fatalf("revoke supplier result=%+v keyCleared=%v err=%v", revoked, allZero(revokeKey), err)
	}
	replayed, err := service.RevokeSupplierAuthorization(ctx, SupplierAuthorizationRevocation{
		Provider: ProviderTencentESSBasic, BindingNo: bound.BindingNo,
		EmployeePrincipalNo: "staff:42", EvidenceSetSHA256: revocationEvidence,
		IdempotencyKey: bytes.Repeat([]byte{0x42}, supplieridentity.SecretBytes),
	})
	if err != nil || !replayed.Replayed || replayed.OperationNo != revoked.OperationNo {
		t.Fatalf("replay supplier revocation=%+v err=%v", replayed, err)
	}
	assertSupplierStopped(t, ctx, database, ProviderTencentESSBasic, bound.BindingNo)
	guard, err := supplierguard.NewService(database, installationID, supplierguard.DefaultPolicy())
	if err != nil {
		t.Fatalf("new supplier guard: %v", err)
	}
	if _, err = guard.Acquire(ctx, supplierguard.Admission{
		RequestNo: "lbr_" + strings.Repeat("9", 32), Provider: ProviderTencentESSBasic,
		OperationScope: "contract.create",
	}); !errors.Is(err, supplierguard.ErrRejected) {
		t.Fatalf("stopped supplier write was not rejected: %v", err)
	}

	evidenceNo, routeNo, evidencePlaintext := seedLifecycleEvidence(
		t, ctx, database, installationID, evidenceRing,
	)
	archiveKey := bytes.Repeat([]byte{0x63}, 32)
	decryptKey := append([]byte(nil), archiveKey...)
	exportResult, err := service.ExportEvidence(ctx, EvidenceExportInput{
		Provider: ProviderTencentESSBasic, RouteNo: routeNo,
		EvidenceNos: []string{evidenceNo}, EmployeePrincipalNo: "staff:42",
		ArchiveKey: archiveKey, ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil || exportResult.EvidenceCount != 1 || !allZero(archiveKey) {
		t.Fatalf("export result=%+v keyCleared=%v err=%v", exportResult, allZero(archiveKey), err)
	}
	assertEncryptedExport(t, exportResult, decryptKey, evidencePlaintext)
	clear(decryptKey)
	failedExportKey := bytes.Repeat([]byte{0x64}, 32)
	if _, err = service.ExportEvidence(ctx, EvidenceExportInput{
		Provider:            ProviderTencentESSBasic,
		EvidenceNos:         []string{"lev_" + strings.Repeat("f", 32)},
		EmployeePrincipalNo: "staff:42", ArchiveKey: failedExportKey,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); !errors.Is(err, ErrConflict) || !allZero(failedExportKey) {
		t.Fatalf("missing evidence export err=%v keyCleared=%v", err, allZero(failedExportKey))
	}
	var failedExports int
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_evidence_export
WHERE status_code = 'failed'`).Scan(&failedExports); err != nil || failedExports != 1 {
		t.Fatalf("failed export events=%d err=%v", failedExports, err)
	}

	usageEvidenceNo := seedRetentionProjection(t, ctx, database)
	if err = service.ConfigurePolicy(ctx, LifecyclePolicyInput{
		DataClassCode: "supplier_evidence", RetentionDays: 1, DeletionMode: "delete",
		LegalBasisCode: "merchant.request", EmployeePrincipalNo: "staff:42",
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("statutory delete policy was accepted: %v", err)
	}
	hold, err := service.PlaceLegalHold(ctx, LegalHoldInput{
		DataClassCode: "supplier_usage", ReasonCode: "active_dispute",
		EmployeePrincipalNo: "staff:42",
	})
	if err != nil {
		t.Fatalf("place legal hold: %v", err)
	}
	holdResult, err := service.RunRetention(ctx, RetentionInput{
		DataClassCode: "supplier_usage", EmployeePrincipalNo: "staff:42", MaximumRows: 10,
	})
	if !errors.Is(err, ErrLegalHold) || holdResult.OutcomeCode != "legal_hold" {
		t.Fatalf("legal hold retention result=%+v err=%v", holdResult, err)
	}
	if _, err = service.ReleaseLegalHold(ctx, hold.HoldNo, LegalHoldInput{
		DataClassCode: "supplier_usage", ReasonCode: "dispute_closed",
		EmployeePrincipalNo: "staff:42",
	}); err != nil {
		t.Fatalf("release legal hold: %v", err)
	}
	restoreManifest := sha256.Sum256([]byte("verified-backup-manifest"))
	if err = service.BlockSupplierProcessingForRestore(ctx, "staff:42", restoreManifest); err != nil {
		t.Fatalf("block supplier processing for restore: %v", err)
	}
	deleted, err := service.RunRetention(ctx, RetentionInput{
		DataClassCode: "supplier_usage", EmployeePrincipalNo: "staff:42", MaximumRows: 10,
	})
	if err != nil || deleted.OutcomeCode != "deleted" || deleted.DeletedRows != 1 {
		t.Fatalf("usage retention result=%+v err=%v", deleted, err)
	}
	var usageRows int
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_supplier_usage_snapshot`).Scan(&usageRows); err != nil || usageRows != 0 {
		t.Fatalf("usage rows=%d err=%v", usageRows, err)
	}
	statutory, err := service.RunRetention(ctx, RetentionInput{
		DataClassCode: "supplier_evidence", EmployeePrincipalNo: "staff:42", MaximumRows: 10,
	})
	if !errors.Is(err, ErrStatutoryRetention) || statutory.OutcomeCode != "statutory_retained" {
		t.Fatalf("statutory retention result=%+v err=%v", statutory, err)
	}
	var evidenceRows int
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_supplier_evidence WHERE evidence_no = $1`,
		usageEvidenceNo).Scan(&evidenceRows); err != nil || evidenceRows != 1 {
		t.Fatalf("statutory evidence rows=%d err=%v", evidenceRows, err)
	}
	if err = service.ConfigurePolicy(ctx, LifecyclePolicyInput{
		DataClassCode: "supplier_sla", RetentionDays: 1, DeletionMode: "crypto_erase",
		LegalBasisCode: "merchant.sla", EmployeePrincipalNo: "staff:42",
	}); err != nil {
		t.Fatalf("configure reviewed crypto erase policy: %v", err)
	}
	cryptoResult, err := service.RunRetention(ctx, RetentionInput{
		DataClassCode: "supplier_sla", EmployeePrincipalNo: "staff:42", MaximumRows: 10,
	})
	if !errors.Is(err, ErrCryptoEraseRequired) || cryptoResult.OutcomeCode != "crypto_erase_required" {
		t.Fatalf("crypto erase result=%+v err=%v", cryptoResult, err)
	}
	seedExpiredNonce(t, ctx, database, installationID)
	nonceRetention, err := service.RunRetention(ctx, RetentionInput{
		DataClassCode: "process_nonce", EmployeePrincipalNo: "staff:42", MaximumRows: 10,
	})
	if err != nil || nonceRetention.DeletedRows != 1 {
		t.Fatalf("nonce retention result=%+v err=%v", nonceRetention, err)
	}

	restore, err := service.VerifyRestore(ctx, RestoreVerificationInput{
		BackupManifestSHA256: restoreManifest, EmployeePrincipalNo: "staff:42",
		SupplierWritesBlocked: true,
	})
	if err != nil || !restore.DatabaseVerified || !restore.KeyringVerified ||
		!restore.EvidenceIntegrityVerified || !restore.SupplierWritesBlocked {
		t.Fatalf("restore verification=%+v err=%v", restore, err)
	}
	reopenSubject := []byte("merchant-subject")
	reopenKey := bytes.Repeat([]byte{0x52}, supplieridentity.SecretBytes)
	reopened, err := service.ReopenSupplierAuthorization(ctx, SupplierAuthorizationReopening{
		Provider: ProviderTencentESSBasic, PreviousBindingNo: bound.BindingNo,
		ReasonCode: supplieridentity.ReasonSupplierReauthorized,
		Subject:    reopenSubject, EmployeePrincipalNo: "staff:42",
		EvidenceSetSHA256: sha256.Sum256([]byte("reviewed-local-reopening-evidence")),
		IdempotencyKey:    reopenKey,
	})
	if err != nil || reopened.BindingNo == bound.BindingNo ||
		!allZero(reopenSubject) || !allZero(reopenKey) {
		t.Fatalf(
			"reopen supplier result=%+v subjectCleared=%v keyCleared=%v err=%v",
			reopened, allZero(reopenSubject), allZero(reopenKey), err,
		)
	}
	replayedReopen, err := service.ReopenSupplierAuthorization(ctx, SupplierAuthorizationReopening{
		Provider: ProviderTencentESSBasic, PreviousBindingNo: bound.BindingNo,
		ReasonCode: supplieridentity.ReasonSupplierReauthorized,
		Subject:    []byte("merchant-subject"), EmployeePrincipalNo: "staff:42",
		EvidenceSetSHA256: sha256.Sum256([]byte("reviewed-local-reopening-evidence")),
		IdempotencyKey:    bytes.Repeat([]byte{0x52}, supplieridentity.SecretBytes),
	})
	if err != nil || !replayedReopen.Replayed || replayedReopen.BindingNo != reopened.BindingNo {
		t.Fatalf("replay supplier reopening=%+v err=%v", replayedReopen, err)
	}
	var processingStatus string
	var resumedEvents int
	if err = database.QueryRowContext(ctx, `
SELECT status_code FROM idc_gateway_supplier_processing_state
WHERE provider_code = $1`, ProviderTencentESSBasic).Scan(&processingStatus); err != nil ||
		processingStatus != "allowed" {
		t.Fatalf("reopened processing status=%q err=%v", processingStatus, err)
	}
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_supplier_authorization_event
WHERE provider_code = $1 AND action_code = 'processing_resumed'`,
		ProviderTencentESSBasic).Scan(&resumedEvents); err != nil || resumedEvents != 1 {
		t.Fatalf("processing resumed events=%d err=%v", resumedEvents, err)
	}
}

func configureLifecyclePolicies(t *testing.T, ctx context.Context, service *Service) {
	t.Helper()
	for _, policy := range []LifecyclePolicyInput{
		{DataClassCode: "supplier_callback", RetentionDays: 3650, DeletionMode: "retain_digest", LegalBasisCode: "statutory.callback", EmployeePrincipalNo: "staff:42"},
		{DataClassCode: "supplier_evidence", RetentionDays: 3650, DeletionMode: "retain_digest", LegalBasisCode: "statutory.evidence", EmployeePrincipalNo: "staff:42"},
		{DataClassCode: "supplier_usage", RetentionDays: 1, DeletionMode: "retain_digest", LegalBasisCode: "merchant.usage", EmployeePrincipalNo: "staff:42"},
		{DataClassCode: "supplier_cost", RetentionDays: 1, DeletionMode: "retain_digest", LegalBasisCode: "merchant.cost", EmployeePrincipalNo: "staff:42"},
		{DataClassCode: "supplier_sla", RetentionDays: 1, DeletionMode: "retain_digest", LegalBasisCode: "merchant.sla", EmployeePrincipalNo: "staff:42"},
		{DataClassCode: "process_nonce", RetentionDays: 1, DeletionMode: "delete", LegalBasisCode: "security.replay", EmployeePrincipalNo: "staff:42"},
		{DataClassCode: "audit", RetentionDays: 3650, DeletionMode: "retain_digest", LegalBasisCode: "statutory.audit", EmployeePrincipalNo: "staff:42"},
	} {
		if err := service.ConfigurePolicy(ctx, policy); err != nil {
			t.Fatalf("configure %s policy: %v", policy.DataClassCode, err)
		}
	}
}

func assertSupplierStopped(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	provider string,
	bindingNo string,
) {
	t.Helper()
	var processingStatus, subjectStatus, reason string
	if err := database.QueryRowContext(ctx, `
SELECT processing.status_code, subject.status_code, subject.revoke_reason_code
FROM idc_gateway_supplier_processing_state processing
JOIN idc_gateway_local_supplier_subject subject ON subject.provider_code = processing.provider_code
WHERE processing.provider_code = $1 AND subject.binding_no = $2`,
		provider, bindingNo).Scan(&processingStatus, &subjectStatus, &reason); err != nil ||
		processingStatus != "stopped" || subjectStatus != "revoked" ||
		reason != supplieridentity.ReasonAuthorizationLost {
		t.Fatalf("processing=%s subject=%s reason=%s err=%v", processingStatus, subjectStatus, reason, err)
	}
	var events int
	if err := database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_supplier_authorization_event
WHERE provider_code = $1 AND action_code IN ('processing_stopped', 'revoked')`, provider).Scan(&events); err != nil || events != 2 {
		t.Fatalf("authorization events=%d err=%v", events, err)
	}
}

func seedLifecycleEvidence(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	installationID string,
	keyring *callbacksecurity.Ring,
) (string, string, []byte) {
	t.Helper()
	requestNo := "lbr_" + strings.Repeat("2", 32)
	attemptNo := "lwa_" + strings.Repeat("3", 32)
	routeNo := "lwr_" + strings.Repeat("4", 32)
	evidenceNo := "lev_" + strings.Repeat("5", 32)
	digest := bytes.Repeat([]byte{0x11}, sha256.Size)
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_business_request
    (request_no, installation_id, operation_scope, method, request_target_sha256,
     idempotency_hmac, business_content_sha256, body_sha256, status_code)
VALUES ($1, $2, 'contract.create', 'POST', $3, $3, $3, $3, 'succeeded')`,
		requestNo, installationID, digest); err != nil {
		t.Fatalf("seed business request: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_write_attempt
    (attempt_no, request_no, installation_id, provider_code, operation_scope,
     circuit_version, status_code, result_class, stable_code_sha256,
     elapsed_milliseconds, observed_at)
VALUES ($1, $2, $3, 'tencent_essbasic', 'contract.create', 1,
        'observed', 'succeeded', $4, 10, CURRENT_TIMESTAMP)`,
		attemptNo, requestNo, installationID, digest); err != nil {
		t.Fatalf("seed supplier attempt: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_route
    (route_no, attempt_no, request_no, installation_id, provider_code,
     operation_scope, projected_status, reconciliation_status)
VALUES ($1, $2, $3, $4, 'tencent_essbasic', 'contract.create', 'succeeded', 'none')`,
		routeNo, attemptNo, requestNo, installationID); err != nil {
		t.Fatalf("seed supplier route: %v", err)
	}
	plaintext := []byte("authorized-evidence-export-payload")
	sealed, err := keyring.Seal(
		rand.Reader, evidenceNo, routeNo, ProviderTencentESSBasic, "callback", plaintext,
	)
	if err != nil {
		t.Fatalf("seal callback evidence: %v", err)
	}
	defer clear(sealed.Ciphertext)
	defer clear(sealed.Nonce)
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_callback_evidence
    (evidence_no, route_no, request_no, installation_id, provider_code,
     operation_scope, source_kind, event_key_version, external_event_hmac,
     payload_ciphertext, payload_nonce, payload_sha256, cipher_format_version,
     encryption_key_version, normalized_status, decision_code, route_version,
     projected_status, reconciliation_status)
VALUES ($1, $2, $3, $4, 'tencent_essbasic', 'contract.create', 'callback',
        1, $5, $6, $7, $8, $9, $10, 'succeeded', 'applied', 1,
        'succeeded', 'none')`,
		evidenceNo, routeNo, requestNo, installationID, digest,
		sealed.Ciphertext, sealed.Nonce, sealed.PlainSHA256[:],
		sealed.FormatVersion, sealed.KeyVersion); err != nil {
		t.Fatalf("seed callback evidence: %v", err)
	}
	return evidenceNo, routeNo, plaintext
}

func assertEncryptedExport(
	t *testing.T,
	result EvidenceExportResult,
	key []byte,
	expectedPlaintext []byte,
) {
	t.Helper()
	if digest := sha256.Sum256(result.Archive); digest != result.ManifestSHA256 {
		t.Fatal("encrypted archive manifest digest mismatch")
	}
	var envelope encryptedEnvelope
	if err := json.Unmarshal(result.Archive, &envelope); err != nil || envelope.Algorithm != "AES-256-GCM" {
		t.Fatalf("decode encrypted envelope: %+v err=%v", envelope, err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		t.Fatalf("decode export nonce: %v", err)
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		t.Fatalf("decode export ciphertext: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("open export key: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("open export AEAD: %v", err)
	}
	plaintext, err := gcm.Open(
		nil, nonce, ciphertext,
		[]byte(envelope.ExportNo+"\x00"+envelope.ScopeSHA256),
	)
	if err != nil {
		t.Fatalf("decrypt evidence archive: %v", err)
	}
	defer clear(plaintext)
	var archive evidenceArchive
	if err = json.Unmarshal(plaintext, &archive); err != nil || len(archive.Evidence) != 1 ||
		!bytes.Equal(archive.Evidence[0].Payload, expectedPlaintext) {
		t.Fatalf("unexpected evidence archive: %+v err=%v", archive, err)
	}
}

func seedRetentionProjection(t *testing.T, ctx context.Context, database *sql.DB) string {
	t.Helper()
	operationNo := "lop_" + strings.Repeat("6", 32)
	evidenceNo := "lse_" + strings.Repeat("7", 32)
	usageNo := "lus_" + strings.Repeat("8", 32)
	digest := bytes.Repeat([]byte{0x72}, sha256.Size)
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_operation
    (operation_no, provider_code, operation_kind, idempotency_hmac,
     request_sha256, status_code)
VALUES ($1, 'tencent_essbasic', 'usage_collect', $2, $2, 'succeeded')`,
		operationNo, digest); err != nil {
		t.Fatalf("seed legacy supplier operation: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_evidence
    (evidence_no, operation_no, evidence_kind, event_hmac, payload_sha256,
     payload_ciphertext, key_version, observed_at)
VALUES ($1, $2, 'usage_snapshot', $3, $3, $4, 'evidence-v1',
        CURRENT_TIMESTAMP - INTERVAL '10 days')`,
		evidenceNo, operationNo, digest, bytes.Repeat([]byte{0x44}, 17)); err != nil {
		t.Fatalf("seed statutory supplier evidence: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_usage_snapshot
    (usage_snapshot_no, provider_code, metric_code, period_started_at,
     period_ended_at, quantity_numerator, quantity_denominator, evidence_no,
     observed_at, created_at)
VALUES ($1, 'tencent_essbasic', 'contract.count',
        CURRENT_TIMESTAMP - INTERVAL '11 days', CURRENT_TIMESTAMP - INTERVAL '10 days',
        1, 1, $2, CURRENT_TIMESTAMP - INTERVAL '10 days',
        CURRENT_TIMESTAMP - INTERVAL '10 days')`, usageNo, evidenceNo); err != nil {
		t.Fatalf("seed usage projection: %v", err)
	}
	return evidenceNo
}

func seedExpiredNonce(t *testing.T, ctx context.Context, database *sql.DB, installationID string) {
	t.Helper()
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_process_nonce
    (installation_id, nonce_sha256, frame_sha256, request_no, key_id,
     expires_at, consumed_at)
VALUES ($1, $2, $3, NULL, 'lgk_1111111111111111',
        CURRENT_TIMESTAMP - INTERVAL '9 days',
        CURRENT_TIMESTAMP - INTERVAL '10 days')`,
		installationID, bytes.Repeat([]byte{0x81}, sha256.Size),
		bytes.Repeat([]byte{0x82}, sha256.Size)); err != nil {
		t.Fatalf("seed expired nonce: %v", err)
	}
}

func lifecycleTestDatabase(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(lifecycleIntegrationDSN))
	if dsn == "" {
		t.Skipf("set %s to run lifecycle PostgreSQL tests", lifecycleIntegrationDSN)
	}
	bootstrap, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL bootstrap: %v", err)
	}
	schema := "idc_lifecycle_test_" + randomTestHex(t, 8)
	if _, err = bootstrap.ExecContext(ctx, `CREATE SCHEMA `+quoteTestIdentifier(schema)); err != nil {
		bootstrap.Close()
		t.Fatalf("create lifecycle schema: %v", err)
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		bootstrap.Close()
		t.Fatalf("parse lifecycle DSN: %v", err)
	}
	if config.RuntimeParams == nil {
		config.RuntimeParams = map[string]string{}
	}
	config.RuntimeParams["search_path"] = schema
	database := stdlib.OpenDB(*config)
	database.SetMaxOpenConns(8)
	database.SetMaxIdleConns(2)
	if err = database.PingContext(ctx); err != nil {
		database.Close()
		bootstrap.Close()
		t.Fatalf("ping lifecycle database: %v", err)
	}
	if err = migration.Apply(ctx, database); err != nil {
		database.Close()
		bootstrap.Close()
		t.Fatalf("apply lifecycle migrations: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close lifecycle database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "idc_lifecycle_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
		} else if _, dropErr := bootstrap.ExecContext(
			context.Background(), `DROP SCHEMA `+quoteTestIdentifier(schema)+` CASCADE`,
		); dropErr != nil {
			t.Errorf("drop lifecycle schema: %v", dropErr)
		}
		if closeErr := bootstrap.Close(); closeErr != nil {
			t.Errorf("close lifecycle bootstrap database: %v", closeErr)
		}
	})
	return database
}

func randomTestHex(t *testing.T, bytesCount int) string {
	t.Helper()
	value := make([]byte, bytesCount)
	if _, err := rand.Read(value); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	return hex.EncodeToString(value)
}

func quoteTestIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func allZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}

func TestLifecycleBoundsAndStatutoryClasses(t *testing.T) {
	values := canonicalEvidenceNos([]string{" lev_" + strings.Repeat("1", 32) + " ", "lev_" + strings.Repeat("1", 32)})
	if len(values) != 1 || !evidencePattern.MatchString(values[0]) {
		t.Fatalf("canonical evidence values=%v", values)
	}
	for _, class := range []string{"supplier_callback", "supplier_evidence", "audit"} {
		if !isStatutoryClass(class) {
			t.Fatalf("%s was not protected as statutory", class)
		}
	}
	if !errors.Is(classifyDatabaseError(nil), ErrUnavailable) {
		t.Fatal("a missing database row-count error was classified as success")
	}
}
