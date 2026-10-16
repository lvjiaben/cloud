// This file validates durable trial expiry against isolated PostgreSQL tables.
// Only the external resource and notification ports are substituted; task leases,
// transactions, ownership predicates and entitlement updates use real SQL.

package triallifecycle

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

// trialResource records immutable requests and returns a controlled real-port projection.
type trialResource struct {
	status         lifecycleresourcecap.TaskStatus
	creates, reads int
	requests       []lifecycleresourcecap.TaskRequest
	unavailable    bool
}

func (r *trialResource) CreateTask(_ context.Context, request lifecycleresourcecap.TaskRequest) (*lifecycleresourcecap.Task, error) {
	r.creates++
	r.requests = append(r.requests, request)
	return r.result(request)
}
func (r *trialResource) ReadTask(_ context.Context, request lifecycleresourcecap.TaskReadRequest) (*lifecycleresourcecap.Task, error) {
	r.reads++
	for _, original := range r.requests {
		if request.Request == original && request.ProviderTaskNo == "provider-"+original.LifecycleTaskNo {
			return r.result(original)
		}
	}
	return nil, gerror.New("changed immutable resource request")
}
func (r *trialResource) result(request lifecycleresourcecap.TaskRequest) (*lifecycleresourcecap.Task, error) {
	if r.unavailable {
		return nil, gerror.New("resource unavailable")
	}
	result := &lifecycleresourcecap.Task{ProviderTaskNo: "provider-" + request.LifecycleTaskNo, Status: r.status, CapabilityEvidenceNo: "evidence-trial", CapabilityEvidenceDigest: strings.Repeat("a", 64), EvidenceDigest: strings.Repeat("b", 64), ObservedAt: time.Now()}
	if r.status == lifecycleresourcecap.TaskStatusFailed || r.status == lifecycleresourcecap.TaskStatusResultUnknown {
		result.FailureCode = "provider.operation_failed"
	}
	return result, nil
}

// trialNotifications requires the same active transaction as the business owner.
type trialNotifications struct {
	database    gdb.DB
	requests    []notificationcap.EventRequest
	unavailable bool
}

func (n *trialNotifications) Enqueue(ctx context.Context, request notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	if gdb.TXFromCtx(ctx, n.database.GetGroup()) == nil {
		return notificationcap.EventReceipt{}, gerror.New("missing source transaction")
	}
	if n.unavailable {
		return notificationcap.EventReceipt{}, gerror.New("notification unavailable")
	}
	n.requests = append(n.requests, request)
	return notificationcap.EventReceipt{EventNo: "event-trial"}, nil
}

// trialHarness owns an isolated default group, matching the DAO-backed task service.
type trialHarness struct {
	ctx           context.Context
	database      gdb.DB
	worker        *serviceImpl
	tasks         asynctask.Service
	resource      *trialResource
	notifications *trialNotifications
}

