// This file verifies supplier-subject lifecycle and concurrency guarantees
// against the installation-local PostgreSQL database.

package supplieridentity

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"lina-platform/idc-compliance-gateway/internal/migration"
)

const supplierIdentityTestDSNEnv = "LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN"

type exactCanonicalizer struct{}

func (exactCanonicalizer) Canonicalize(_ context.Context, input []byte) ([]byte, error) {
	return append([]byte(nil), input...), nil
}

func TestSupplierIdentityPostgreSQLLifecycleAndRotation(t *testing.T) {
	database, cleanup := openSupplierIdentityDatabase(t)
	defer cleanup()
	file, err := Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	ring, err := New(file)
	if err != nil {
		t.Fatalf("construct keyring: %v", err)
	}
	defer ring.Clear()
	installationID := "linst_" + strings.Repeat("1", 64)
	service, err := NewService(database, installationID, ring, map[string]Canonicalizer{
		ProviderTencentESSBasic: exactCanonicalizer{},
	})
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}
	ctx := context.Background()
	if err = service.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare service: %v", err)
	}

	const contenderCount = 8
	type contenderResult struct {
		index  int
		result Result
		err    error
	}
	results := make(chan contenderResult, contenderCount)
	var wait sync.WaitGroup
	for index := 0; index < contenderCount; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			subject := []byte("provider-subject-001")
			key := bytes.Repeat([]byte{byte(index + 1)}, SecretBytes)
			result, bindErr := service.Bind(ctx, BindInput{
				Provider: ProviderTencentESSBasic, Subject: subject, IdempotencyKey: key,
			})
			if !allZero(subject) || !allZero(key) {
				bindErr = errors.Join(bindErr, errors.New("plaintext input was not cleared"))
			}
			results <- contenderResult{index: index, result: result, err: bindErr}
		}(index)
	}
	wait.Wait()
	close(results)
	var winner contenderResult
	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result.err == nil:
			successes++
			winner = result
		case errors.Is(result.err, ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected contender result: %+v", result)
		}
	}
	if successes != 1 || conflicts != contenderCount-1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	secondInstallationID := "linst_" + strings.Repeat("3", 64)
	secondService, err := NewService(database, secondInstallationID, ring, map[string]Canonicalizer{
		ProviderTencentESSBasic: exactCanonicalizer{},
	})
	if err != nil {
		t.Fatalf("construct second installation service: %v", err)
	}
	if err = secondService.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare second installation: %v", err)
	}
	if _, err = secondService.Bind(ctx, BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("provider-subject-001"),
		IdempotencyKey: bytes.Repeat([]byte{31}, SecretBytes),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("second active installation duplicate err=%v", err)
	}
	if _, err = database.ExecContext(ctx, `
UPDATE idc_gateway_local_subject_lookup SET status_code = 'revoked'
WHERE binding_no = $1 AND status_code = 'active'`, winner.result.BindingNo); err == nil {
		t.Fatal("database allowed lookup revocation while its subject was active")
	}
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_identity_operation
    (operation_no, installation_id, operation_kind, provider_code, binding_no,
     idempotency_key_version, idempotency_hmac, request_sha256, reason_code)
