// This file verifies local rate, lease, circuit, probe, and unknown-result
// guarantees against a real PostgreSQL database.

package supplierguard

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"lina-platform/idc-compliance-gateway/internal/migration"
)

const supplierGuardTestDSNEnv = "LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN"

func TestSupplierGuardPostgreSQLSingleLeaseAndRateRollback(t *testing.T) {
	database, installationID, cleanup := openSupplierGuardDatabase(t)
	defer cleanup()
	policy := DefaultPolicy()
	policy.InstallationRate = RatePolicy{Window: time.Minute, Limit: 10}
	policy.OperationRate = RatePolicy{Window: time.Minute, Limit: 1}
	service := mustSupplierGuardService(t, database, installationID, policy)
	ctx := context.Background()
	first := insertBusinessRequest(t, database, installationID, 1, "contract.create")
	second := insertBusinessRequest(t, database, installationID, 2, "contract.create")

	const contenders = 8
	results := make(chan struct {
		lease Lease
		err   error
	}, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			lease, err := service.Acquire(ctx, Admission{
				RequestNo: first, Provider: ProviderTencentESSBasic,
				OperationScope: "contract.create",
			})
			results <- struct {
				lease Lease
				err   error
			}{lease: lease, err: err}
		}()
	}
	wait.Wait()
	close(results)
	callers := 0
	recovered := 0
	var attemptNo string
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent acquire: %v", result.err)
		}
		if attemptNo == "" {
			attemptNo = result.lease.AttemptNo
		} else if result.lease.AttemptNo != attemptNo {
			t.Fatalf("attempt=%q want=%q", result.lease.AttemptNo, attemptNo)
		}
		if result.lease.CallSupplier {
			callers++
		}
		if result.lease.Recovered {
			recovered++
		}
	}
	if callers != 1 || recovered != contenders-1 {
		t.Fatalf("callers=%d recovered=%d", callers, recovered)
	}
	_, err := service.Acquire(ctx, Admission{
		RequestNo: second, Provider: ProviderTencentESSBasic,
		OperationScope: "contract.create",
	})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("operation rate error=%v", err)
	}
	var installationCount int
	if err = database.QueryRowContext(ctx, `
SELECT request_count FROM idc_gateway_local_rate_limit_window
WHERE dimension_kind = 'installation' AND installation_id = $1 AND operation_scope = ''`,
		installationID).Scan(&installationCount); err != nil || installationCount != 1 {
		t.Fatalf("installation rate count=%d want=1 err=%v", installationCount, err)
	}

	observed, err := service.Observe(ctx, Observation{
		AttemptNo: attemptNo, Result: ResultUnknown,
		StableCode: "transport_ambiguous", Elapsed: 1500 * time.Millisecond,
	})
	if err != nil || observed.Result != ResultUnknown {
		t.Fatalf("observe unknown=%+v err=%v", observed, err)
	}
	replayed, err := service.Acquire(ctx, Admission{
		RequestNo: first, Provider: ProviderTencentESSBasic,
		OperationScope: "contract.create",
	})
	if err != nil || replayed.CallSupplier || !replayed.Recovered || replayed.Result != ResultUnknown {
		t.Fatalf("unknown replay=%+v err=%v", replayed, err)
	}
}

