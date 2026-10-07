package notificationpolicy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

func TestNotificationEventSinkClaimAckPostgreSQL(t *testing.T) {
	ctx, database := newNotificationEventTestDatabase(t)
	writer, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher, err := outbox.NewService(database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := NewEventSink("idc-core", writer)
	if err != nil {
		t.Fatal(err)
	}
	feed, err := NewEventFeed(dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	request := notificationcap.EventRequest{
		EventCode: "support.ticket_updated",
		Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact,
			CustomerNo: "customer-event", ContactNo: "contact-event"},
		BusinessObjectType: notificationcap.BusinessObjectSupportTicket,
		BusinessObjectNo:   "TKT_event", BusinessObjectVersion: 2,
		Variables:     map[string]string{"ticket_no": "TKT_event", "title": "Updated"},
		CorrelationNo: "TKT_event", IdempotencyKey: "support-event-TKT_event-v2-user7",
	}
	if _, err = sink.Enqueue(ctx, request); !bizerr.Is(err, outbox.CodeTransactionRequired) {
		t.Fatalf("notification event accepted outside business transaction: %v", err)
	}
	var receipt notificationcap.EventReceipt
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var enqueueErr error
		receipt, enqueueErr = sink.Enqueue(ctx, request)
		return enqueueErr
	}); err != nil {
		t.Fatalf("enqueue notification event in transaction: %v", err)
	}
	claimed, err := feed.Claim(ctx, notificationcap.EventClaimInput{WorkerID: "notify-test", Limit: 10})
	if err != nil || len(claimed) != 1 || claimed[0].EventNo != receipt.EventNo || claimed[0].Request.BusinessObjectNo != "TKT_event" {
		t.Fatalf("claim notification event: claimed=%+v err=%v", claimed, err)
	}
	acked, err := feed.Ack(ctx, claimed[0].Lease)
	if err != nil || acked.Status != string(outbox.StatusPublished) || acked.EventNo != receipt.EventNo {
		t.Fatalf("ack notification event: acked=%+v err=%v", acked, err)
	}
	claimed, err = feed.Claim(ctx, notificationcap.EventClaimInput{WorkerID: "notify-test", Limit: 10})
	if err != nil || len(claimed) != 0 {
		t.Fatalf("published notification event was claimed again: claimed=%+v err=%v", claimed, err)
	}
}

func newNotificationEventTestDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run notification event PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_notification_event_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		if strings.HasPrefix(schema, "linapro_notification_event_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		}
		_ = admin.Close(ctx)
	})
	if _, err = database.Exec(ctx, `
CREATE TABLE sys_dict_type (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(128) NOT NULL,
    status INT NOT NULL,
    is_builtin INT NOT NULL,
    allow_tenant_override BOOLEAN NOT NULL,
    remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL,
    dict_type VARCHAR(128) NOT NULL,
    label VARCHAR(128) NOT NULL,
    value VARCHAR(128) NOT NULL,
    sort INT NOT NULL,
    tag_style VARCHAR(32) NOT NULL DEFAULT '',
    status INT NOT NULL,
    is_builtin INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, dict_type, value)
)`); err != nil {
		t.Fatalf("create host dictionary fixtures: %v", err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join(root, "001-core-foundation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("execute core foundation migration: %v", err)
	}
	return ctx, database
}
