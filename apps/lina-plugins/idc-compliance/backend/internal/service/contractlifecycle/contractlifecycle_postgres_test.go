package contractlifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type lifecycleAudit struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
	fail        bool
}

func (a *lifecycleAudit) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.transitions = append(a.transitions, input)
	if a.fail {
		return errors.New("audit unavailable")
	}
	return nil
}

func (*lifecycleAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type lifecycleGateway struct {
	items        []processauth.TencentContractDeliveryItem
	acknowledged []string
	key          string
}

func (g *lifecycleGateway) ListTencentContractDeliveries(
	context.Context,
	int,
) ([]processauth.TencentContractDeliveryItem, error) {
	return append([]processauth.TencentContractDeliveryItem(nil), g.items...), nil
}

func (g *lifecycleGateway) AcknowledgeTencentContractDeliveries(
	_ context.Context,
	numbers []string,
	key string,
) error {
	g.acknowledged = append([]string(nil), numbers...)
	g.key = key
	return nil
}

func TestContractLifecycleExactStatusPostgreSQL(t *testing.T) {
	database, cleanup := openLifecyclePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertLifecycleContract(t, ctx, database, "contract-1", "flow-1", "issue",
		"lwr_11111111111111111111111111111111")
	audit := &lifecycleAudit{}
	gateway := &lifecycleGateway{}
	serviceValue, err := New(database, gateway, audit)
	if err != nil {
		t.Fatalf("construct lifecycle service: %v", err)
	}

	observations := []Observation{
		lifecycleObservation("ldv_11111111111111111111111111111111", 2, "INIT", "callback", "applied"),
		lifecycleObservation("ldv_22222222222222222222222222222222", 3, "PART", "callback", "applied"),
		lifecycleObservation("ldv_33333333333333333333333333333333", 4, "WILLEXPIRE", "active_query", "applied"),
		lifecycleObservation("ldv_44444444444444444444444444444444", 5, "ALL", "callback", "applied"),
		lifecycleObservation("ldv_55555555555555555555555555555555", 6, "DEADLINE", "callback", "conflict"),
		lifecycleObservation("ldv_66666666666666666666666666666666", 7, "RELIEVED", "callback", "conflict"),
	}
	for index, observation := range observations {
		result, applyErr := serviceValue.Apply(ctx, observation, audit)
		if applyErr != nil {
			t.Fatalf("apply exact observation %d: %v", index, applyErr)
		}
		if index == 4 && result.DecisionCode != "conflict" {
			t.Fatalf("opposite terminal decision=%q want=conflict", result.DecisionCode)
		}
	}

	var state struct {
		ContractStatus      string `orm:"contract_status"`
		FlowStatus          string `orm:"flow_status"`
		ProviderStatus      string `orm:"provider_status"`
		GatewayRouteVersion int64  `orm:"gateway_route_version"`
	}
	if err = database.GetScan(ctx, &state, `
SELECT c.status_code AS contract_status, f.status_code AS flow_status,
       f.provider_status, f.gateway_route_version
FROM plugin_idc_compliance_contract c
JOIN plugin_idc_compliance_contract_flow f ON f.contract_no = c.contract_no
WHERE c.contract_no = 'contract-1'`); err != nil {
		t.Fatalf("read exact lifecycle state: %v", err)
	}
	if state.ContractStatus != "completed" || state.FlowStatus != "completed" ||
		state.ProviderStatus != "ALL" || state.GatewayRouteVersion != 7 {
		t.Fatalf("unexpected exact lifecycle state: %+v", state)
	}
	eventCount, err := database.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_compliance_contract_status_event
WHERE contract_no = 'contract-1'`)
	if err != nil || eventCount.Int() != len(observations) {
		t.Fatalf("timeline count=%d want=%d err=%v", eventCount.Int(), len(observations), err)
	}
	replayed, err := serviceValue.Apply(ctx, observations[len(observations)-1], audit)
	if err != nil || !replayed.Replayed {
		t.Fatalf("exact delivery replay was not idempotent result=%+v err=%v", replayed, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_contract_status_event
SET reason_code = 'tampered' WHERE contract_no = 'contract-1'`); err == nil {
		t.Fatal("append-only contract status timeline was mutable")
	}
}