func TestSupplierGuardPostgreSQLCircuitAndSingleProbe(t *testing.T) {
	database, installationID, cleanup := openSupplierGuardDatabase(t)
	defer cleanup()
	service := mustSupplierGuardService(t, database, installationID, DefaultPolicy())
	ctx := context.Background()
	results := []ResultClass{
		ResultSucceeded, ResultSucceeded,
		ResultAvailabilityFailed, ResultAvailabilityFailed, ResultAvailabilityFailed,
	}
	for index, resultClass := range results {
		requestNo := insertBusinessRequest(t, database, installationID, index+1, "invoice.issue")
		lease, err := service.Acquire(ctx, Admission{
			RequestNo: requestNo, Provider: ProviderNuonuoInvoice,
			OperationScope: "invoice.issue",
		})
		if err != nil || !lease.CallSupplier {
			t.Fatalf("acquire sample %d lease=%+v err=%v", index, lease, err)
		}
		if _, err = service.Observe(ctx, Observation{
			AttemptNo: lease.AttemptNo, Result: resultClass,
			StableCode: fmt.Sprintf("sample_%d", index), Elapsed: 100 * time.Millisecond,
		}); err != nil {
			t.Fatalf("observe sample %d: %v", index, err)
		}
	}
	blocked := insertBusinessRequest(t, database, installationID, 20, "invoice.issue")
	if _, err := service.Acquire(ctx, Admission{
		RequestNo: blocked, Provider: ProviderNuonuoInvoice,
		OperationScope: "invoice.issue",
	}); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("open circuit error=%v", err)
	}
	if _, err := database.ExecContext(ctx, `
UPDATE idc_gateway_supplier_circuit
SET open_until = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE provider_code = $1 AND operation_scope = $2`,
		ProviderNuonuoInvoice, "invoice.issue"); err != nil {
		t.Fatalf("expire circuit: %v", err)
	}

	const contenders = 8
	type contenderResult struct {
		lease Lease
		err   error
	}
	channel := make(chan contenderResult, contenders)
	var wait sync.WaitGroup
	for index := 0; index < contenders; index++ {
		requestNo := insertBusinessRequest(
			t, database, installationID, 30+index, "invoice.issue",
		)
		wait.Add(1)
		go func(requestNo string) {
			defer wait.Done()
			lease, acquireErr := service.Acquire(ctx, Admission{
				RequestNo: requestNo, Provider: ProviderNuonuoInvoice,
				OperationScope: "invoice.issue",
			})
			channel <- contenderResult{lease: lease, err: acquireErr}
		}(requestNo)
	}
	wait.Wait()
	close(channel)
	probes := 0
	blockedCount := 0
	var probe Lease
	for result := range channel {
		switch {
		case result.err == nil && result.lease.CallSupplier && result.lease.Probe:
			probes++
			probe = result.lease
		case errors.Is(result.err, ErrCircuitOpen):
			blockedCount++
		default:
			t.Fatalf("unexpected probe contender=%+v", result)
		}
	}
	if probes != 1 || blockedCount != contenders-1 {
		t.Fatalf("probes=%d blocked=%d", probes, blockedCount)
	}
	if _, err := service.Observe(ctx, Observation{
		AttemptNo: probe.AttemptNo, Result: ResultSucceeded,
		StableCode: "probe_succeeded", Elapsed: 80 * time.Millisecond,
	}); err != nil {
		t.Fatalf("observe probe: %v", err)
	}
	afterClose := insertBusinessRequest(t, database, installationID, 50, "invoice.issue")
	lease, err := service.Acquire(ctx, Admission{
		RequestNo: afterClose, Provider: ProviderNuonuoInvoice,
		OperationScope: "invoice.issue",
	})
	if err != nil || !lease.CallSupplier || lease.Probe {
		t.Fatalf("closed circuit lease=%+v err=%v", lease, err)
	}
	if _, err = database.ExecContext(ctx, `
UPDATE idc_gateway_local_supplier_instance
SET status_code = 'revoked', revoked_at = CURRENT_TIMESTAMP,
    revoke_reason_code = 'instance_revoked', updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE installation_id = $1`, installationID); err != nil {
		t.Fatalf("revoke local installation: %v", err)
	}
	afterRevoke := insertBusinessRequest(t, database, installationID, 51, "invoice.cancel")
	if _, err = service.Acquire(ctx, Admission{
		RequestNo: afterRevoke, Provider: ProviderNuonuoInvoice,
		OperationScope: "invoice.cancel",
	}); !errors.Is(err, ErrRejected) {
		t.Fatalf("revoked installation error=%v", err)
	}
}

