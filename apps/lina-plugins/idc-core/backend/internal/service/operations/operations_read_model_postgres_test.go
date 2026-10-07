package operations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/bizctxcap"
)

func TestOperationsSignalsPersistAndUpdatePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run operations PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	var token [8]byte
	if _, err = rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_operations_test_" + hex.EncodeToString(token[:])
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`) })
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	for _, statement := range []string{
		`CREATE TABLE plugin_idc_core_async_task(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(24) NOT NULL,updated_at TIMESTAMPTZ NOT NULL,task_no VARCHAR(40),task_type VARCHAR(64),subject_type VARCHAR(32),subject_no VARCHAR(64),attempt_count INT,last_error_code VARCHAR(128),last_error_summary VARCHAR(1024),created_at TIMESTAMPTZ)`,
		`CREATE TABLE plugin_idc_core_commerce_order(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_risk_record(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_billing_invoice(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),currency_code VARCHAR(3),total_amount_minor BIGINT,paid_amount_minor BIGINT,refunded_amount_minor BIGINT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_service_instance(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,runtime_status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`ALTER TABLE plugin_idc_core_service_instance ADD COLUMN entitlement_expires_at TIMESTAMPTZ`,
		`CREATE TABLE plugin_idc_core_catalog_product(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_support_ticket(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,ticket_no VARCHAR(40),closed_at TIMESTAMPTZ,sla_risk_code VARCHAR(32),current_round INT,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_support_ticket_sla_round(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,ticket_no VARCHAR(40),round_no INT,stopped_at TIMESTAMPTZ,resolution_due_at TIMESTAMPTZ)`,
		`CREATE TABLE plugin_idc_core_refund_request(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_server_ip_change_intent(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,reservation_status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_individual_profile(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,identity_status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_enterprise_profile(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,verification_status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_contract_failure_compensation(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_sales_commission_settlement(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),requested_at TIMESTAMPTZ,completed_at TIMESTAMPTZ)`,
		`CREATE TABLE plugin_idc_core_business_operation(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,status_code VARCHAR(32),updated_at TIMESTAMPTZ NOT NULL)`,
	} {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	filename, _ := os.Getwd()
	migration, err := os.ReadFile(filepath.Join(filename, "..", "..", "..", "..", "manifest", "sql", "100-operations-read-model.sql"))
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0)
	for _, line := range strings.Split(string(migration), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "--") {
			continue
		}
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatalf("apply operations migration: %v\n%s", err, statement)
		}
	}
	service, err := New(database, operationsBizCtxStub{current: bizctxcap.CurrentContext{UserID: 1, TenantID: 0, DataScope: 2}})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err = service.UpsertSignal(context.Background(), Signal{Kind: "adapter_health", Code: "provider-a", Status: "critical", Count: 1, ObservedAt: now, SourceRevision: "r1"}); err != nil {
		t.Fatal(err)
	}
	if err = service.UpsertSignal(context.Background(), Signal{Kind: "adapter_health", Code: "provider-a", Status: "warning", Count: 2, AgeSeconds: 4, ObservedAt: now.Add(time.Second), SourceRevision: "r2"}); err != nil {
		t.Fatal(err)
	}
	alerts, err := service.ListOperationalAlerts(context.Background())
	if err != nil || len(alerts) != 1 {
		t.Fatalf("alerts=%+v err=%v", alerts, err)
	}
	if alerts[0].Code != "provider-a" || alerts[0].Severity != "warning" || alerts[0].Count != 2 {
		t.Fatalf("unexpected alert: %+v", alerts[0])
	}
	if taskAlerts, err := service.ListTaskAlerts(context.Background()); err != nil || len(taskAlerts) != 1 || taskAlerts[0].Code != "adapter_health:provider-a" {
		t.Fatalf("unexpected combined alerts=%+v err=%v", taskAlerts, err)
	}
	if _, err = service.Refresh(context.Background(), RefreshRequest{}); err != nil {
		t.Fatalf("refresh read model: %v", err)
	}
	if count, err := database.GetValue(context.Background(), `SELECT count(*) FROM plugin_idc_core_operations_read_model WHERE tenant_id=0`); err != nil || count.Int64() != 1 {
		t.Fatalf("read model rows=%d err=%v", count.Int64(), err)
	}
}