func TestContractLifecycleWorkerAcknowledgesCommittedBatchPostgreSQL(t *testing.T) {
	database, cleanup := openLifecyclePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertLifecycleContract(t, ctx, database, "contract-2", "flow-2", "issue",
		"lwr_22222222222222222222222222222222")
	now := time.Now().UTC().Add(-time.Second).Unix()
	gateway := &lifecycleGateway{items: []processauth.TencentContractDeliveryItem{
		{
			DeliveryNo: "ldv_77777777777777777777777777777777",
			RouteNo:    "lwr_22222222222222222222222222222222", RouteVersion: 2,
			Status: "running", ProviderStatus: "PART", ReconciliationStatus: "none",
			SourceKind: "callback", DecisionCode: "applied",
			SupplierObservedUnix: now, ObservedUnix: now,
		},
	}}
	audit := &lifecycleAudit{}
	serviceValue, err := New(database, gateway, audit)
	if err != nil {
		t.Fatalf("construct lifecycle worker: %v", err)
	}
	result, err := serviceValue.Consume(ctx, MaximumBatchSize)
	if err != nil || result.Received != 1 || result.Acknowledged != 1 {
		t.Fatalf("consume lifecycle batch result=%+v err=%v", result, err)
	}
	if len(gateway.acknowledged) != 1 || gateway.acknowledged[0] != gateway.items[0].DeliveryNo ||
		!strings.HasPrefix(gateway.key, "contract-status-delivery:") {
		t.Fatalf("unexpected lifecycle acknowledgement numbers=%v key=%q", gateway.acknowledged, gateway.key)
	}
	if _, err = serviceValue.Consume(ctx, MaximumBatchSize); err != nil {
		t.Fatalf("replayed worker batch failed: %v", err)
	}
	count, err := database.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_compliance_contract_status_event WHERE contract_no = 'contract-2'`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("worker replay appended duplicate timeline count=%d err=%v", count.Int(), err)
	}
}

func TestContractLifecycleAuditFailureRollsBackPostgreSQL(t *testing.T) {
	database, cleanup := openLifecyclePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertLifecycleContract(t, ctx, database, "contract-3", "flow-3", "issue",
		"lwr_33333333333333333333333333333333")
	audit := &lifecycleAudit{fail: true}
	serviceValue, err := New(database, &lifecycleGateway{}, audit)
	if err != nil {
		t.Fatalf("construct rollback lifecycle service: %v", err)
	}
	observation := lifecycleObservation("ldv_88888888888888888888888888888888", 2, "PART", "callback", "applied")
	observation.RouteNo = "lwr_33333333333333333333333333333333"
	if _, err = serviceValue.Apply(ctx, observation, audit); err == nil {
		t.Fatal("audit failure did not reject exact lifecycle observation")
	}
	version, err := database.GetValue(ctx, `SELECT gateway_route_version
FROM plugin_idc_compliance_contract_flow WHERE flow_no = 'flow-3'`)
	if err != nil || version.Int64() != 1 {
		t.Fatalf("audit failure changed flow version=%d err=%v", version.Int64(), err)
	}
	count, err := database.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_compliance_contract_status_event WHERE flow_no = 'flow-3'`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("audit failure persisted timeline count=%d err=%v", count.Int(), err)
	}
}

func TestContractLifecycleDirectTerminalSnapshotCatchesUpPostgreSQL(t *testing.T) {
	database, cleanup := openLifecyclePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertLifecycleContract(t, ctx, database, "contract-4", "flow-4", "issue",
		"lwr_44444444444444444444444444444444")
	audit := &lifecycleAudit{}
	serviceValue, err := New(database, &lifecycleGateway{}, audit)
	if err != nil {
		t.Fatalf("construct direct snapshot lifecycle service: %v", err)
	}
	now := time.Now().UTC().Add(-time.Second)
	result, err := serviceValue.Apply(ctx, Observation{
		RouteNo: "lwr_44444444444444444444444444444444", RouteVersion: 2,
		ProviderStatus: "ALL", SourceKind: "active_query", GatewayDecisionCode: "stale",
		SupplierObservedAt: now, ObservedAt: now,
	}, audit)
	if err != nil || result.DecisionCode != "applied" || result.ContractStatus != "completed" {
		t.Fatalf("direct terminal snapshot result=%+v err=%v", result, err)
	}
}

