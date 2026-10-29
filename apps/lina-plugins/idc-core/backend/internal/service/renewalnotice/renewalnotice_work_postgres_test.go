// These tests exercise actual accessory work-item migrations and the real
// notificationpolicy/outbox sink in disposable PostgreSQL schemas. Host identity
// and authorization capabilities are local test boundaries; no provider is called.
package renewalnotice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/dialect"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/notificationpolicy"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

// noticeIdentity uses the current context rather than accepting a recipient ID.
type noticeIdentity struct{}

func (noticeIdentity) Current(ctx context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentFromContext(ctx)
}

// noticeAudience projects a local host boundary for the actual context user.
type noticeAudience struct {
	code     principal.AudienceCode
	status   principal.AudienceStatus
	customer string
	mismatch bool
	err      error
}

func (a noticeAudience) Current(ctx context.Context) (*principal.Audience, error) {
	if a.err != nil {
		return nil, a.err
	}
	id := bizctxcap.CurrentFromContext(ctx).UserID
	if a.mismatch {
		id++
	}
	return &principal.Audience{UserID: id, Code: a.code, Status: a.status, CustomerNo: a.customer}, nil
}

// noticeAuthorization models permission outcomes, including revocation during read.
type noticeAuthorization struct {
	authz.Service
	grant, wildcard, admin bool
	err                    error
	revokeAfter            int32
	calls                  atomic.Int32
}

func (a *noticeAuthorization) BatchHasPermissions(_ context.Context, _ []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	n := a.calls.Add(1)
	grant := a.grant
	if a.revokeAfter > 0 && n > a.revokeAfter {
		grant = false
	}
	return map[authz.PermissionKey]bool{authz.PermissionKey(Permission): grant, authz.PermissionKey(idccap.HostPermissionWildcard): a.wildcard}, a.err
}
func (a *noticeAuthorization) IsPlatformAdmin(ctx context.Context, id authz.UserID) (bool, error) {
	if string(id) != fmt.Sprint(bizctxcap.CurrentFromContext(ctx).UserID) {
		return false, errors.New("authorization checked a different user")
	}
	return a.admin, a.err
}

// noticeContext creates one authenticated installation-staff context.
func noticeContext(id int) context.Context {
	return bizctxcap.WithCurrentContext(context.Background(), bizctxcap.CurrentContext{UserID: id, DataScope: 1})
}