func newTrialHarness(t *testing.T) trialHarness {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN for trial lifecycle PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("linapro_trial_worker_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	original := gdb.GetAllConfig()
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if err := database.Close(ctx); err != nil {
				t.Error(err)
			}
		}
		if err := gdb.SetConfig(original); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	if err = gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: gdb.ConfigGroup{{Link: dsn + separator + "search_path=" + schema}}}); err != nil {
		t.Fatal(err)
	}
	database = g.DB()
	actual, err := database.GetValue(ctx, "SELECT current_schema()")
	if err != nil || actual.String() != schema {
		t.Fatalf("schema mismatch: %v %v", actual, err)
	}
	foundation, err := os.ReadFile("../../../../manifest/sql/001-core-foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"business_number", "async_task", "async_task_attempt"} {
		pattern := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS plugin_idc_core_` + name + ` \(.*?\n\);`)
		ddl := pattern.FindString(string(foundation))
		if ddl == "" {
			t.Fatalf("missing production DDL: %s", name)
		}
		if _, err = database.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	// The entitlement and contact fixtures expose only the authoritative columns
	// consumed by this worker; delivery activation has a separate full-schema test.
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_service_entitlement (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id int DEFAULT 0,
 entitlement_no text UNIQUE,customer_no text,version bigint DEFAULT 1,
 charge_type_code text DEFAULT 'trial',status_code text DEFAULT 'active',
 entitlement_owner_id text DEFAULT 'idc-resource',upstream_service_no text DEFAULT '',expires_at timestamptz,
 updated_at timestamptz DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE plugin_idc_core_customer_contact_role(id bigint GENERATED ALWAYS AS IDENTITY,tenant_id int DEFAULT 0,customer_no text,contact_no text,role_code text DEFAULT 'billing',status_code text DEFAULT 'active');
 CREATE TABLE plugin_idc_core_customer_contact(tenant_id int DEFAULT 0,customer_no text,contact_no text,status_code text DEFAULT 'active');`); err != nil {
		t.Fatal(err)
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatal(err)
	}
	resource := &trialResource{status: lifecycleresourcecap.TaskStatusSucceeded}
	notifications := &trialNotifications{database: database}
	worker, err := New(database, tasks, resource, notifications)
	if err != nil {
		t.Fatal(err)
	}
	return trialHarness{ctx, database, worker.(*serviceImpl), tasks, resource, notifications}
}
func (h trialHarness) seed(t *testing.T) string {
	t.Helper()
	no := "ent_" + strings.Repeat("a", 32)
	if _, err := h.database.Exec(h.ctx, `INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,expires_at) VALUES (?,'customer-trial',CURRENT_TIMESTAMP-INTERVAL '1 hour')`, no); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `
 INSERT INTO plugin_idc_core_customer_contact(customer_no,contact_no) VALUES ('customer-trial','contact-trial');
 INSERT INTO plugin_idc_core_customer_contact_role(customer_no,contact_no) VALUES ('customer-trial','contact-trial')`); err != nil {
		t.Fatal(err)
	}
	return no
}
func (h trialHarness) assertState(t *testing.T, status string, version int64) {
	t.Helper()
	var row struct {
		StatusCode string
		Version    int64
	}
	if err := h.database.GetScan(h.ctx, &row, `SELECT status_code,version FROM plugin_idc_core_service_entitlement WHERE customer_no='customer-trial'`); err != nil {
		t.Fatal(err)
	}
	if row.StatusCode != status || row.Version != version {
		t.Fatalf("entitlement got %+v want %s v%d", row, status, version)
	}
}
func (h trialHarness) count(t *testing.T, kind, status string) int {
	t.Helper()
	n, err := h.database.GetValue(h.ctx, `SELECT count(*) FROM plugin_idc_core_async_task WHERE task_type=? AND status_code=?`, kind, status)
	if err != nil {
		t.Fatal(err)
	}
	return n.Int()
}
func TestTrialSuspendsOnceAndNotifiesAfterSuccessPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	no := h.seed(t)
	for i := 0; i < 3; i++ {
		if err := h.worker.Run(h.ctx, 32); err != nil {
			t.Fatal(err)
		}
	}
	h.assertState(t, statusSuspended, 2)
	if h.resource.creates != 1 || len(h.notifications.requests) != 1 || h.count(t, taskType, "succeeded") != 1 {
		t.Fatalf("duplicate or missing effects: resource=%d notifications=%d", h.resource.creates, len(h.notifications.requests))
	}
	r := h.notifications.requests[0]
	if r.Recipient.CustomerNo != "customer-trial" || r.BusinessObjectNo != no || r.EventCode != "service.expiry_reminder" {
		t.Fatalf("wrong notification: %+v", r)
	}
}
func TestTrialPendingIsObservedWithoutRedispatchPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	h.resource.status = lifecycleresourcecap.TaskStatusPending
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusActive, 1)
	if h.count(t, taskType, "pending") != 1 || h.count(t, taskType, "failed") != 0 {
		t.Fatal("pending operation treated as failed")
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='pending'`); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusSuspended, 2)
	if h.resource.creates != 1 || h.resource.reads != 1 {
		t.Fatalf("redispatched resource operation: %+v", h.resource)
	}
}
func TestTrialUnknownAndFailureNeverChangeEntitlementPostgreSQL(t *testing.T) {
	for _, status := range []lifecycleresourcecap.TaskStatus{lifecycleresourcecap.TaskStatusFailed, lifecycleresourcecap.TaskStatusResultUnknown} {
		t.Run(string(status), func(t *testing.T) {
			h := newTrialHarness(t)
			h.seed(t)
			h.resource.status = status
			for i := 0; i < 2; i++ {
				if err := h.worker.Run(h.ctx, 32); err != nil {
					t.Fatal(err)
				}
			}
			h.assertState(t, statusActive, 1)
			expected := "failed"
			if status == lifecycleresourcecap.TaskStatusResultUnknown {
				expected = "result_unknown"
			}
			if h.count(t, taskType, expected) != 1 || h.resource.creates != 1 || len(h.notifications.requests) != 0 {
				t.Fatal("failed/unknown operation retried or projected success")
			}
		})
	}
}
func TestTrialPaidBeforeDispatchCancelsAndObsoleteReminderSkippedPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	if err := h.worker.schedule(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',version=2`); err != nil {
		t.Fatal(err)
	}
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusActive, 2)
	if h.resource.creates != 0 {
		t.Fatal("paid service suspended before dispatch")
	}
	if h.count(t, taskType, "succeeded") != 1 {
		t.Fatal("frozen suspension was not explicitly cancelled")
	}
}

func TestTrialObsoleteReminderAfterPaymentPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if h.count(t, notificationTaskType, "pending") != 1 {
		t.Fatal("missing pending reminder")
	}
	// A real pending reminder must not be sent after formal conversion.
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',status_code='active',version=3`); err != nil {
		t.Fatal(err)
	}
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if len(h.notifications.requests) != 0 {
		t.Fatal("obsolete trial reminder sent")
	}
	if h.count(t, notificationTaskType, "succeeded") != 1 {
		t.Fatal("obsolete reminder was not completed")
	}
}

func TestTrialRevisionBeforeDispatchDoesNotPermanentlySkipExpiryPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	if err := h.worker.schedule(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET version=2`); err != nil {
		t.Fatal(err)
	}
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusActive, 2)
	if h.resource.creates != 0 {
		t.Fatal("stale version was dispatched")
	}
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusSuspended, 3)
	if h.resource.creates != 1 {
		t.Fatal("current revision never suspended")
	}
}
func TestTrialReminderFailureDoesNotRollbackSuspensionPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	h.notifications.unavailable = true
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if err := h.worker.Run(h.ctx, 32); err == nil {
		t.Fatal("notification failure hidden from scheduler")
	}
	h.assertState(t, statusSuspended, 2)
	if h.count(t, notificationTaskType, "retry_wait") != 1 || h.count(t, taskType, "succeeded") != 1 {
		t.Fatal("notification failure rolled back suspension")
	}
	h.notifications.unavailable = false
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='retry_wait'`); err != nil {
		t.Fatal(err)
	}
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if len(h.notifications.requests) != 1 || h.resource.creates != 1 {
		t.Fatal("reminder retry repeated suspension")
	}
}

// TestTrialLateSuspensionRecoversFormalServicePostgreSQL covers the race using
// durable SQL tasks. The fixture commits formal facts, not an end-to-end payment.
func TestTrialLateSuspensionRecoversFormalServicePostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	h.resource.status = lifecycleresourcecap.TaskStatusPending
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	formalExpiry := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Microsecond)
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',version=2,expires_at=?`, formalExpiry); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='pending'`); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusSuspended, 3)
	if h.count(t, resumeTaskType, "pending") != 1 || h.count(t, notificationTaskType, "pending") != 0 {
		t.Fatal("late suspension did not queue recovery alone")
	}
	h.resource.status = lifecycleresourcecap.TaskStatusPending
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusSuspended, 3)
	if len(h.resource.requests) != 2 || h.resource.requests[1].Action != lifecycleresourcecap.ActionResume || h.resource.requests[1].ExpectedServiceVersion != 3 || !h.resource.requests[1].ExpectedExpiresAt.Equal(formalExpiry) {
		t.Fatalf("incorrect formal recovery: %+v", h.resource.requests)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='pending'`); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
	for i := 0; i < 2; i++ {
		if err := h.worker.Run(h.ctx, 32); err != nil {
			t.Fatal(err)
		}
	}
	h.assertState(t, statusActive, 4)
	if h.resource.creates != 2 || h.resource.reads != 2 || len(h.notifications.requests) != 0 || h.count(t, resumeTaskType, "succeeded") != 2 {
		t.Fatal("recovery redispatched or sent an obsolete reminder")
	}
	expires, err := h.database.GetValue(h.ctx, `SELECT expires_at FROM plugin_idc_core_service_entitlement`)
	if err != nil || !expires.Time().Equal(formalExpiry) {
		t.Fatalf("formal expiry changed: %v %v", expires, err)
	}
}

// TestTrialRecoveryFailureKeepsFormalServiceSuspendedPostgreSQL proves an
// external failure or ambiguous result never turns a paid service active early.
func TestTrialRecoveryFailureKeepsFormalServiceSuspendedPostgreSQL(t *testing.T) {
	for _, status := range []lifecycleresourcecap.TaskStatus{lifecycleresourcecap.TaskStatusFailed, lifecycleresourcecap.TaskStatusResultUnknown} {
		t.Run(string(status), func(t *testing.T) {
			h := newTrialHarness(t)
			h.seed(t)
			h.resource.status = lifecycleresourcecap.TaskStatusPending
			if err := h.worker.Run(h.ctx, 32); err != nil {
				t.Fatal(err)
			}
			if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',version=2,expires_at=CURRENT_TIMESTAMP+INTERVAL '30 days'; UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='pending'`); err != nil {
				t.Fatal(err)
			}
			h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
			if err := h.worker.Run(h.ctx, 32); err != nil {
				t.Fatal(err)
			}
			h.resource.status = status
			for i := 0; i < 2; i++ {
				if err := h.worker.Run(h.ctx, 32); err != nil {
					t.Fatal(err)
				}
			}
			h.assertState(t, statusSuspended, 3)
			expected := "failed"
			if status == lifecycleresourcecap.TaskStatusResultUnknown {
				expected = "result_unknown"
			}
			if h.count(t, resumeTaskType, expected) != 1 || h.resource.creates != 2 {
				t.Fatal("failed or unknown recovery was redispatched")
			}
		})
	}
}

