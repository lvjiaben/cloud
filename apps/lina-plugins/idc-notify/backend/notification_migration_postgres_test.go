package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const notificationIntegrationDSNEnv = "LINAPRO_IDC_NOTIFY_TEST_DSN"

func TestNotificationFoundationMigrationPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(notificationIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL notification migration tests", notificationIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := newNotificationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteNotificationIdentifier(schema)); err != nil {
		t.Fatalf("create notification schema: %v", err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "linapro_notify_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoteNotificationIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop notification schema: %v", dropErr)
		}
		_ = admin.Close(ctx)
	}()
	database, err := gdb.New(gdb.ConfigNode{Link: appendNotificationDSN(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped PostgreSQL: %v", err)
	}
	defer database.Close(ctx)

	install := readNotificationSQL(t, filepath.Join("manifest", "sql", "001-notification-foundation.sql"))
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, install); err != nil {
			t.Fatalf("apply notification migration attempt %d: %v", attempt, err)
		}
	}
	count, err := database.GetValue(ctx, "SELECT count(*) FROM plugin_idc_notify_channel")
	if err != nil || count.Int() != 8 {
		t.Fatalf("expected five required and three extension channels, count=%d err=%v", count.Int(), err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_notify_channel
        (tenant_id,channel_code,channel_kind,display_name) VALUES (1,'cross_tenant','email','Cross tenant')`); err == nil {
		t.Fatal("notification channel accepted a cross-tenant row")
	}
	preferenceTable, err := database.GetValue(ctx, "SELECT to_regclass('plugin_idc_notify_preference')")
	if err != nil || !preferenceTable.IsNil() {
		t.Fatalf("notify must not own a second preference table, value=%v err=%v", preferenceTable, err)
	}

	digest := strings.Repeat("a", 64)
	err = database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, insertErr := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_chain
            (chain_no,root_task_no,tail_task_no) VALUES ('nchain_test','ntask_test','ntask_test')`); insertErr != nil {
			return insertErr
		}
		_, insertErr := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_task
            (task_no,chain_no,event_code,topic_code,channel_code,locale_code,business_object_type,
             business_object_no,body_content,content_digest)
            VALUES ('ntask_test','nchain_test','support.ticket_updated','support','in_app','zh-CN',
                    'support_ticket','ticket_test','body',?)`, digest)
		return insertErr
	})
	if err != nil {
		t.Fatalf("deferred notification-chain foreign keys rejected a valid linear root: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_notify_delivery_attempt
        (attempt_no,task_no,retry_sequence,attempt_fence,result_class,evidence_digest,started_at)
        VALUES ('attempt_1','ntask_test',0,'ntask_test:0','none',?,CURRENT_TIMESTAMP)`, digest); err != nil {
		t.Fatalf("insert first attempt fence: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_notify_delivery_attempt
        (attempt_no,task_no,retry_sequence,attempt_fence,result_class,evidence_digest,started_at)
        VALUES ('attempt_2','ntask_test',0,'ntask_test:0:duplicate','none',?,CURRENT_TIMESTAMP)`, digest); err == nil {
		t.Fatal("duplicate task retry sequence bypassed the persistent attempt fence")
	}

	uninstall := readNotificationSQL(t, filepath.Join("manifest", "sql", "uninstall", "001-notification-foundation.sql"))
	if _, err = database.Exec(ctx, uninstall); err != nil {
		t.Fatalf("uninstall notification schema with cyclic task links: %v", err)
	}
	remaining, err := database.GetValue(ctx, "SELECT to_regclass('plugin_idc_notify_delivery_task')")
	if err != nil || !remaining.IsNil() {
		t.Fatalf("notification uninstall left delivery task table, value=%v err=%v", remaining, err)
	}
}

func readNotificationSQL(t *testing.T, relative string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve notification test path")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", relative))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(content)
}

func newNotificationSchema(t *testing.T) string {
	t.Helper()
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	return "linapro_notify_test_" + hex.EncodeToString(value[:])
}

func appendNotificationDSN(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func quoteNotificationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