func TestSupplierGuardPostgreSQLProbeTimeoutAndBoundedSamples(t *testing.T) {
	database, installationID, cleanup := openSupplierGuardDatabase(t)
	defer cleanup()
	policy := DefaultPolicy()
	policy.ProbeTimeout = time.Second
	service := mustSupplierGuardService(t, database, installationID, policy)
	ctx := context.Background()
	for index := 0; index < 25; index++ {
		requestNo := insertBusinessRequest(t, database, installationID, index+1, "contract.query")
		lease, err := service.Acquire(ctx, Admission{
			RequestNo: requestNo, Provider: ProviderTencentESSBasic,
			OperationScope: "contract.query",
		})
		if err != nil {
			t.Fatalf("acquire bounded sample %d: %v", index, err)
		}
		if _, err = service.Observe(ctx, Observation{
			AttemptNo: lease.AttemptNo, Result: ResultSucceeded,
			StableCode: "query_succeeded", Elapsed: 20 * time.Millisecond,
		}); err != nil {
			t.Fatalf("observe bounded sample %d: %v", index, err)
		}
	}
	var sampleCount int
	if err := database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_supplier_health_sample
WHERE provider_code = $1 AND operation_scope = $2`,
		ProviderTencentESSBasic, "contract.query").Scan(&sampleCount); err != nil || sampleCount != 20 {
		t.Fatalf("sample count=%d want=20 err=%v", sampleCount, err)
	}

	for index, resultClass := range []ResultClass{
		ResultSucceeded, ResultSucceeded, ResultSucceeded, ResultUnknown, ResultUnknown,
	} {
		requestNo := insertBusinessRequest(t, database, installationID, 100+index, "contract.sign")
		lease, err := service.Acquire(ctx, Admission{
			RequestNo: requestNo, Provider: ProviderTencentESSBasic,
			OperationScope: "contract.sign",
		})
		if err != nil {
			t.Fatalf("acquire unknown sample %d: %v", index, err)
		}
		if _, err = service.Observe(ctx, Observation{
			AttemptNo: lease.AttemptNo, Result: resultClass,
			StableCode: fmt.Sprintf("sign_sample_%d", index), Elapsed: 30 * time.Millisecond,
		}); err != nil {
			t.Fatalf("observe unknown sample %d: %v", index, err)
		}
	}
	if _, err := database.ExecContext(ctx, `
UPDATE idc_gateway_supplier_circuit
SET open_until = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE provider_code = $1 AND operation_scope = $2`,
		ProviderTencentESSBasic, "contract.sign"); err != nil {
		t.Fatalf("expire unknown circuit: %v", err)
	}
	probeRequest := insertBusinessRequest(t, database, installationID, 120, "contract.sign")
	probe, err := service.Acquire(ctx, Admission{
		RequestNo: probeRequest, Provider: ProviderTencentESSBasic,
		OperationScope: "contract.sign",
	})
	if err != nil || !probe.Probe {
		t.Fatalf("acquire timeout probe=%+v err=%v", probe, err)
	}
	time.Sleep(1100 * time.Millisecond)
	nextRequest := insertBusinessRequest(t, database, installationID, 121, "contract.sign")
	if _, err = service.Acquire(ctx, Admission{
		RequestNo: nextRequest, Provider: ProviderTencentESSBasic,
		OperationScope: "contract.sign",
	}); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expired probe error=%v", err)
	}
	var attemptStatus, resultClass, requestStatus string
	if err = database.QueryRowContext(ctx, `
SELECT a.status_code, a.result_class, r.status_code
FROM idc_gateway_local_supplier_write_attempt a
JOIN idc_gateway_local_business_request r ON r.request_no = a.request_no
WHERE a.attempt_no = $1`, probe.AttemptNo).Scan(
		&attemptStatus, &resultClass, &requestStatus,
	); err != nil || attemptStatus != "observed" || resultClass != "result_unknown" ||
		requestStatus != "result_unknown" {
		t.Fatalf("expired probe attempt=%s result=%s request=%s err=%v",
			attemptStatus, resultClass, requestStatus, err)
	}
}

func mustSupplierGuardService(
	t *testing.T,
	database *sql.DB,
	installationID string,
	policy Policy,
) *Service {
	t.Helper()
	service, err := NewService(database, installationID, policy)
	if err != nil {
		t.Fatalf("construct supplier guard: %v", err)
	}
	return service
}

func insertBusinessRequest(
	t *testing.T,
	database *sql.DB,
	installationID string,
	index int,
	operationScope string,
) string {
	t.Helper()
	requestNo := fmt.Sprintf("lbr_%032x", index)
	idempotency := sha256.Sum256([]byte("idempotency\x00" + requestNo))
	target := sha256.Sum256([]byte("target\x00" + requestNo))
	content := sha256.Sum256([]byte("content\x00" + requestNo))
	body := sha256.Sum256([]byte("body\x00" + requestNo))
	_, err := database.ExecContext(context.Background(), `
INSERT INTO idc_gateway_local_business_request
    (request_no, installation_id, operation_scope, method, request_target_sha256,
     idempotency_hmac, business_content_sha256, body_sha256)
VALUES ($1, $2, $3, 'POST', $4, $5, $6, $7)`,
		requestNo, installationID, operationScope, target[:], idempotency[:], content[:], body[:])
	if err != nil {
		t.Fatalf("insert business request %s: %v", requestNo, err)
	}
	return requestNo
}

func openSupplierGuardDatabase(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(supplierGuardTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run supplier guard PostgreSQL tests", supplierGuardTestDSNEnv)
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
	schema := "idc_supplier_guard_test_" + hex.EncodeToString(suffix)
	if _, err = bootstrap.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	database := stdlib.OpenDB(*config)
	database.SetMaxOpenConns(16)
	database.SetMaxIdleConns(16)
	if err = migration.Apply(ctx, database); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	installationID := "linst_" + strings.Repeat("1", 64)
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_instance (installation_id)
VALUES ($1)`, installationID); err != nil {
		t.Fatalf("insert local installation: %v", err)
	}
	return database, installationID, func() {
		if !strings.HasPrefix(schema, "idc_supplier_guard_test_") {
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
