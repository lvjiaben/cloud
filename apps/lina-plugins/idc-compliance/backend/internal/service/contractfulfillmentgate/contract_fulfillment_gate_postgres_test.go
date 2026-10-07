package contractfulfillmentgate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/contractfulfillmentcap"
)

const postgresDSNEnvironment = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

func TestBatchEvaluatePostgreSQLAuthoritativeStates(t *testing.T) {
	database, cleanup := newPostgreSQLDatabase(t)
	defer cleanup()
	ctx := context.Background()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract
    (tenant_id, order_no, required_for_fulfillment, status_code)
VALUES
    (0, 'ORDER_COMPLETED', TRUE, 'completed'),
    (0, 'ORDER_SIGNING', TRUE, 'signing'),
    (0, 'ORDER_OPTIONAL_EVIDENCE', FALSE, 'completed')`); err != nil {
		t.Fatalf("seed contracts: %v", err)
	}
	service, err := New(database)
	if err != nil {
		t.Fatalf("construct gate: %v", err)
	}
	decisions, err := service.BatchEvaluate(ctx, []contractfulfillmentcap.Requirement{
		{OrderNo: "ORDER_NO_CONTRACT_NEEDED", RequiresContract: false},
		{OrderNo: "ORDER_COMPLETED", RequiresContract: true},
		{OrderNo: "ORDER_SIGNING", RequiresContract: true},
		{OrderNo: "ORDER_MISSING", RequiresContract: true},
		{OrderNo: "ORDER_OPTIONAL_EVIDENCE", RequiresContract: true},
	})
	if err != nil {
		t.Fatalf("evaluate PostgreSQL states: %v", err)
	}
	wantReady := []bool{true, true, false, false, false}
	for index, decision := range decisions {
		if decision.Ready != wantReady[index] {
			t.Fatalf("decision[%d]=%+v", index, decision)
		}
	}
}

func newPostgreSQLDatabase(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(postgresDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run contract fulfillment PostgreSQL tests", postgresDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL admin database: %v", err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_contract_gate_test_" + hex.EncodeToString(random[:])
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	scopedDSN := dsn + "?search_path=" + schema
	if strings.Contains(dsn, "?") {
		scopedDSN = dsn + "&search_path=" + schema
	}
	database, err := gdb.New(gdb.ConfigNode{Link: scopedDSN})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_compliance_contract (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    order_no VARCHAR(40) NOT NULL,
    required_for_fulfillment BOOLEAN NOT NULL DEFAULT FALSE,
    status_code VARCHAR(24) NOT NULL,
    UNIQUE (tenant_id, order_no)
);
CREATE INDEX idx_contract_fulfillment_gate
    ON plugin_idc_compliance_contract
    (tenant_id, required_for_fulfillment, status_code, order_no, id)
    WHERE required_for_fulfillment;`); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "idc_contract_gate_test_") {
			t.Errorf("refusing unexpected schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); dropErr != nil {
			t.Errorf("drop test schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close admin database: %v", closeErr)
		}
	}
	return database, cleanup
}