// noticeService binds a real transaction-local event sink to an isolated database.
func noticeService(t *testing.T, database gdb.DB, authority *noticeAuthorization, audience noticeAudience) Service {
	t.Helper()
	writer, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sink, err := notificationpolicy.NewEventSink("idc-core", writer)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(database, noticeIdentity{}, audience, authority, sink, &noticeInbox{})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

// noticeInbox is the public owner boundary. Actual host delivery is verified by
// shared-services tests and the authenticated runtime test, not this substitute.
type noticeInbox struct {
	err    error
	calls  int
	userID int
}

func (inbox *noticeInbox) CurrentSync(ctx context.Context, limit int) (*inboxcap.CurrentSyncResult, error) {
	if limit != maximumPageSize {
		return nil, errors.New("inbox sync limit changed")
	}
	inbox.calls++
	inbox.userID = bizctxcap.CurrentFromContext(ctx).UserID
	return &inboxcap.CurrentSyncResult{}, inbox.err
}

// noticeDatabase uses production DDL for the work queue and outbox; only unrelated
// parent aggregate tables are minimal fixtures. Every invocation owns its cleanup.
func noticeDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "renewalnotice_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + sep + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Error(err)
		}
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if _, err = database.Exec(context.Background(), noticeBoundarySQL); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration directory")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql"))
	for _, name := range []string{"058-server-accessory-pricing.sql", "060-server-accessory-entitlement.sql", "151-native-server-accessory-lifecycle.sql", "001-core-foundation.sql"} {
		if strings.HasPrefix(name, "060") {
			if _, err = database.Exec(context.Background(), `ALTER TABLE plugin_idc_core_server_accessory_quote ADD COLUMN expected_service_version BIGINT NOT NULL DEFAULT 1`); err != nil {
				t.Fatal(err)
			}
		}
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, statement := range dialect.SplitSQLStatements(string(content)) {
			if name == "001-core-foundation.sql" && !strings.Contains(statement, "CREATE TABLE IF NOT EXISTS plugin_idc_core_outbox_event (") {
				continue
			}
			if _, err = database.Exec(context.Background(), statement); err != nil {
				t.Fatalf("apply %s: %v", name, err)
			}
		}
	}
	if _, err = database.Exec(context.Background(), `INSERT INTO plugin_idc_core_customer_subject VALUES('customer');
 INSERT INTO plugin_idc_core_service_instance VALUES(0,'service','customer','USD');
 INSERT INTO plugin_idc_core_business_operation(operation_no) VALUES('create');
 INSERT INTO plugin_idc_core_server_accessory_entitlement
 (entitlement_no,customer_no,service_no,create_operation_no,object_no,accessory_kind,billing_mode_code,status_code,currency_code,period_unit_code,period_count,baseline_object_nos,baseline_snapshot_no,baseline_observed_at)
 VALUES('accessory','customer','service','create','object','snapshot','free','active','USD','month',1,'[]','baseline',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	return database
}

// noticeBoundarySQL contains only related parents required by the real migrations.
const noticeBoundarySQL = `
CREATE TABLE plugin_idc_core_customer_subject(customer_no VARCHAR(40) PRIMARY KEY);
CREATE TABLE plugin_idc_core_service_instance(tenant_id INT,service_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40),currency_code CHAR(3),UNIQUE(service_no,customer_no,currency_code));
CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40),currency_code CHAR(3),entitlement_owner_id TEXT);
CREATE TABLE plugin_idc_core_business_operation(tenant_id INT,operation_no VARCHAR(40) PRIMARY KEY,subject_type_code TEXT,subject_no VARCHAR(40),customer_no VARCHAR(40),currency_code CHAR(3),operation_type_code TEXT,status_code TEXT,service_no VARCHAR(40),resource_owner_code TEXT,provider_task_no TEXT,execution_started_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_billing_invoice(invoice_no VARCHAR(40) PRIMARY KEY);
CREATE TABLE plugin_idc_core_refund_request(tenant_id INT,order_no VARCHAR(40),invoice_no VARCHAR(40),customer_no VARCHAR(40),currency_code CHAR(3),refund_no VARCHAR(40),status_code TEXT);
CREATE TABLE plugin_idc_core_billing_refund_allocation(tenant_id INT,invoice_no VARCHAR(40),direction_code TEXT,refund_source_reference TEXT,original_refund_allocation_no VARCHAR(40));
`

// noticeWork inserts one independently identified billing/work fixture; timestamp
// ties deliberately exercise the stable secondary id ordering.
func noticeWork(t *testing.T, database gdb.DB, index, tenant int, permission, status string) {
	t.Helper()
	number := fmt.Sprintf("%03d", index)
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_billing_invoice VALUES(?)`, "invoice-"+number); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_server_accessory_renewal_work_item
 (tenant_id,work_item_no,entitlement_no,customer_no,service_no,invoice_no,required_permission,status_code,reason_code,created_at,updated_at,resolved_at)
 VALUES(?,?,'accessory','customer','service',?,?,?,'deletion_invoice_pending','2026-10-01T00:00:00Z','2026-10-01T00:00:00Z',CASE WHEN ?='resolved' THEN CURRENT_TIMESTAMP ELSE NULL END)`, tenant, "work-"+number, "invoice-"+number, permission, status, status); err != nil {
		t.Fatal(err)
	}
}

// noticeCount reads exact durable outbox rows, not a mock enqueue call count.
func noticeCount(t *testing.T, database gdb.DB, want int) {
	t.Helper()
	count, err := database.Model(dao.OutboxEvent.Table()).Ctx(context.Background()).Count()
	if err != nil || count != want {
		t.Fatalf("outbox count=%d want=%d error=%v", count, want, err)
	}
}

// TestRenewalNoticePermissionsPostgreSQL proves fail-closed identity and scope
// checks for both reading and notification writes against populated real tables.
func TestRenewalNoticePermissionsPostgreSQL(t *testing.T) {
	database := noticeDatabase(t)
	noticeWork(t, database, 1, 0, Permission, "open")
	for _, test := range []struct {
		name     string
		context  bizctxcap.CurrentContext
		audience noticeAudience
		auth     *noticeAuthorization
	}{
		{name: "permission denied", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1}, audience: noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive}, auth: &noticeAuthorization{}},
		{name: "customer", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1}, audience: noticeAudience{code: principal.AudienceCustomer, status: principal.AudienceStatusActive, customer: "customer"}, auth: &noticeAuthorization{grant: true}},
		{name: "revoked audience", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1}, audience: noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusRevoked}, auth: &noticeAuthorization{grant: true}},
		{name: "mismatched audience", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1}, audience: noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive, mismatch: true}, auth: &noticeAuthorization{grant: true}},
		{name: "impersonation", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1, IsImpersonation: true}, auth: &noticeAuthorization{admin: true}},
		{name: "acting staff", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1, ActingUserID: 42}, auth: &noticeAuthorization{admin: true}},
		{name: "tenant delegation", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1, ActingAsTenant: true}, auth: &noticeAuthorization{admin: true}},
		{name: "other tenant", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1, TenantID: 1}, auth: &noticeAuthorization{admin: true}},
		{name: "unsupported scope", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1, DataScopeUnsupported: true}, auth: &noticeAuthorization{admin: true}},
		{name: "self scope", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 5}, auth: &noticeAuthorization{admin: true}},
		{name: "department scope", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 3}, auth: &noticeAuthorization{admin: true}},
		{name: "missing scope", context: bizctxcap.CurrentContext{UserID: 41}, auth: &noticeAuthorization{admin: true}},
		{name: "anonymous", context: bizctxcap.CurrentContext{DataScope: 1}, auth: &noticeAuthorization{admin: true}},
		{name: "authorization error", context: bizctxcap.CurrentContext{UserID: 41, DataScope: 1}, audience: noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive}, auth: &noticeAuthorization{err: errors.New("authority unavailable")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := noticeService(t, database, test.auth, test.audience)
			ctx := bizctxcap.WithCurrentContext(context.Background(), test.context)
			if _, err := service.List(ctx, 1, 10); !bizerr.Is(err, CodeDenied) {
				t.Fatalf("list did not deny: %v", err)
			}
			if _, err := service.Sync(ctx, "zh-CN"); !bizerr.Is(err, CodeDenied) {
				t.Fatalf("sync did not deny: %v", err)
			}
		})
	}
	noticeCount(t, database, 0)
}

// TestRenewalNoticeListPostgreSQL verifies one consistent filter for total and
// bounded pages, including empty pages and records outside tenant/grant/status.
func TestRenewalNoticeListPostgreSQL(t *testing.T) {
	database := noticeDatabase(t)
	for i := 1; i <= 3; i++ {
		noticeWork(t, database, i, 0, Permission, "open")
	}
	noticeWork(t, database, 4, 1, Permission, "open")
	noticeWork(t, database, 5, 0, "other:permission", "open")
	noticeWork(t, database, 6, 0, Permission, "resolved")
	audience := noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive}
	service := noticeService(t, database, &noticeAuthorization{grant: true}, audience)
	first, err := service.List(noticeContext(41), 1, 2)
	if err != nil || first.Total != 3 || len(first.Items) != 2 || first.Items[0].InvoiceNo != "invoice-003" || first.Items[1].InvoiceNo != "invoice-002" || first.Items[0].SubjectNo != "service" || first.Items[0].ReasonCode != "deletion_invoice_pending" {
		t.Fatalf("first page=%+v error=%v", first, err)
	}
	second, err := service.List(noticeContext(41), 2, 2)
	if err != nil || second.Total != 3 || len(second.Items) != 1 || second.Items[0].InvoiceNo != "invoice-001" {
		t.Fatalf("second page=%+v error=%v", second, err)
	}
	empty, err := service.List(noticeContext(41), 3, 2)
	if err != nil || empty.Total != 3 || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("empty page=%+v error=%v", empty, err)
	}
	for _, page := range [][2]int{{0, 10}, {1, 0}, {1, 51}, {1000001, 10}} {
		if _, err = service.List(noticeContext(41), page[0], page[1]); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("invalid page accepted: %v %v", page, err)
		}
	}
	for _, authority := range []*noticeAuthorization{{wildcard: true}, {admin: true}} {
		if _, err = noticeService(t, database, authority, audience).List(noticeContext(41), 1, 2); err != nil {
			t.Fatalf("governed wildcard/admin denied: %v", err)
		}
	}
	noticeCount(t, database, 0)
}

// TestRenewalNoticeCurrentInboxRecoveryPostgreSQL preserves an already-committed
// business intent when host delivery fails, and drains it after work resolves.
func TestRenewalNoticeCurrentInboxRecoveryPostgreSQL(t *testing.T) {
	database := noticeDatabase(t)
	noticeWork(t, database, 1, 0, Permission, "open")
	authority := &noticeAuthorization{grant: true}
	service := noticeService(t, database, authority, noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive}).(*serviceImpl)
	inbox := &noticeInbox{err: errors.New("host inbox unavailable")}
	service.inbox = inbox
	if result, err := service.Sync(noticeContext(41), "zh-CN"); result != nil || !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("inbox failure was hidden: result=%+v err=%v", result, err)
	}
	// The business event was committed before host delivery. Recovery must reuse
	// that immutable event instead of creating another intent or losing it.
	noticeCount(t, database, 1)
	inbox.err = nil
	result, err := service.Sync(noticeContext(41), "zh-CN")
	if err != nil || result.EventNo == "" || inbox.calls != 2 || inbox.userID != 41 {
		t.Fatalf("current actor inbox recovery failed: result=%+v inbox=%+v err=%v", result, inbox, err)
	}
	noticeCount(t, database, 1)
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_core_server_accessory_renewal_work_item SET status_code='resolved',resolved_at=CURRENT_TIMESTAMP`); err != nil {
		t.Fatal(err)
	}
	result, err = service.Sync(noticeContext(41), "zh-CN")
	if err != nil || result.Total != 0 || result.EventNo != "" || inbox.calls != 3 {
		t.Fatalf("empty queue did not drain prior messages: result=%+v err=%v", result, err)
	}
	authority.grant = false
	if _, err = service.Sync(noticeContext(41), "zh-CN"); !bizerr.Is(err, CodeDenied) || inbox.calls != 3 {
		t.Fatalf("revoked identity reached inbox: calls=%d err=%v", inbox.calls, err)
	}
}

