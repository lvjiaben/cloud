// This file verifies installation-local supplier collection against an isolated PostgreSQL schema.

package localusage

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/migration"
)

const localUsageTestDSNEnv = "LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN"

type scriptedCollector struct {
	calls   int
	collect func(int) (Collection, error)
}

type blockingCollector struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	result  Collection
}

func (c *blockingCollector) Collect(ctx context.Context) (Collection, error) {
	if c.calls.Add(1) == 1 {
		close(c.started)
	}
	select {
	case <-ctx.Done():
		return Collection{}, ctx.Err()
	case <-c.release:
		return c.result, nil
	}
}

func (c *scriptedCollector) Collect(context.Context) (Collection, error) {
	c.calls++
	return c.collect(c.calls)
}

func TestCollectionValidationRejectsUnboundedInput(t *testing.T) {
	input := sampleCollection(time.Now().UTC().Truncate(time.Second))
	if !validCollection(input) {
		t.Fatal("bounded fixture was rejected")
	}
	input.Reconciliations[0].SupplierCostMinor = math.MaxInt64
	if validCollection(input) {
		t.Fatal("overflow-prone reconciliation was accepted")
	}
	input = sampleCollection(time.Now().UTC().Truncate(time.Second))
	input.Usage = make([]UsageObservation, MaximumUsageRows+1)
	if validCollection(input) {
		t.Fatal("unbounded usage rows were accepted")
	}
}

func TestCollectionServicePersistsAtomicProjectionAndReplay(t *testing.T) {
	database, installationID, cleanup := openLocalUsageDatabase(t)
	defer cleanup()
	keyring := newLocalUsageKeyring(t)
	defer keyring.Clear()
	ctx := context.Background()
	if _, err := database.ExecContext(ctx, `
INSERT INTO idc_gateway_supplier_budget_policy
    (provider_code, currency_code, warning_minor, critical_minor, updated_by_employee_no)
VALUES
    ('tencent_essbasic', 'CNY', 500, 1000, 'employee_1'),
    ('tencent_essbasic', 'USD', 0, 0, 'employee_1')`); err != nil {
		t.Fatalf("insert budget policies: %v", err)
	}
	observedAt := time.Now().UTC().Truncate(time.Second)
	collector := &scriptedCollector{collect: func(call int) (Collection, error) {
		input := sampleCollection(observedAt)
		if call == 3 {
			input.Usage[0].QuantityNumerator++
		}
		return input, nil
	}}
	service, err := NewCollectionService(
		database, installationID, keyring,
		map[string]Collector{ProviderTencentESSBasic: collector},
	)
	if err != nil {
		t.Fatalf("construct collection service: %v", err)
	}
	if err = service.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare collection state: %v", err)
	}
	assertCollectionState(t, database, ProviderTencentESSBasic, "ready", 0, "")
	assertCollectionState(t, database, ProviderNuonuoInvoice, "disabled", 0, "")
	if err = service.RunOnce(ctx); err != nil {
		t.Fatalf("run first collection: %v", err)
	}
	assertCollectionState(t, database, ProviderTencentESSBasic, "ready", 0, "")
	assertCollectionCounts(t, database, 1, 1, 1, 2, 3, 1)
	var alertLevel string
	var threshold int64
	if err = database.QueryRowContext(ctx, `
SELECT level_code, threshold_minor
FROM idc_gateway_supplier_budget_alert
WHERE provider_code = 'tencent_essbasic' AND currency_code = 'USD'`).Scan(
		&alertLevel, &threshold,
	); err != nil || alertLevel != "critical" || threshold != 0 {
		t.Fatalf("zero budget alert level=%q threshold=%d err=%v", alertLevel, threshold, err)
	}
	var difference int64
	var reconciliationStatus string
	if err = database.QueryRowContext(ctx, `
SELECT difference_minor, status_code
FROM idc_gateway_supplier_reconciliation
WHERE provider_code = 'tencent_essbasic' AND currency_code = 'CNY'`).Scan(
		&difference, &reconciliationStatus,
	); err != nil || difference != 100 || reconciliationStatus != "mismatch" {
		t.Fatalf("reconciliation difference=%d status=%q err=%v", difference, reconciliationStatus, err)
	}
	var availability int
	if err = database.QueryRowContext(ctx, `
SELECT availability_ppm FROM idc_gateway_supplier_sla_snapshot
WHERE provider_code = 'tencent_essbasic'`).Scan(&availability); err != nil || availability != 900000 {
		t.Fatalf("availability=%d err=%v", availability, err)
	}
	var ciphertext []byte
	if err = database.QueryRowContext(ctx, `
SELECT payload_ciphertext FROM idc_gateway_supplier_evidence`).Scan(&ciphertext); err != nil {
		t.Fatalf("load encrypted evidence: %v", err)
	}
	if bytes.Contains(ciphertext, []byte("authoritative supplier usage frame")) {
		t.Fatal("supplier evidence was stored in plaintext")
	}
	makeCollectionDue(t, database, ProviderTencentESSBasic)
	if err = service.RunOnce(ctx); err != nil {
		t.Fatalf("run idempotent replay: %v", err)
	}
	if collector.calls != 2 {
		t.Fatalf("collector calls=%d want=2", collector.calls)
	}
	assertCollectionCounts(t, database, 1, 1, 1, 2, 3, 1)

	makeCollectionDue(t, database, ProviderTencentESSBasic)
	if err = service.RunOnce(ctx); err != nil {
		t.Fatalf("record conflicting replay: %v", err)
	}
	assertCollectionState(
		t, database, ProviderTencentESSBasic, "retry_wait", 1, string(CollectionFailureRejected),
	)
	assertCollectionCounts(t, database, 1, 1, 1, 2, 3, 1)
}