func TestRequiredContractTerminalFreezesDefaultHandlingDeadlinePostgreSQL(t *testing.T) {
	database, cleanup := openLifecyclePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertLifecycleContract(t, ctx, database, "contract-5", "flow-5", "issue",
		"lwr_55555555555555555555555555555555")
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract
SET required_for_fulfillment = TRUE
WHERE contract_no = 'contract-5'`); err != nil {
		t.Fatalf("mark required contract fixture: %v", err)
	}
	audit := &lifecycleAudit{}
	serviceValue, err := New(database, &lifecycleGateway{}, audit)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Second)
	result, err := serviceValue.Apply(ctx, Observation{
		DeliveryNo: "ldv_99999999999999999999999999999999",
		RouteNo:    "lwr_55555555555555555555555555555555", RouteVersion: 2,
		ProviderStatus: "REJECT", SourceKind: "callback", GatewayDecisionCode: "applied",
		SupplierObservedAt: now, ObservedAt: now,
	}, audit)
	if err != nil || result.ContractStatus != "rejected" {
		t.Fatalf("apply required terminal result=%+v err=%v", result, err)
	}
	var failure struct {
		StatusCode   string    `orm:"status_code"`
		HandlingDays int       `orm:"handling_days"`
		Deadline     time.Time `orm:"handling_deadline"`
		CreatedAt    time.Time `orm:"created_at"`
	}
	if err = database.GetScan(ctx, &failure, `
SELECT status_code, handling_days, handling_deadline, created_at
FROM plugin_idc_compliance_contract_failure_case
WHERE contract_no = 'contract-5'`); err != nil {
		t.Fatalf("load frozen contract failure case: %v", err)
	}
	if failure.StatusCode != "pending" || failure.HandlingDays != 3 ||
		failure.Deadline.Sub(failure.CreatedAt) != 72*time.Hour {
		t.Fatalf("unexpected frozen contract failure case: %+v", failure)
	}
}

func TestCompletedReplacementClosesOriginalFailureDeadlinePostgreSQL(t *testing.T) {
	database, cleanup := openLifecyclePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertLifecycleContract(t, ctx, database, "contract-old", "flow-old", "issue",
		"lwr_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract
SET required_for_fulfillment = TRUE
WHERE contract_no = 'contract-old'`); err != nil {
		t.Fatal(err)
	}
	audit := &lifecycleAudit{}
	serviceValue, err := New(database, &lifecycleGateway{}, audit)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Second)
	if _, err = serviceValue.Apply(ctx, Observation{
		DeliveryNo: "ldv_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RouteNo:    "lwr_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RouteVersion: 2,
		ProviderStatus: "REJECT", SourceKind: "callback", GatewayDecisionCode: "applied",
		SupplierObservedAt: now, ObservedAt: now,
	}, audit); err != nil {
		t.Fatalf("fail original required contract: %v", err)
	}
	insertLifecycleContract(t, ctx, database, "contract-new", "flow-new", "reissue",
		"lwr_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_replacement
    (replacement_no, old_contract_no, new_contract_no)
VALUES ('replacement-1', 'contract-old', 'contract-new')`); err != nil {
		t.Fatal(err)
	}
	if _, err = serviceValue.Apply(ctx, Observation{
		DeliveryNo: "ldv_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RouteNo:    "lwr_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", RouteVersion: 2,
		ProviderStatus: "ALL", SourceKind: "callback", GatewayDecisionCode: "applied",
		SupplierObservedAt: now, ObservedAt: now,
	}, audit); err != nil {
		t.Fatalf("complete replacement contract: %v", err)
	}
	status, err := database.GetValue(ctx, `
SELECT status_code FROM plugin_idc_compliance_contract_failure_case
WHERE contract_no = 'contract-old'`)
	if err != nil || status.String() != "signed" {
		t.Fatalf("original failure case status=%q error=%v", status.String(), err)
	}
}

func lifecycleObservation(
	deliveryNo string,
	version int64,
	providerStatus string,
	source string,
	decision string,
) Observation {
	now := time.Now().UTC().Add(-time.Second)
	return Observation{
		DeliveryNo: deliveryNo, RouteNo: "lwr_11111111111111111111111111111111",
		RouteVersion: version, ProviderStatus: providerStatus, SourceKind: source,
		GatewayDecisionCode: decision, SupplierObservedAt: now, ObservedAt: now,
	}
}

func insertLifecycleContract(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	contractNo string,
	flowNo string,
	purpose string,
	routeNo string,
) {
	t.Helper()
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract
    (contract_no, customer_no, status_code)
VALUES (?, 'customer-1', 'signing')`, contractNo)
	if err != nil {
		t.Fatalf("insert lifecycle contract fixture: %v", err)
	}
	_, err = database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_flow
    (flow_no, contract_no, purpose_code, provider_code, status_code, gateway_route_no)