// TestRenewalNoticeSyncPostgreSQL proves durable per-user, per-observation
// idempotency across independent service instances and simultaneous requests.
func TestRenewalNoticeSyncPostgreSQL(t *testing.T) {
	database := noticeDatabase(t)
	for i := 1; i <= 55; i++ {
		noticeWork(t, database, i, 0, Permission, "open")
	}
	audience := noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive}
	var wait sync.WaitGroup
	events := make(chan string, 8)
	failures := make(chan error, 8)
	for range 8 {
		service := noticeService(t, database, &noticeAuthorization{grant: true}, audience)
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.Sync(noticeContext(41), "zh-CN")
			if err != nil {
				failures <- err
				return
			}
			if result.Total != 55 {
				failures <- fmt.Errorf("total=%d", result.Total)
				return
			}
			events <- result.EventNo
		}()
	}
	wait.Wait()
	close(events)
	close(failures)
	for err := range failures {
		t.Errorf("concurrent sync: %v", err)
	}
	if t.Failed() {
		return
	}
	first := ""
	for event := range events {
		if event == "" {
			t.Fatal("missing durable event")
		}
		if first == "" {
			first = event
		} else if first != event {
			t.Fatal("same workset duplicated across instances")
		}
	}
	noticeCount(t, database, 1)
	service := noticeService(t, database, &noticeAuthorization{grant: true}, audience)
	other, err := service.Sync(noticeContext(42), "zh-CN")
	if err != nil || other.EventNo == first {
		t.Fatalf("second staff did not get own event: %+v %v", other, err)
	}
	noticeCount(t, database, 2)
	// A new item with a tied timestamp sorts first by id even beyond 50 items.
	noticeWork(t, database, 56, 0, Permission, "open")
	next, err := service.Sync(noticeContext(41), "zh-CN")
	if err != nil || next.EventNo == first || next.Total != 56 {
		t.Fatalf("new workset: %+v %v", next, err)
	}
	replay, err := service.Sync(noticeContext(41), "zh-CN")
	if err != nil || replay.EventNo != next.EventNo {
		t.Fatalf("new-set replay: %+v %v", replay, err)
	}
	noticeCount(t, database, 3)
	var rows []struct{ Payload string }
	if err = database.Model(dao.OutboxEvent.Table()).Ctx(context.Background()).OrderAsc("id").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		var envelope struct {
			SourcePlugin string
			Request      notificationcap.EventRequest
		}
		if err = json.Unmarshal([]byte(row.Payload), &envelope); err != nil {
			t.Fatal(err)
		}
		wantUser, wantCount, wantInvoice := 41, "55", "invoice-055"
		if i == 1 {
			wantUser = 42
		}
		if i == 2 {
			wantCount = "56"
			wantInvoice = "invoice-056"
		}
		request := envelope.Request
		if envelope.SourcePlugin != "idc-core" || request.Recipient.StaffUserID != wantUser || request.Recipient.Kind != notificationcap.RecipientStaffUser || request.Recipient.CustomerNo != "" || request.Recipient.ContactNo != "" || request.EventCode != attentionEvent || request.Variables["pending_count"] != wantCount || request.Variables["invoice_no"] != wantInvoice || request.Variables["locale_code"] != "zh-CN" {
			t.Fatalf("wrong durable staff intent: %+v", envelope)
		}
	}
}