// TestTrialRecoveryRevisionCancelsBeforeDispatchPostgreSQL prevents automatic
// compensation from undoing a newer staff suspension or other service change.
func TestTrialRecoveryRevisionCancelsBeforeDispatchPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	h.resource.status = lifecycleresourcecap.TaskStatusPending
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',version=2,expires_at=CURRENT_TIMESTAMP+INTERVAL '30 days'; UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='pending'`); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET version=4`); err != nil {
		t.Fatal(err)
	}
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusSuspended, 4)
	if h.resource.creates != 1 || h.count(t, resumeTaskType, "succeeded") != 1 {
		t.Fatal("stale recovery overwrote a newer service revision")
	}
}

// TestTrialRecoveryRevisionAfterDispatchFreezesPostgreSQL ensures an external
// success cannot overwrite a newer entitlement revision during observation.
func TestTrialRecoveryRevisionAfterDispatchFreezesPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	h.seed(t)
	h.resource.status = lifecycleresourcecap.TaskStatusPending
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',version=2,expires_at=CURRENT_TIMESTAMP+INTERVAL '30 days'; UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='pending'`); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusPending
	if err := h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET version=4; UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE status_code='pending'`); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
	for i := 0; i < 2; i++ {
		if err := h.worker.Run(h.ctx, 32); err != nil {
			t.Fatal(err)
		}
	}
	h.assertState(t, statusSuspended, 4)
	if h.count(t, resumeTaskType, "result_unknown") != 1 || h.resource.creates != 2 || h.resource.reads != 2 {
		t.Fatal("external success overwrote a revised service or failed to freeze")
	}
}

// TestPaidTrialRecoverySchedulerPostgreSQL exercises the payment producer and
// existing worker together. Failed supplier results retain suspension and the
// immutable paid expiry; a replay never creates another recovery intent.
func TestPaidTrialRecoverySchedulerPostgreSQL(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			h := newTrialHarness(t)
			entitlementNo := h.seed(t)
			if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement
 SET charge_type_code='recurring',status_code='suspended',expires_at=CURRENT_TIMESTAMP+INTERVAL '1 month',version=3`); err != nil {
				t.Fatal(err)
			}
			scheduler, err := NewRecoveryScheduler(h.database, h.tasks)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err = h.database.Transaction(h.ctx, func(ctx context.Context, _ gdb.TX) error {
					return scheduler.ScheduleRecovery(ctx, entitlementNo, "customer-trial")
				}); err != nil {
					t.Fatal(err)
				}
			}
			if h.count(t, resumeTaskType, "pending") != 1 {
				t.Fatal("payment recovery not idempotent")
			}
			before, err := h.database.GetValue(h.ctx, `SELECT expires_at FROM plugin_idc_core_service_entitlement`)
			if err != nil {
				t.Fatal(err)
			}
			if !success {
				h.resource.status = lifecycleresourcecap.TaskStatusFailed
			}
			if err = h.worker.Run(h.ctx, 32); err != nil {
				t.Fatal(err)
			}
			status, version := statusActive, int64(4)
			if !success {
				status, version = statusSuspended, 3
			}
			h.assertState(t, status, version)
			after, err := h.database.GetValue(h.ctx, `SELECT expires_at FROM plugin_idc_core_service_entitlement`)
			if err != nil || !before.Time().Equal(after.Time()) {
				t.Fatalf("recovery changed paid expiry: before=%v after=%v err=%v", before, after, err)
			}
			if h.resource.creates != 1 || len(h.notifications.requests) != 0 {
				t.Fatal("recovery redispatched or sent an obsolete trial reminder")
			}
			if err = scheduler.ScheduleRecovery(h.ctx, entitlementNo, "other-customer"); err == nil {
				t.Fatal("cross-customer recovery accepted")
			}
		})
	}
}

