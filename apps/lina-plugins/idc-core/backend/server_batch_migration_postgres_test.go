package backend

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestServerBatchMigrationAndIndependentTransactionsPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run server batch PostgreSQL tests", catalogIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_billing_test_") {
			_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		} else {
			t.Errorf("refusing to drop unexpected server batch schema %q", schema)
		}
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(
		dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })

	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_service_instance (
service_no VARCHAR(40) NOT NULL, customer_no VARCHAR(40) NOT NULL, currency_code CHAR(3) NOT NULL,
CONSTRAINT uk_batch_fixture_service UNIQUE(service_no),
CONSTRAINT uk_batch_fixture_owner UNIQUE(service_no,customer_no),
CONSTRAINT uk_batch_fixture_currency UNIQUE(service_no,customer_no,currency_code))`); err != nil {
		t.Fatal(err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve server batch migration path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "066-server-safe-batch.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute 066 migration attempt %d: %v", attempt, err)
		}
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_service_instance
(service_no,customer_no,currency_code) VALUES ('service-1','customer-1','CNY'),('service-2','customer-2','CNY')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_classification
(tenant_id,classification_no,name_zh_cn,name_en_us,status_code,version,created_by,updated_by)
VALUES (0,'scl_core','Core business','Business critical','active',1,7,7);
INSERT INTO plugin_idc_core_server_service_classification
(tenant_id,service_no,customer_no,classification_no,assigned_by)
VALUES (0,'service-1','customer-1','scl_core',7)`); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_service_batch
(tenant_id,batch_no,action_code,dangerous,status_code,item_count,preview_digest,idempotency_digest,
request_digest,requested_by) VALUES (0,'batch-1','terminate',TRUE,'previewed',2,?,?,?,7)`, digest, digest, digest); err != nil {
		t.Fatal(err)
	}
	for line, service := range []string{"service-1", "service-2"} {
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_service_batch_item
(tenant_id,batch_no,line_number,service_no,customer_no,service_version,currency_code,
entitlement_status_code,runtime_status_code,provider_task_status_code,unpaid_amount_minor,
action_amount_minor,resource_summary,irreversible,parameters,preview_status_code,execution_status_code)
VALUES (0,'batch-1',?,?,?,3,'CNY','active','running','none',0,0,'cloud_server / pool',TRUE,
'{}'::JSONB,'eligible','pending')`, line+1, service, "customer-"+string(rune('1'+line))); err != nil {
			t.Fatal(err)
		}
	}
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_server_service_batch
SET status_code='executing',confirmed_at=CURRENT_TIMESTAMP WHERE batch_no='batch-1'`); updateErr != nil {
			return updateErr
		}
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_server_service_batch_item
SET execution_status_code='succeeded',operation_no='operation-1',executed_at=CURRENT_TIMESTAMP
WHERE batch_no='batch-1' AND line_number=1 AND execution_status_code='pending'`); updateErr != nil {
			return updateErr
		}
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_server_service_batch_item
SET execution_status_code='manual_review',failure_code='upstream_unavailable',executed_at=CURRENT_TIMESTAMP
WHERE batch_no='batch-1' AND line_number=2 AND execution_status_code='pending'`); updateErr != nil {
			return updateErr
		}
		_, updateErr := database.Exec(ctx, `UPDATE plugin_idc_core_server_service_batch
SET status_code='completed',completed_at=CURRENT_TIMESTAMP WHERE batch_no='batch-1'`)
		return updateErr
	}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Completed, ManualReview int
	}
	if err = database.GetScan(ctx, &result, `SELECT
(SELECT COUNT(1)::INT FROM plugin_idc_core_server_service_batch WHERE status_code='completed') AS completed,
(SELECT COUNT(1)::INT FROM plugin_idc_core_server_service_batch_item WHERE execution_status_code='manual_review') AS manual_review`); err != nil {
		t.Fatal(err)
	}
	if result.Completed != 1 || result.ManualReview != 1 {
		t.Fatalf("unexpected independent batch result: %+v", result)
	}
}