// noticeFailAfterEnqueue injects a failure after the actual sink has written.
type noticeFailAfterEnqueue struct{ notificationcap.EventSink }

func (s noticeFailAfterEnqueue) Enqueue(ctx context.Context, input notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	receipt, err := s.EventSink.Enqueue(ctx, input)
	if err != nil {
		return receipt, err
	}
	return receipt, errors.New("fail after durable insert")
}

// TestRenewalNoticeSyncFailurePostgreSQL proves empty queues do not emit events,
// errors roll back real outbox writes, and post-read revocation fails closed.
func TestRenewalNoticeSyncFailurePostgreSQL(t *testing.T) {
	database := noticeDatabase(t)
	audience := noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive}
	service := noticeService(t, database, &noticeAuthorization{grant: true}, audience)
	result, err := service.Sync(noticeContext(41), "en-US")
	if err != nil || result.Total != 0 || result.EventNo != "" {
		t.Fatalf("empty sync: %+v %v", result, err)
	}
	noticeCount(t, database, 0)
	if _, err = service.Sync(noticeContext(41), "invalid"); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid locale accepted: %v", err)
	}
	noticeWork(t, database, 1, 0, Permission, "open")
	service.(*serviceImpl).events = noticeFailAfterEnqueue{service.(*serviceImpl).events}
	if result, err = service.Sync(noticeContext(41), "en-US"); result != nil || !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("enqueue failure escaped: %+v %v", result, err)
	}
	noticeCount(t, database, 0)
	revoked := noticeService(t, database, &noticeAuthorization{grant: true, revokeAfter: 1}, audience)
	if _, err = revoked.Sync(noticeContext(41), "en-US"); !bizerr.Is(err, CodeDenied) {
		t.Fatalf("post-read revocation not checked: %v", err)
	}
	noticeCount(t, database, 0)
	healthy := noticeService(t, database, &noticeAuthorization{grant: true}, audience)
	if _, err = healthy.Sync(noticeContext(41), "en-US"); err != nil {
		t.Fatal(err)
	}
	noticeCount(t, database, 1)
}