func TestCollectionServiceDisabledAndFailureBackoff(t *testing.T) {
	database, installationID, cleanup := openLocalUsageDatabase(t)
	defer cleanup()
	keyring := newLocalUsageKeyring(t)
	defer keyring.Clear()
	ctx := context.Background()
	disabled, err := NewCollectionService(database, installationID, keyring, map[string]Collector{})
	if err != nil {
		t.Fatalf("construct disabled collection service: %v", err)
	}
	if err = disabled.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare disabled collection service: %v", err)
	}
	if disabled.HasCollectors() || !errors.Is(disabled.RunOnce(ctx), ErrCollectorDisabled) {
		t.Fatal("disabled production boundary attempted to run")
	}
	assertCollectionState(t, database, ProviderTencentESSBasic, "disabled", 0, "")
	assertCollectionState(t, database, ProviderNuonuoInvoice, "disabled", 0, "")

	collector := &scriptedCollector{collect: func(int) (Collection, error) {
		return Collection{}, &CollectionError{Code: CollectionFailureTimeout}
	}}
	enabled, err := NewCollectionService(
		database, installationID, keyring,
		map[string]Collector{ProviderNuonuoInvoice: collector},
	)
	if err != nil {
		t.Fatalf("construct controlled collection service: %v", err)
	}
	if err = enabled.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare controlled collection service: %v", err)
	}
	if err = enabled.RunOnce(ctx); err != nil {
		t.Fatalf("run failed collection: %v", err)
	}
	assertCollectionState(
		t, database, ProviderNuonuoInvoice, "retry_wait", 1, string(CollectionFailureTimeout),
	)
	assertCollectionCounts(t, database, 0, 0, 0, 0, 0, 0)
}

