package customer360summary

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestComplianceContributorUsesOneCompleteSetQueryPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_COMPLIANCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_COMPLIANCE_TEST_DSN to run customer 360 PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_360_test_" + hex.EncodeToString(nonce[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Error(err)
		}
		_ = admin.Close(ctx)
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	// Only the columns consumed by this owner projection are needed here.
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_compliance_contract(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,customer_no VARCHAR(40) NOT NULL);CREATE TABLE plugin_idc_compliance_tax_invoice(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL,customer_no VARCHAR(40) NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	contributor := &Contributor{database: database}
	items, err := contributor.BatchCount(context.Background(), []string{"CUS_missing_one", "CUS_missing_two"})
	if err != nil || len(items) != 4 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	for _, item := range items {
		if item.Total != 0 || item.CustomerNo == "" || item.Code == "" {
			t.Fatalf("unexpected empty customer summary: %+v", item)
		}
	}
}