// noticeSnapshotProbe interleaves a committed writer after PostgreSQL fixes the
// read snapshot. It delegates all service queries to the real database.
type noticeSnapshotProbe struct {
	gdb.DB
	afterSnapshot func()
}

func (d noticeSnapshotProbe) TransactionWithOptions(ctx context.Context, opts gdb.TxOptions, run func(context.Context, gdb.TX) error) error {
	return d.DB.TransactionWithOptions(ctx, opts, func(ctx context.Context, tx gdb.TX) error {
		isolation, err := d.DB.GetValue(ctx, "SHOW transaction_isolation")
		if err != nil {
			return err
		}
		readonly, err := d.DB.GetValue(ctx, "SHOW transaction_read_only")
		if err != nil {
			return err
		}
		if isolation.String() != "repeatable read" || readonly.String() != "on" {
			return fmt.Errorf("unexpected snapshot transaction: %s readonly=%s", isolation, readonly)
		}
		if _, err = d.DB.GetValue(ctx, "SELECT count(*) FROM plugin_idc_core_server_accessory_renewal_work_item"); err != nil {
			return err
		}
		d.afterSnapshot()
		return run(ctx, tx)
	})
}

// TestRenewalNoticeSnapshotPostgreSQL proves count and rows retain one observed
// work set while another transaction commits a newly queued item.
func TestRenewalNoticeSnapshotPostgreSQL(t *testing.T) {
	database := noticeDatabase(t)
	noticeWork(t, database, 1, 0, Permission, "open")
	audience := noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive}
	probe := noticeSnapshotProbe{DB: database, afterSnapshot: func() { noticeWork(t, database, 2, 0, Permission, "open") }}
	service := noticeService(t, probe, &noticeAuthorization{grant: true}, audience)
	page, err := service.List(noticeContext(41), 1, 10)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].InvoiceNo != "invoice-001" {
		t.Fatalf("snapshot tore after committed writer: %+v %v", page, err)
	}
	current := noticeService(t, database, &noticeAuthorization{grant: true}, audience)
	page, err = current.List(noticeContext(41), 1, 10)
	if err != nil || page.Total != 2 || len(page.Items) != 2 || page.Items[0].InvoiceNo != "invoice-002" {
		t.Fatalf("next snapshot missed committed work: %+v %v", page, err)
	}
}

