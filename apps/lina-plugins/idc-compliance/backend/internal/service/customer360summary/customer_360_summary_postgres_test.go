package customer360summary

import (
	"context"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestComplianceContributorUsesOneCompleteSetQueryPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run customer 360 PostgreSQL tests")
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
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