VALUES (?, ?, ?, 'tencent_essbasic', 'signing', ?)`,
		flowNo, contractNo, purpose, routeNo)
	if err != nil {
		t.Fatalf("insert lifecycle contract fixture: %v", err)
	}
}

func openLifecyclePostgreSQL(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_COMPLIANCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_COMPLIANCE_TEST_DSN to run contract lifecycle PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open contract lifecycle PostgreSQL admin connection: %v", err)
	}
	var randomBytes [8]byte
	if _, err = rand.Read(randomBytes[:]); err != nil {
		t.Fatalf("generate contract lifecycle schema: %v", err)
	}
	schema := "idc_contract_lifecycle_test_" + hex.EncodeToString(randomBytes[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create contract lifecycle schema: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("open scoped contract lifecycle database: %v", err)
	}
	if _, err = database.Exec(ctx, lifecycleFoundationSQL); err != nil {
		t.Fatalf("create contract lifecycle foundation: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract lifecycle migration path")
	}
	migrationRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql")
	for _, name := range []string{"016-contract-status-timeline.sql", "025-contract-failure-compensation.sql"} {
		migration, readErr := os.ReadFile(filepath.Join(migrationRoot, name))
		if readErr != nil {
			t.Fatalf("read contract lifecycle migration %s: %v", name, readErr)
		}
		for attempt := 1; attempt <= 2; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute contract lifecycle migration %s attempt %d: %v", name, attempt, err)
			}
		}
	}
	return database, func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	}
}

const lifecycleFoundationSQL = `
CREATE TABLE plugin_idc_compliance_contract (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    contract_no VARCHAR(64) NOT NULL UNIQUE,
    customer_no VARCHAR(64) NOT NULL,
    order_no VARCHAR(64) NOT NULL DEFAULT 'order-lifecycle-fixture',
    required_for_fulfillment BOOLEAN NOT NULL DEFAULT FALSE,
    status_code VARCHAR(24) NOT NULL,
    completed_at TIMESTAMPTZ,
    terminated_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE plugin_idc_compliance_contract_flow (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    flow_no VARCHAR(64) NOT NULL UNIQUE,
    contract_no VARCHAR(64) NOT NULL REFERENCES plugin_idc_compliance_contract(contract_no),
    purpose_code VARCHAR(16) NOT NULL,
    provider_code VARCHAR(32) NOT NULL,
    status_code VARCHAR(24) NOT NULL,
    completed_at TIMESTAMPTZ,
    failure_code VARCHAR(128) NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gateway_route_no VARCHAR(64) NOT NULL UNIQUE,
    UNIQUE (flow_no, contract_no)
);
CREATE TABLE plugin_idc_compliance_contract_replacement (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    replacement_no VARCHAR(64) NOT NULL UNIQUE,
    old_contract_no VARCHAR(64) NOT NULL REFERENCES plugin_idc_compliance_contract(contract_no),
    new_contract_no VARCHAR(64) NOT NULL REFERENCES plugin_idc_compliance_contract(contract_no),
    UNIQUE (old_contract_no),
    UNIQUE (new_contract_no)
);
CREATE OR REPLACE FUNCTION plugin_idc_compliance_reject_append_only_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'compliance history is append-only';
END;
$$;`