// TestRenewalNoticeNativeSubjectPostgreSQL verifies the real 151 subject guard
// and generated projection support native entitlements without legacy service IDs.
func TestRenewalNoticeNativeSubjectPostgreSQL(t *testing.T) {
	database := noticeDatabase(t)
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'native','customer','USD','idc-resource');
 INSERT INTO plugin_idc_core_business_operation(tenant_id,operation_no,subject_type_code,subject_no,customer_no,currency_code,operation_type_code,status_code)
 VALUES(0,'create-native','entitlement','native','customer','USD','addon','succeeded');
 INSERT INTO plugin_idc_core_billing_invoice VALUES('invoice-native');
 INSERT INTO plugin_idc_core_server_accessory_entitlement
 (entitlement_no,customer_no,subject_type_code,subject_no,create_operation_no,object_no,accessory_kind,billing_mode_code,status_code,currency_code,period_unit_code,period_count,current_period_start_at,current_period_end_at,baseline_object_nos,baseline_snapshot_no,baseline_observed_at)
 VALUES('native-accessory','customer','entitlement','native','create-native','native-object','backup','paid','payment_pending','USD','month',1,'2026-09-01','2026-10-01','[]','baseline',CURRENT_TIMESTAMP);
 INSERT INTO plugin_idc_core_server_accessory_renewal_work_item
 (work_item_no,entitlement_no,customer_no,subject_type_code,subject_no,invoice_no,required_permission,status_code,reason_code,created_at,updated_at)
 VALUES('native-work','native-accessory','customer','entitlement','native','invoice-native','idc:admin:resource:accessory-renewal:handle','open','balance_payment_failed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	service := noticeService(t, database, &noticeAuthorization{grant: true}, noticeAudience{code: principal.AudienceStaff, status: principal.AudienceStatusActive})
	page, err := service.List(noticeContext(41), 1, 10)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].SubjectNo != "native" || page.Items[0].InvoiceNo != "invoice-native" || page.Items[0].ReasonCode != "balance_payment_failed" {
		t.Fatalf("native work projection: %+v %v", page, err)
	}
	result, err := service.Sync(noticeContext(41), "en-US")
	if err != nil || result.EventNo == "" || result.Total != 1 {
		t.Fatalf("native work event: %+v %v", result, err)
	}
	noticeCount(t, database, 1)
}
