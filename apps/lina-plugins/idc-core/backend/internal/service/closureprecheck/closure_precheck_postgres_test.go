package closureprecheck

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/customerclosurecap"
)

func TestCoreContributorCountsGenericActiveEntitlementsPostgreSQL(t *testing.T) {
	database := newClosurePrecheckPostgreSQLDatabase(t)
	ctx := t.Context()

	statements := []string{
		`CREATE TABLE plugin_idc_core_service_entitlement (tenant_id BIGINT NOT NULL, customer_no VARCHAR(40) NOT NULL, status_code VARCHAR(24) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_billing_invoice (tenant_id BIGINT NOT NULL, customer_no VARCHAR(40) NOT NULL, status_code VARCHAR(24) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_business_operation (tenant_id BIGINT NOT NULL, customer_no VARCHAR(40) NOT NULL, status_code VARCHAR(40) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_billing_ledger_account (tenant_id BIGINT NOT NULL, account_no VARCHAR(40) NOT NULL, customer_no VARCHAR(40) NOT NULL, status_code VARCHAR(24) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_billing_balance_snapshot (tenant_id BIGINT NOT NULL, account_no VARCHAR(40) NOT NULL, sequence_no BIGINT NOT NULL, closing_balance_minor BIGINT NOT NULL)`,
		`CREATE TABLE plugin_idc_core_billing_fund_hold (tenant_id BIGINT NOT NULL, customer_no VARCHAR(40) NOT NULL, status_code VARCHAR(24) NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatalf("create closure precheck fixture: %v", err)
		}
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_entitlement(tenant_id,customer_no,status_code) VALUES
 (0,'CUS_A','active'),(0,'CUS_A','suspended'),(0,'CUS_A','expired'),(0,'CUS_A','terminated'),
 (0,'CUS_B','active'),(0,'CUS_OTHER','active'),(9,'CUS_A','active')`); err != nil {
		t.Fatalf("seed closure precheck fixture: %v", err)
	}

	contributor, err := NewCoreContributor(database)
	if err != nil {
		t.Fatalf("new core contributor: %v", err)
	}
	items, err := contributor.BatchCount(ctx, []string{"CUS_A", "CUS_B", "CUS_EMPTY"}, time.Now())
	if err != nil {
		t.Fatalf("count closure blockers: %v", err)
	}
	counts := make(map[string]int, len(items))
	for _, item := range items {
		counts[item.CustomerNo+"/"+string(item.Code)] = item.Count
	}
	if got := counts["CUS_A/"+string(customerclosurecap.CodeActiveServices)]; got != 2 {
		t.Fatalf("CUS_A active services = %d, want 2 active generic entitlements", got)
	}
	if got := counts["CUS_B/"+string(customerclosurecap.CodeActiveServices)]; got != 1 {
		t.Fatalf("CUS_B active services = %d, want 1 active generic entitlement", got)
	}
	if got := counts["CUS_EMPTY/"+string(customerclosurecap.CodeActiveServices)]; got != 0 {
		t.Fatalf("CUS_EMPTY active services = %d, want 0", got)
	}
	if len(items) != 3*len(contributor.Codes()) {
		t.Fatalf("result count = %d, want %d complete customer/code results", len(items), 3*len(contributor.Codes()))
	}
}

func newClosurePrecheckPostgreSQLDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run closure precheck PostgreSQL tests")
	}
	adminDatabase, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	randomBytes := make([]byte, 8)
	if _, err = rand.Read(randomBytes); err != nil {
		t.Fatalf("generate isolated schema name: %v", err)
	}
	schema := "closure_precheck_" + hex.EncodeToString(randomBytes)
	if _, err = adminDatabase.Exec(t.Context(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDatabase.Exec(t.Context(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
		_ = adminDatabase.Close(t.Context())
	})

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(t.Context()) })
	currentSchemaValue, err := database.GetValue(t.Context(), `SELECT current_schema()`)
	if err != nil {
		t.Fatalf("read isolated schema identity: %v", err)
	}
	currentSchema := currentSchemaValue.String()
	if currentSchema != schema {
		t.Fatalf("unsafe PostgreSQL test target: current schema %q, want %q", currentSchema, schema)
	}
	return database
}