func TestCollectionServiceLeasePreventsConcurrentProviderRead(t *testing.T) {
	database, installationID, cleanup := openLocalUsageDatabase(t)
	defer cleanup()
	keyring := newLocalUsageKeyring(t)
	defer keyring.Clear()
	collector := &blockingCollector{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  sampleCollection(time.Now().UTC().Truncate(time.Second)),
	}
	collectors := map[string]Collector{ProviderTencentESSBasic: collector}
	first, err := NewCollectionService(database, installationID, keyring, collectors)
	if err != nil {
		t.Fatalf("construct first collection service: %v", err)
	}
	second, err := NewCollectionService(database, installationID, keyring, collectors)
	if err != nil {
		t.Fatalf("construct second collection service: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = first.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare collection state: %v", err)
	}
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- first.RunOnce(ctx)
	}()
	select {
	case <-collector.started:
	case <-ctx.Done():
		t.Fatalf("first collector did not start: %v", ctx.Err())
	}
	if err = second.RunOnce(ctx); err != nil {
		t.Fatalf("run concurrent collection pass: %v", err)
	}
	if calls := collector.calls.Load(); calls != 1 {
		t.Fatalf("collector calls while lease held=%d want=1", calls)
	}
	close(collector.release)
	select {
	case err = <-firstResult:
		if err != nil {
			t.Fatalf("finish first collection pass: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("first collection pass did not finish: %v", ctx.Err())
	}
	assertCollectionCounts(t, database, 1, 1, 1, 0, 3, 1)
}

func TestCollectionServiceRunOnceClaimsBothProvidersWithFreshLeases(t *testing.T) {
	database, installationID, cleanup := openLocalUsageDatabase(t)
	defer cleanup()
	keyring := newLocalUsageKeyring(t)
	defer keyring.Clear()
	observedAt := time.Now().UTC().Truncate(time.Second)
	tencent := &scriptedCollector{collect: func(int) (Collection, error) {
		return sampleCollection(observedAt), nil
	}}
	nuonuo := &scriptedCollector{collect: func(int) (Collection, error) {
		input := sampleCollection(observedAt)
		input.ProviderCode = ProviderNuonuoInvoice
		return input, nil
	}}
	service, err := NewCollectionService(
		database, installationID, keyring,
		map[string]Collector{
			ProviderTencentESSBasic: tencent,
			ProviderNuonuoInvoice:   nuonuo,
		},
	)
	if err != nil {
		t.Fatalf("construct two-provider collection service: %v", err)
	}
	ctx := context.Background()
	if err = service.EnsureReady(ctx); err != nil {
		t.Fatalf("prepare two-provider collection state: %v", err)
	}
	if err = service.RunOnce(ctx); err != nil {
		t.Fatalf("run two-provider collection pass: %v", err)
	}
	if tencent.calls != 1 || nuonuo.calls != 1 {
		t.Fatalf("collector calls tencent=%d nuonuo=%d want=1/1", tencent.calls, nuonuo.calls)
	}
	assertCollectionCounts(t, database, 2, 2, 2, 0, 6, 2)
}

func sampleCollection(observedAt time.Time) Collection {
	periodStartedAt := observedAt.Add(-time.Hour)
	periodEndedAt := observedAt
	cost := int64(1200)
	return Collection{
		ProviderCode:  ProviderTencentESSBasic,
		EventID:       []byte("supplier-event-20260814-1"),
		EvidenceFrame: []byte("authoritative supplier usage frame"),
		ObservedAt:    observedAt,
		Plan: PlanObservation{
			Reference: []byte("provider-plan-1"), StatusCode: "active", Currency: "CNY",
			ValidFrom: &periodStartedAt,
		},
		Usage: []UsageObservation{{
			MetricCode: "api.calls", PeriodStartedAt: periodStartedAt,
			PeriodEndedAt: periodEndedAt, QuantityNumerator: 10,
			QuantityDenominator: 1, SupplierCostMinor: &cost, CurrencyCode: "CNY",
		}},
		Reconciliations: []ReconciliationInput{
			{PeriodStartedAt: periodStartedAt, PeriodEndedAt: periodEndedAt,
				CurrencyCode: "CNY", SupplierCostMinor: 1200,
				LocalObservedCostMinor: 1100, LocalEvidenceComplete: true},
			{PeriodStartedAt: periodStartedAt, PeriodEndedAt: periodEndedAt,
				CurrencyCode: "USD", SupplierCostMinor: 0,
				LocalObservedCostMinor: 0, LocalEvidenceComplete: true},
			{PeriodStartedAt: periodStartedAt, PeriodEndedAt: periodEndedAt,
				CurrencyCode: "EUR", SupplierCostMinor: 500,
				LocalObservedCostMinor: 0, LocalEvidenceComplete: false},
		},
		SLA: SLAObservation{
			PeriodStartedAt: periodStartedAt, PeriodEndedAt: periodEndedAt,
			RequestCount: 10, SuccessCount: 9, LatencyP95Millis: 100,
			LatencyP99Millis: 200,
		},
	}
}

func newLocalUsageKeyring(t *testing.T) *callbacksecurity.Ring {
	t.Helper()
	file, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate callback keyring: %v", err)
	}
	keyring, err := callbacksecurity.New(file)
	if err != nil {
		t.Fatalf("construct callback keyring: %v", err)
	}
	return keyring
}