VALUES ($1, $2, 'subject_revoke', $3, $4, 1,
        decode(repeat('ee', 32), 'hex'), decode(repeat('ef', 32), 'hex'), $5)`,
		"lsi_"+strings.Repeat("e", 32), installationID,
		ProviderTencentESSBasic, winner.result.BindingNo, ReasonAuthorizationLost,
	); err == nil {
		t.Fatal("database accepted a revoke operation before its subject was revoked")
	}

	replaySubject := []byte("provider-subject-001")
	replayKey := bytes.Repeat([]byte{byte(winner.index + 1)}, SecretBytes)
	replayed, err := service.Bind(ctx, BindInput{
		Provider: ProviderTencentESSBasic, Subject: replaySubject, IdempotencyKey: replayKey,
	})
	if err != nil || !replayed.Replayed || replayed.BindingNo != winner.result.BindingNo {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	if _, err = service.Revoke(ctx, RevokeInput{
		BindingNo: winner.result.BindingNo, ReasonCode: ReasonAuthorizationLost,
		IdempotencyKey: bytes.Repeat([]byte{byte(winner.index + 1)}, SecretBytes),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-operation idempotency reuse err=%v", err)
	}

	wrongReasonKey := bytes.Repeat([]byte{21}, SecretBytes)
	if _, err = service.Revoke(ctx, RevokeInput{
		BindingNo: winner.result.BindingNo, ReasonCode: "free_text", IdempotencyKey: wrongReasonKey,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("free-text reason err=%v", err)
	}
	revokeKey := bytes.Repeat([]byte{22}, SecretBytes)
	if _, err = service.Revoke(ctx, RevokeInput{
		BindingNo: winner.result.BindingNo, ReasonCode: ReasonAuthorizationLost,
		IdempotencyKey: revokeKey,
	}); err != nil {
		t.Fatalf("revoke subject: %v", err)
	}
	secondBinding, err := secondService.Bind(ctx, BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("provider-subject-001"),
		IdempotencyKey: bytes.Repeat([]byte{32}, SecretBytes),
	})
	if err != nil {
		t.Fatalf("bind in second installation after first revocation: %v", err)
	}
	if _, err = secondService.Revoke(ctx, RevokeInput{
		BindingNo: secondBinding.BindingNo, ReasonCode: ReasonAuthorizationLost,
		IdempotencyKey: bytes.Repeat([]byte{33}, SecretBytes),
	}); err != nil {
		t.Fatalf("revoke second installation subject: %v", err)
	}
	if _, err = service.Bind(ctx, BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("provider-subject-001"),
		IdempotencyKey: bytes.Repeat([]byte{23}, SecretBytes),
	}); !errors.Is(err, ErrReopenRequired) {
		t.Fatalf("initial bind after revocation err=%v", err)
	}
	reopened, err := service.Reopen(ctx, ReopenInput{
		PreviousBindingNo: winner.result.BindingNo, Provider: ProviderTencentESSBasic,
		Subject: []byte("provider-subject-001"), ReasonCode: ReasonSupplierReauthorized,
		IdempotencyKey: bytes.Repeat([]byte{24}, SecretBytes),
	})
	if err != nil || reopened.BindingNo == winner.result.BindingNo {
		t.Fatalf("reopened=%+v err=%v", reopened, err)
	}
	var callbackBuffer []byte
	if err = service.WithActiveSubject(ctx, ProviderTencentESSBasic, func(subject []byte) error {
		if string(subject) != "provider-subject-001" {
			t.Fatalf("callback subject=%q", subject)
		}
		callbackBuffer = subject
		return nil
	}); err != nil {
		t.Fatalf("use active subject: %v", err)
	}
	if !allZero(callbackBuffer) {
		t.Fatal("callback plaintext was not cleared")
	}
	assertPanickingCallbackClearsPlaintext(t, ctx, service)

	file, err = Rotate(file, PurposeSubjectLookup, rand.Reader)
	if err != nil {
		t.Fatalf("rotate lookup family: %v", err)
	}
	file, err = Rotate(file, PurposeIdentityOperation, rand.Reader)
	if err != nil {
		t.Fatalf("rotate idempotency family: %v", err)
	}
	rotatedRing, err := New(file)
	if err != nil {
		t.Fatalf("construct rotated keyring: %v", err)
	}
	defer rotatedRing.Clear()
	rotatedService, err := NewService(database, installationID, rotatedRing, map[string]Canonicalizer{
		ProviderTencentESSBasic: exactCanonicalizer{},
	})
	if err != nil {
		t.Fatalf("construct rotated service: %v", err)
	}
	if err = rotatedService.EnsureReady(ctx); err != nil {
		t.Fatalf("synchronize retained lookup versions: %v", err)
	}
	if _, err = rotatedService.Bind(ctx, BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("provider-subject-001"),
		IdempotencyKey: bytes.Repeat([]byte{25}, SecretBytes),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-version duplicate err=%v", err)
	}
	replayed, err = rotatedService.Bind(ctx, BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("provider-subject-001"),
		IdempotencyKey: bytes.Repeat([]byte{byte(winner.index + 1)}, SecretBytes),
	})
	if err != nil || !replayed.Replayed || replayed.BindingNo != winner.result.BindingNo {
		t.Fatalf("cross-version replay=%+v err=%v", replayed, err)
	}

	if _, err = rotatedService.RevokeInstance(ctx, InstanceRevokeInput{
		IdempotencyKey: bytes.Repeat([]byte{26}, SecretBytes),
	}); err != nil {
		t.Fatalf("revoke instance: %v", err)
	}
	if err = rotatedService.WithActiveSubject(ctx, ProviderTencentESSBasic, func([]byte) error {
		return nil
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("active subject after instance revocation err=%v", err)
	}
	if _, err = rotatedService.Bind(ctx, BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("provider-subject-002"),
		IdempotencyKey: bytes.Repeat([]byte{27}, SecretBytes),
	}); !errors.Is(err, ErrInstanceRevoked) {
		t.Fatalf("bind after instance revocation err=%v", err)
	}

	var activeSubjects, subjectRows, plaintextMatches int
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FILTER (WHERE status_code = 'active'), COUNT(*)
FROM idc_gateway_local_supplier_subject`).Scan(&activeSubjects, &subjectRows); err != nil {
		t.Fatalf("count subject history: %v", err)
	}
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_local_supplier_subject
WHERE position(convert_to('provider-subject-001', 'UTF8') in subject_ciphertext) > 0`).Scan(&plaintextMatches); err != nil {
		t.Fatalf("scan ciphertext for plaintext: %v", err)
	}
	if activeSubjects != 0 || subjectRows != 3 || plaintextMatches != 0 {
		t.Fatalf("active=%d rows=%d plaintextMatches=%d", activeSubjects, subjectRows, plaintextMatches)
	}
	if _, err = database.ExecContext(ctx, `
