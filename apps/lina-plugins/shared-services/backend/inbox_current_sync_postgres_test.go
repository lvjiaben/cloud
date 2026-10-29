// PostgreSQL verifies the plugin TX protocol. The real host/UI is checked separately.
package backend

import (
	"context"
	"errors"
	"fmt"
	"github.com/gogf/gf/v2/database/gdb"
	usermsgv1 "lina-core/api/usermsg/v1"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/capability/notifycap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
	"lina-plugin-shared-services/backend/internal/service/inbox"
	"strconv"
	"sync"
	"testing"
)

type syncIdentity struct{}

func (syncIdentity) Current(ctx context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentFromContext(ctx)
}

type syncHost struct {
	notifycap.Service
	db           gdb.DB
	fail, badAck bool
}

func (h *syncHost) Send(ctx context.Context, in notifycap.SendInput) (*notifycap.SendResult, error) {
	c := bizctxcap.CurrentFromContext(ctx)
	if c.UserID <= 0 || len(in.Recipients) != 1 || in.Recipients[0] != strconv.Itoa(c.UserID) || in.SenderUserID != 0 || in.SourceType != usermsgv1.SourceTypePlugin {
		return nil, errors.New("identity boundary violated")
	}
	if in.Payload["callerPluginId"] != "test-consumer" || in.Payload["actionUrl"] != "/idc/renewal" {
		return nil, errors.New("provenance missing")
	}
	var id int64
	err := h.db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var e error
		id, e = h.db.Model("test_host_inbox_delivery").Ctx(txCtx).Data(gdb.Map{"recipient": c.UserID, "source_id": in.SourceID, "title": in.Title, "content": in.Content}).InsertAndGetId()
		return e
	})
	if err != nil {
		return nil, err
	}
	if h.fail {
		return nil, errors.New("failure after nested write")
	}
	count := 1
	if h.badAck {
		count = 0
	}
	return &notifycap.SendResult{MessageID: notifycap.MessageID(strconv.FormatInt(id, 10)), DeliveryCount: count}, nil
}

type syncLostCommit struct{ gdb.DB }