// TestPaidResumeConfirmedFailureRetriesWithFreshIdentityPostgreSQL preserves
// the failed evidence and paid facts while bounded confirmed retries recover.
func TestPaidResumeConfirmedFailureRetriesWithFreshIdentityPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	no := h.seed(t)
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',status_code='suspended',expires_at=CURRENT_TIMESTAMP+INTERVAL '1 month',version=3`); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewRecoveryScheduler(h.database, h.tasks)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.database.Transaction(h.ctx, func(ctx context.Context, _ gdb.TX) error {
		return scheduler.ScheduleRecovery(ctx, no, "customer-trial")
	}); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusFailed
	for attempt := 0; attempt <= maximumResumeRetries; attempt++ {
		if err = h.worker.Run(h.ctx, 32); err != nil {
			t.Fatal(err)
		}
		h.assertState(t, statusSuspended, 3)
		if h.count(t, resumeTaskType, "failed") != attempt+1 {
			t.Fatal("confirmed failure evidence was lost")
		}
		if attempt < maximumResumeRetries {
			if h.count(t, resumeTaskType, "pending") != 1 {
				t.Fatal("missing durable retry")
			}
			if _, err = h.database.Exec(h.ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE task_type=? AND status_code='pending'`, resumeTaskType); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	if h.resource.creates != maximumResumeRetries+1 || h.count(t, resumeTaskType, "pending") != 0 {
		t.Fatal("recovery exceeded bounded attempts")
	}
	seen := make(map[string]bool)
	for _, request := range h.resource.requests {
		if seen[request.LifecycleTaskNo] || request.CustomerNo != "customer-trial" || request.ServiceNo != no || request.ExpectedServiceVersion != 3 {
			t.Fatal("retry reused failed identity or changed ownership")
		}
		seen[request.LifecycleTaskNo] = true
	}
}

func TestPaidResumeRetrySuccessPreservesExpiryPostgreSQL(t *testing.T) {
	h := newTrialHarness(t)
	no := h.seed(t)
	if _, err := h.database.Exec(h.ctx, `UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring',status_code='suspended',expires_at=CURRENT_TIMESTAMP+INTERVAL '1 month',version=3`); err != nil {
		t.Fatal(err)
	}
	before, err := h.database.GetValue(h.ctx, `SELECT expires_at FROM plugin_idc_core_service_entitlement`)
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewRecoveryScheduler(h.database, h.tasks)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.database.Transaction(h.ctx, func(ctx context.Context, _ gdb.TX) error {
		return scheduler.ScheduleRecovery(ctx, no, "customer-trial")
	}); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusFailed
	if err = h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.resource.status = lifecycleresourcecap.TaskStatusSucceeded
	if _, err = h.database.Exec(h.ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP WHERE task_type=? AND status_code='pending'`, resumeTaskType); err != nil {
		t.Fatal(err)
	}
	if err = h.worker.Run(h.ctx, 32); err != nil {
		t.Fatal(err)
	}
	h.assertState(t, statusActive, 4)
	after, err := h.database.GetValue(h.ctx, `SELECT expires_at FROM plugin_idc_core_service_entitlement`)
	if err != nil || !after.Time().Equal(before.Time()) || h.count(t, resumeTaskType, "failed") != 1 || h.count(t, resumeTaskType, "succeeded") != 1 {
		t.Fatalf("retry lost paid expiry or evidence: %v %v", after, err)
	}
}