DELETE FROM idc_gateway_local_supplier_subject WHERE binding_no = $1`, winner.result.BindingNo); err == nil {
		t.Fatal("database allowed supplier subject history deletion")
	}
	if _, err = database.ExecContext(ctx, `
UPDATE idc_gateway_local_identity_operation SET reason_code = reason_code
WHERE operation_no = $1`, winner.result.OperationNo); err == nil {
		t.Fatal("database allowed identity operation mutation")
	}
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_identity_operation
    (operation_no, installation_id, operation_kind, provider_code, binding_no,
     idempotency_key_version, idempotency_hmac, request_sha256, reason_code)
VALUES ($1, $2, 'subject_revoke', $3, $4, 1,
        decode(repeat('ab', 32), 'hex'), decode(repeat('cd', 32), 'hex'), $5)`,
		"lsi_"+strings.Repeat("f", 32), secondInstallationID,
		ProviderTencentESSBasic, winner.result.BindingNo, ReasonAuthorizationLost,
	); err == nil {
		t.Fatal("database allowed an operation to reference another installation's binding")
	}
}

func TestProductionServiceFailsClosedWithoutCanonicalizers(t *testing.T) {
	database, cleanup := openSupplierIdentityDatabase(t)
	defer cleanup()
	file, err := Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	ring, err := New(file)
	if err != nil {
		t.Fatalf("construct keyring: %v", err)
	}
	defer ring.Clear()
	service, err := NewService(
		database, "linst_"+strings.Repeat("2", 64), ring, nil,
	)
	if err != nil {
		t.Fatalf("construct closed service: %v", err)
	}
	if err = service.EnsureReady(context.Background()); err != nil {
		t.Fatalf("prepare closed service: %v", err)
	}
	if _, err = service.Bind(context.Background(), BindInput{
		Provider: ProviderTencentESSBasic, Subject: []byte("unconfirmed"),
		IdempotencyKey: bytes.Repeat([]byte{1}, SecretBytes),
	}); !errors.Is(err, ErrCanonicalizerOff) {
		t.Fatalf("unregistered canonicalizer err=%v", err)
	}
}

func openSupplierIdentityDatabase(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(supplierIdentityTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run supplier identity PostgreSQL tests", supplierIdentityTestDSNEnv)
	}
	bootstrap, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	ctx := context.Background()
	suffix := make([]byte, 8)
	if _, err = rand.Read(suffix); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	schema := "idc_supplier_identity_test_" + hex.EncodeToString(suffix)
	if _, err = bootstrap.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	database := stdlib.OpenDB(*config)
	database.SetMaxOpenConns(12)
	database.SetMaxIdleConns(12)
	if err = migration.Apply(ctx, database); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return database, func() {
		if !strings.HasPrefix(schema, "idc_supplier_identity_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
			return
		}
		_ = database.Close()
		if _, dropErr := bootstrap.ExecContext(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop schema: %v", dropErr)
		}
		_ = bootstrap.Close()
	}
}

func allZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}

func assertPanickingCallbackClearsPlaintext(t *testing.T, ctx context.Context, service *Service) {
	t.Helper()
	var callbackBuffer []byte
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("plaintext callback did not panic")
			}
		}()
		_ = service.WithActiveSubject(ctx, ProviderTencentESSBasic, func(subject []byte) error {
			callbackBuffer = subject
			panic("expected callback failure")
		})
	}()
	if !allZero(callbackBuffer) {
		t.Fatal("panicking callback plaintext was not cleared")
	}
}