func (d syncLostCommit) Transaction(ctx context.Context, f func(context.Context, gdb.TX) error) error {
	if err := d.DB.Transaction(ctx, f); err != nil {
		return err
	}
	return errors.New("commit acknowledgment lost")
}
func syncContext(t *testing.T, id int) context.Context {
	return bizctxcap.WithCurrentContext(t.Context(), bizctxcap.CurrentContext{UserID: id, DataScope: 1})
}
func syncFixture(t *testing.T) (gdb.DB, *syncHost, inboxcap.Service) {
	t.Helper()
	db, cleanup := openSharedServicesPostgreSQL(t)
	t.Cleanup(cleanup)
	if _, e := db.Exec(t.Context(), `CREATE TABLE test_host_inbox_delivery(id BIGSERIAL PRIMARY KEY,recipient BIGINT NOT NULL,source_id TEXT NOT NULL,title TEXT NOT NULL,content TEXT NOT NULL)`); e != nil {
		t.Fatal(e)
	}
	svc, e := inbox.New(db)
	if e != nil {
		t.Fatal(e)
	}
	pub, e := inbox.Bind(svc, "test-consumer")
	if e != nil {
		t.Fatal(e)
	}
	return db, &syncHost{db: db}, pub
}
func syncPublish(t *testing.T, pub inboxcap.Service, id int64, key string) {
	t.Helper()
	_, e := pub.Publish(t.Context(), inboxcap.PublishRequest{RecipientUserID: id, SourceType: "billing", SourceNo: key, IdempotencyKey: key, Title: "Renewal pending", Body: "Payment pending.", Category: "billing", ActionURL: "/idc/renewal"})
	if e != nil {
		t.Fatal(e)
	}
}
func syncCount(t *testing.T, db gdb.DB, table string) int {
	t.Helper()
	n, e := db.Model(table).Ctx(t.Context()).Count()
	if e != nil {
		t.Fatal(e)
	}
	return n
}
func TestInboxCurrentSyncSelfBoundaryPostgreSQL(t *testing.T) {
	db, host, pub := syncFixture(t)
	for i := 0; i < 3; i++ {
		syncPublish(t, pub, 41, fmt.Sprintf("self-%d", i))
	}
	syncPublish(t, pub, 42, "other")
	svc, e := inbox.NewCurrentSync(db, host, syncIdentity{})
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []bizctxcap.CurrentContext{{}, {UserID: 41, TenantID: 3, DataScope: 1}, {UserID: 41, ActingUserID: 42, DataScope: 1}, {UserID: 41, ActingAsTenant: true, DataScope: 1}, {UserID: 41, IsImpersonation: true, DataScope: 1}} {
		if _, e = svc.CurrentSync(bizctxcap.WithCurrentContext(t.Context(), c), 0); e == nil {
			t.Fatalf("invalid identity accepted %+v", c)
		}
	}
	if syncCount(t, db, "test_host_inbox_delivery") != 0 {
		t.Fatal("denied identity wrote messages")
	}
	for _, limit := range []int{-1, 51} {
		if _, e = svc.CurrentSync(syncContext(t, 41), limit); e == nil {
			t.Fatal("invalid limit accepted")
		}
	}
	for _, want := range []struct{ sent, pending int }{{2, 1}, {1, 0}, {0, 0}} {
		r, err := svc.CurrentSync(syncContext(t, 41), 2)
		if err != nil || r.SyncedCount != want.sent || r.PendingCount != want.pending {
			t.Fatalf("result=%+v error=%v", r, err)
		}
	}
	r, e := svc.CurrentSync(syncContext(t, 42), 0)
	if e != nil || r.SyncedCount != 1 || r.PendingCount != 0 {
		t.Fatalf("other user=%+v error=%v", r, e)
	}
	if syncCount(t, db, "test_host_inbox_delivery") != 4 {
		t.Fatal("incorrect host deliveries")
	}
	inv := inbox.NewInvoker()
	if e = inv.Bind(svc); e != nil {
		t.Fatal(e)
	}
	for _, payload := range []string{`{"limit":1,"recipientUserId":42}`, `{"limit":1} {}`} {
		if _, e = inv.Invoke(syncContext(t, 41), capregistry.Invocation{CallerPluginID: "test-consumer", OwnerPluginID: inboxcap.OwnerPluginID, Service: inboxcap.ServiceName, Version: inboxcap.ContractVersion, Method: inboxcap.MethodCurrentSync, Payload: []byte(payload)}); e == nil {
			t.Fatal("guest override accepted")
		}
	}
	out, e := inv.Invoke(syncContext(t, 41), capregistry.Invocation{CallerPluginID: "test-consumer", OwnerPluginID: inboxcap.OwnerPluginID, Service: inboxcap.ServiceName, Version: inboxcap.ContractVersion, Method: inboxcap.MethodCurrentSync, Payload: []byte(`{"limit":1}`)})
	if e != nil || string(out.Payload) != `{"syncedCount":0,"pendingCount":0}` {
		t.Fatalf("public current sync=%v error=%v", out, e)
	}
}
func TestInboxCurrentSyncConcurrentInstancesPostgreSQL(t *testing.T) {
	db, host, pub := syncFixture(t)
	for i := 0; i < 12; i++ {
		syncPublish(t, pub, 41, fmt.Sprintf("concurrent-%d", i))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc, e := inbox.NewCurrentSync(db, host, syncIdentity{})
			if e == nil {
				_, e = svc.CurrentSync(syncContext(t, 41), 50)
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if syncCount(t, db, "test_host_inbox_delivery") != 12 {
		t.Fatal("duplicate or lost host messages")
	}
	n, e := db.GetValue(t.Context(), `SELECT count(DISTINCT source_id) FROM test_host_inbox_delivery`)
	if e != nil || n.Int() != 12 {
		t.Fatalf("duplicate source %v", e)
	}
}
func TestInboxCurrentSyncRollbackAndUnknownCommitPostgreSQL(t *testing.T) {
	db, host, pub := syncFixture(t)
	syncPublish(t, pub, 41, "retry")
	svc, e := inbox.NewCurrentSync(db, host, syncIdentity{})
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"failure", "bad_ack"} {
		host.fail = mode == "failure"
		host.badAck = mode == "bad_ack"
		if _, e = svc.CurrentSync(syncContext(t, 41), 1); e == nil {
			t.Fatal("failure hidden")
		}
		if syncCount(t, db, "test_host_inbox_delivery") != 0 || syncCount(t, db, "plugin_shared_services_inbox_delivery_evidence") != 1 {
			t.Fatal("failed host delivery escaped rollback")
		}
	}
	host.fail = false
	host.badAck = false
	uncertain, e := inbox.NewCurrentSync(syncLostCommit{db}, host, syncIdentity{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = uncertain.CurrentSync(syncContext(t, 41), 1); e == nil {
		t.Fatal("unknown commit hidden")
	}
	r, e := svc.CurrentSync(syncContext(t, 41), 1)
	if e != nil || r.SyncedCount != 0 || r.PendingCount != 0 || syncCount(t, db, "test_host_inbox_delivery") != 1 {
		t.Fatalf("unknown commit replay=%+v error=%v", r, e)
	}
}

func TestInboxCurrentSyncCommitFailurePostgreSQL(t *testing.T) {
	db, host, pub := syncFixture(t)
	syncPublish(t, pub, 41, "commit-failure")
	if _, err := db.Exec(t.Context(), `CREATE FUNCTION reject_test_commit() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test deferred commit failure'; END $$;
 CREATE CONSTRAINT TRIGGER test_commit_failure AFTER INSERT ON test_host_inbox_delivery DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_test_commit()`); err != nil {
		t.Fatal(err)
	}
	svc, err := inbox.NewCurrentSync(db, host, syncIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.CurrentSync(syncContext(t, 41), 1); err == nil {
		t.Fatal("deferred commit failure hidden")
	}
	if syncCount(t, db, "test_host_inbox_delivery") != 0 || syncCount(t, db, "plugin_shared_services_inbox_delivery_evidence") != 1 {
		t.Fatal("commit failure left delivery evidence")
	}
	if _, err = db.Exec(t.Context(), `DROP TRIGGER test_commit_failure ON test_host_inbox_delivery`); err != nil {
		t.Fatal(err)
	}
	result, err := svc.CurrentSync(syncContext(t, 41), 1)
	if err != nil || result.SyncedCount != 1 || syncCount(t, db, "test_host_inbox_delivery") != 1 {
		t.Fatalf("retry after failed commit=%+v err=%v", result, err)
	}
}

func TestInboxCurrentSyncEscapesPlainTextPostgreSQL(t *testing.T) {
	db, host, publisher := syncFixture(t)
	body := "<img src=x onerror=alert(1)>\nInvoice <b>pending</b> & \"quoted\""
	request := inboxcap.PublishRequest{RecipientUserID: 41, SourceType: "billing", SourceNo: "unsafe-body", IdempotencyKey: "unsafe-body", Title: "Plain <title>", Body: body, Category: "billing", ActionURL: "/idc/renewal"}
	intent, err := publisher.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	var before struct{ Body, ContentDigest string }
	if err = db.Model("plugin_shared_services_inbox_message").Ctx(t.Context()).Where("message_no", intent.MessageNo).Fields("body,content_digest").Scan(&before); err != nil {
		t.Fatal(err)
	}
	svc, err := inbox.NewCurrentSync(db, host, syncIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.CurrentSync(syncContext(t, 41), 1)
	if err != nil || result.SyncedCount != 1 {
		t.Fatalf("escaped delivery=%+v err=%v", result, err)
	}
	var delivered struct{ Title, Content string }
	if err = db.Model("test_host_inbox_delivery").Ctx(t.Context()).Where("source_id", intent.MessageNo).Fields("title,content").Scan(&delivered); err != nil {
		t.Fatal(err)
	}
	want := "&lt;img src=x onerror=alert(1)&gt;<br>Invoice &lt;b&gt;pending&lt;/b&gt; &amp; &#34;quoted&#34;"
	if delivered.Content != want || delivered.Title != request.Title {
		t.Fatalf("unsafe content/title: %+v", delivered)
	}
	var after struct{ Body, ContentDigest string }
	if err = db.Model("plugin_shared_services_inbox_message").Ctx(t.Context()).Where("message_no", intent.MessageNo).Fields("body,content_digest").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after || after.Body != body || after.ContentDigest == "" {
		t.Fatal("HTML projection mutated frozen intent or digest")
	}
	replay, err := publisher.Publish(t.Context(), request)
	if err != nil || !replay.Reused {
		t.Fatalf("escaped projection broke publish idempotency: %+v err=%v", replay, err)
	}
}

func TestInboxCurrentSyncCustomerWithoutStaffScopePostgreSQL(t *testing.T) {
	db, host, pub := syncFixture(t)
	svc, err := inbox.NewCurrentSync(db, host, syncIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	syncPublish(t, pub, 42, "other-customer-scope")
	for index, current := range []bizctxcap.CurrentContext{{UserID: 41}, {UserID: 41, DataScope: 99, DataScopeUnsupported: true, UnsupportedDataScope: 99}} {
		syncPublish(t, pub, 41, fmt.Sprintf("customer-scope-%d", index))
		result, syncErr := svc.CurrentSync(bizctxcap.WithCurrentContext(t.Context(), current), 50)
		if syncErr != nil || result.SyncedCount != 1 || result.PendingCount != 0 {
			t.Fatalf("customer self sync=%+v err=%v", result, syncErr)
		}
	}
	others, err := db.Model("test_host_inbox_delivery").Ctx(t.Context()).Where("recipient", 42).Count()
	if err != nil || others != 0 {
		t.Fatalf("customer self sync reached another user: %d %v", others, err)
	}
}