func assertCollectionState(
	t *testing.T,
	database *sql.DB,
	provider string,
	status string,
	failures int,
	failureCode string,
) {
	t.Helper()
	var actualStatus string
	var actualFailures int
	var actualFailureCode sql.NullString
	var nextCollectAt sql.NullTime
	if err := database.QueryRowContext(context.Background(), `
SELECT status_code, failure_count, last_failure_code, next_collect_at
FROM idc_gateway_supplier_collection_state
WHERE provider_code = $1`, provider).Scan(
		&actualStatus, &actualFailures, &actualFailureCode, &nextCollectAt,
	); err != nil {
		t.Fatalf("load collection state for %s: %v", provider, err)
	}
	if actualStatus != status || actualFailures != failures ||
		actualFailureCode.String != failureCode || actualFailureCode.Valid != (failureCode != "") {
		t.Fatalf(
			"state provider=%s status=%s failures=%d code=%q want=%s/%d/%q",
			provider, actualStatus, actualFailures, actualFailureCode.String,
			status, failures, failureCode,
		)
	}
	if status == "disabled" && nextCollectAt.Valid {
		t.Fatalf("disabled provider %s has a schedule", provider)
	}
	if status != "disabled" && !nextCollectAt.Valid {
		t.Fatalf("enabled provider %s has no schedule", provider)
	}
}

func assertCollectionCounts(
	t *testing.T,
	database *sql.DB,
	operations int,
	evidence int,
	plans int,
	alerts int,
	reconciliations int,
	sla int,
) {
	t.Helper()
	checks := []struct {
		table string
		want  int
	}{
		{"idc_gateway_supplier_operation", operations},
		{"idc_gateway_supplier_evidence", evidence},
		{"idc_gateway_supplier_plan_snapshot", plans},
		{"idc_gateway_supplier_usage_snapshot", plans},
		{"idc_gateway_supplier_budget_alert", alerts},
		{"idc_gateway_supplier_reconciliation", reconciliations},
		{"idc_gateway_supplier_sla_snapshot", sla},
	}
	for _, check := range checks {
		var count int
		if err := database.QueryRowContext(
			context.Background(), "SELECT COUNT(*) FROM "+check.table,
		).Scan(&count); err != nil || count != check.want {
			t.Fatalf("table=%s count=%d want=%d err=%v", check.table, count, check.want, err)
		}
	}
}

func makeCollectionDue(t *testing.T, database *sql.DB, provider string) {
	t.Helper()
	if _, err := database.ExecContext(context.Background(), `
UPDATE idc_gateway_supplier_collection_state
SET next_collect_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE provider_code = $1 AND status_code = 'ready'`, provider); err != nil {
		t.Fatalf("make provider collection due: %v", err)
	}
}

func openLocalUsageDatabase(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(localUsageTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run local usage PostgreSQL tests", localUsageTestDSNEnv)
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
	schema := "idc_local_usage_test_" + hex.EncodeToString(suffix)
	if _, err = bootstrap.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	database := stdlib.OpenDB(*config)
	database.SetMaxOpenConns(8)
	database.SetMaxIdleConns(8)
	if err = migration.Apply(ctx, database); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	installationID := "linst_" + strings.Repeat("7", 64)
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_instance (installation_id) VALUES ($1)`,
		installationID); err != nil {
		t.Fatalf("insert local installation: %v", err)
	}
	return database, installationID, func() {
		if !strings.HasPrefix(schema, "idc_local_usage_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
			return
		}
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close local usage database: %v", closeErr)
		}
		if _, dropErr := bootstrap.ExecContext(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop schema: %v", dropErr)
		}
		if closeErr := bootstrap.Close(); closeErr != nil {
			t.Errorf("close bootstrap database: %v", closeErr)
		}
	}
}
