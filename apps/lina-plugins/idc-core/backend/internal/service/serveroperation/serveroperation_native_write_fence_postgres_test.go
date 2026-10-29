// Free native write fences use real isolated PostgreSQL locks and the actual
// 151 migration. No provider, customer session or financial executor is used.

package serveroperation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// nativeWriteFixture adds the real parent version to the bounded aggregate fixture.
func nativeWriteFixture(t *testing.T) (*nativeCommercialService, servercommercialcap.QueuedWrite) {
	t.Helper()
	database := nativeLifecycleDB(t)
	if _, err := database.Exec(context.Background(), `ALTER TABLE plugin_idc_core_service_entitlement ADD COLUMN version BIGINT DEFAULT 1`); err != nil {
		t.Fatal(err)
	}
	return &nativeCommercialService{database: database}, servercommercialcap.QueuedWrite{OperationNo: "nso_free", TaskNo: "resource-task",
		EntitlementNo: "native-a", CustomerNo: "customer", ExpectedEntitlementVersion: 1,
		Action: serveroperationresourcecap.ActionReboot, RequestDigest: strings.Repeat("a", 64)}
}

// nativeWriteHold preserves the shared source/core transaction contract.
func nativeWriteHold(ctx context.Context, service *nativeCommercialService, input servercommercialcap.QueuedWrite) error {
	return service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return service.HoldQueuedWrite(ctx, input) })
}

func TestNativeQueuedWriteIdentityPostgreSQL(t *testing.T) {
	service, input := nativeWriteFixture(t)
	ctx := context.Background()
	if err := service.HoldQueuedWrite(ctx, input); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("nontransactional admission accepted: %v", err)
	}
	for _, action := range []serveroperationresourcecap.Action{serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionBackupCreate,
		serveroperationresourcecap.ActionResize, serveroperationresourcecap.ActionChangeIP, serveroperationresourcecap.ActionTrafficAdd} {
		invalid := input
		invalid.Action = action
		if err := nativeWriteHold(ctx, service, invalid); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("commercial action accepted: %s %v", action, err)
		}
	}
	for _, alter := range []func(*servercommercialcap.QueuedWrite){
		func(in *servercommercialcap.QueuedWrite) { in.CustomerNo = "other" },
		func(in *servercommercialcap.QueuedWrite) { in.ExpectedEntitlementVersion = 2 },
	} {
		invalid := input
		alter(&invalid)
		if err := nativeWriteHold(ctx, service, invalid); !errors.Is(err, ErrActionUnavailable) {
			t.Fatalf("foreign/stale parent accepted: %v", err)
		}
	}
	if err := nativeWriteHold(ctx, service, input); err != nil {
		t.Fatal(err)
	}
	if err := nativeWriteHold(ctx, service, input); err != nil {
		t.Fatalf("same executing identity failed: %v", err)
	}
	otherTask := input
	otherTask.TaskNo, otherTask.OperationNo = "another-task", "nso_another"
	if err := nativeWriteHold(ctx, service, otherTask); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("parallel task on the same parent accepted: %v", err)
	}
	for _, alter := range []func(*servercommercialcap.QueuedWrite){
		func(in *servercommercialcap.QueuedWrite) { in.OperationNo = "nso_other" },
		func(in *servercommercialcap.QueuedWrite) { in.EntitlementNo = "native-b" },
		func(in *servercommercialcap.QueuedWrite) { in.Action = serveroperationresourcecap.ActionStop },
		func(in *servercommercialcap.QueuedWrite) { in.RequestDigest = strings.Repeat("b", 64) },
	} {
		invalid := input
		alter(&invalid)
		if err := nativeWriteHold(ctx, service, invalid); !errors.Is(err, ErrOperationConflict) {
			t.Fatalf("changed task identity accepted: %v", err)
		}
	}
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_native_server_write_fence SET request_digest=repeat('b',64)`); err == nil {
		t.Fatal("database allowed immutable task digest change")
	}
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET customer_no='other' WHERE entitlement_no='native-a'`); err == nil {
		t.Fatal("ownership changed during provider execution")
	}
	if _, err := service.database.Exec(ctx, `DELETE FROM plugin_idc_core_native_server_write_fence`); err == nil {
		t.Fatal("execution history could be removed")
	}
	// Reapplying the actual upgrade preserves an already admitted fence.
	nativeLifecycleMigration(t, service.database, "151-native-server-accessory-lifecycle.sql")
	if err := nativeWriteHold(ctx, service, input); err != nil {
		t.Fatal(err)
	}
}

func TestNativeQueuedWriteParentAndCommercialGatePostgreSQL(t *testing.T) {
	for _, change := range []string{
		`UPDATE plugin_idc_core_service_entitlement SET status_code='suspended' WHERE entitlement_no='native-a'`,
		`UPDATE plugin_idc_core_service_entitlement SET expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE entitlement_no='native-a'`,
		`INSERT INTO plugin_idc_core_refund_entitlement_lock(tenant_id,entitlement_no) VALUES(0,'native-a')`,
		`INSERT INTO plugin_idc_core_business_operation(operation_no,subject_type_code,subject_no,operation_type_code,status_code) VALUES('commercial','entitlement','native-a','addon','ready')`,
		`INSERT INTO plugin_idc_core_business_operation(operation_no,subject_type_code,subject_no,operation_type_code,status_code,financial_settlement_required,financially_settled_at) VALUES('commercial','entitlement','native-a','ip_change','ready',TRUE,CURRENT_TIMESTAMP)`,
	} {
		t.Run(change, func(t *testing.T) {
			service, input := nativeWriteFixture(t)
			if _, err := service.database.Exec(context.Background(), change); err != nil {
				t.Fatal(err)
			}
			if err := nativeWriteHold(context.Background(), service, input); !errors.Is(err, ErrActionUnavailable) {
				t.Fatalf("current parent/commercial gate bypassed: %v", err)
			}
		})
	}
}

func TestNativeQueuedWriteReceiptRecoveryPostgreSQL(t *testing.T) {
	service, input := nativeWriteFixture(t)
	ctx := context.Background()
	if err := nativeWriteHold(ctx, service, input); err != nil {
		t.Fatal(err)
	}
	receipt := servercommercialcap.QueuedWriteCompletion{TaskNo: input.TaskNo, EntitlementNo: input.EntitlementNo, CustomerNo: input.CustomerNo,
		Status: servercommercialcap.CompletionUnknown, EvidenceDigest: strings.Repeat("b", 64)}
	if err := service.CompleteQueuedWrite(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	if err := service.CompleteQueuedWrite(ctx, receipt); err != nil {
		t.Fatalf("unknown same receipt not idempotent: %v", err)
	}
	if err := nativeWriteHold(ctx, service, input); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("unknown execution could be submitted again: %v", err)
	}
	for _, digest := range []string{strings.Repeat("c", 64), strings.Repeat("e", 64)} {
		receipt.EvidenceDigest = digest
		if err := service.CompleteQueuedWrite(ctx, receipt); err != nil {
			t.Fatalf("new unknown observation could not update projection: %v", err)
		}
		row, err := nativeReadQueuedWrite(ctx, service.database, input.TaskNo)
		if err != nil || row == nil || row.StatusCode != nativeQueuedWriteState(servercommercialcap.CompletionUnknown) || row.EvidenceDigest != digest {
			t.Fatalf("latest unknown receipt not retained: %+v %v", row, err)
		}
		if err := nativeWriteHold(ctx, service, input); !errors.Is(err, ErrOperationConflict) {
			t.Fatalf("new unknown observation released execution hold: %v", err)
		}
		busy, err := service.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_native_server_write_fence WHERE tenant_id=0 AND entitlement_no=? AND status_code IN ('executing','result_unknown'))`, input.EntitlementNo)
		if err != nil || !busy.Bool() {
			t.Fatalf("new unknown observation stopped blocking refund: %v %v", busy, err)
		}
	}
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='expired' WHERE entitlement_no='native-a'`); err != nil {
		t.Fatal(err)
	}
	receipt.Status = servercommercialcap.CompletionSucceeded
	receipt.EvidenceDigest = strings.Repeat("d", 64)
	if err := service.CompleteQueuedWrite(ctx, receipt); err != nil {
		t.Fatalf("real completion blocked by expired parent: %v", err)
	}
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET customer_no='other' WHERE entitlement_no='native-a'`); err != nil {
		t.Fatalf("terminal fence blocked later ownership change: %v", err)
	}
	if err := service.CompleteQueuedWrite(ctx, receipt); err != nil {
		t.Fatalf("historical same receipt not idempotent: %v", err)
	}
	conflict := receipt
	conflict.EvidenceDigest = strings.Repeat("f", 64)
	if err := service.CompleteQueuedWrite(ctx, conflict); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("terminal evidence changed: %v", err)
	}
	conflict = receipt
	conflict.Status = servercommercialcap.CompletionFailed
	if err := service.CompleteQueuedWrite(ctx, conflict); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("terminal outcome changed: %v", err)
	}
	rows, err := service.ListQueuedWrites(ctx, 100)
	if err != nil || len(rows) != 0 {
		t.Fatalf("terminal fence remained an active candidate: %v %v", rows, err)
	}
}

func TestNativeQueuedWriteConcurrentReceiptsPostgreSQL(t *testing.T) {
	service, input := nativeWriteFixture(t)
	ctx := context.Background()
	var group sync.WaitGroup
	errorsFound := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() { defer group.Done(); errorsFound <- nativeWriteHold(ctx, service, input) }()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	results := make(chan error, 2)
	for _, status := range []servercommercialcap.CompletionStatus{servercommercialcap.CompletionSucceeded, servercommercialcap.CompletionFailed} {
		group.Add(1)
		go func(status servercommercialcap.CompletionStatus) {
			defer group.Done()
			results <- service.CompleteQueuedWrite(ctx, servercommercialcap.QueuedWriteCompletion{TaskNo: input.TaskNo, EntitlementNo: input.EntitlementNo,
				CustomerNo: input.CustomerNo, Status: status, EvidenceDigest: strings.Repeat("c", 64)})
		}(status)
	}
	group.Wait()
	close(results)
	var accepted, rejected int
	for err := range results {
		if err == nil {
			accepted++
		} else if errors.Is(err, ErrOperationConflict) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("competing receipts did not linearize: accepted=%d rejected=%d", accepted, rejected)
	}
}

func TestNativeQueuedWriteRefundLockOrderPostgreSQL(t *testing.T) {
	for _, holdFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "write_commits_first", false: "refund_commits_first"}[holdFirst], func(t *testing.T) {
			service, input := nativeWriteFixture(t)
			ctx := context.Background()
			var (
				locked  = make(chan struct{})
				release = make(chan struct{})
				first   = make(chan error, 1)
				second  = make(chan error, 1)
				pid     = make(chan int, 1)
			)
			refund := func(ctx context.Context) error {
				if _, err := service.database.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE entitlement_no=? FOR UPDATE`, input.EntitlementNo); err != nil {
					return err
				}
				// A fresh statement after the parent lock observes the write that
				// committed while waiting, matching the financial owner's guard.
				busy, err := service.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_native_server_write_fence WHERE entitlement_no=? AND status_code IN ('executing','result_unknown'))`, input.EntitlementNo)
				if err != nil {
					return err
				}
				if busy.Bool() {
					return ErrOperationConflict
				}
				_, err = service.database.Exec(ctx, `INSERT INTO plugin_idc_core_refund_entitlement_lock(tenant_id,entitlement_no) VALUES(0,?)`, input.EntitlementNo)
				return err
			}
			go func() {
				first <- service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					var err error
					if holdFirst {
						err = service.HoldQueuedWrite(ctx, input)
					} else {
						err = refund(ctx)
					}
					close(locked)
					<-release
					return err
				})
			}()
			<-locked
			go func() {
				second <- service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					value, err := service.database.GetValue(ctx, `SELECT pg_backend_pid()`)
					if err != nil {
						return err
					}
					pid <- value.Int()
					if holdFirst {
						return refund(ctx)
					}
					return service.HoldQueuedWrite(ctx, input)
				})
			}()
			nativeAwaitSQLLock(t, service.database, <-pid)
			close(release)
			if err := <-first; err != nil {
				t.Fatal(err)
			}
			err := <-second
			if (holdFirst && !errors.Is(err, ErrOperationConflict)) || (!holdFirst && !errors.Is(err, ErrActionUnavailable)) {
				t.Fatalf("refund/write boundary did not serialize: %v", err)
			}
		})
	}
}

func TestNativeQueuedWriteRotationPostgreSQL(t *testing.T) {
	service, input := nativeWriteFixture(t)
	ctx := context.Background()
	if err := nativeWriteHold(ctx, service, input); err != nil {
		t.Fatal(err)
	}
	other := input
	other.EntitlementNo, other.OperationNo, other.TaskNo = "native-b", "nso_second", "second-task"
	if err := nativeWriteHold(ctx, service, other); err != nil {
		t.Fatal(err)
	}
	if err := service.CompleteQueuedWrite(ctx, servercommercialcap.QueuedWriteCompletion{TaskNo: input.TaskNo, EntitlementNo: input.EntitlementNo,
		CustomerNo: input.CustomerNo, Status: servercommercialcap.CompletionUnknown, EvidenceDigest: strings.Repeat("b", 64)}); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for range 2 {
		rows, err := service.ListQueuedWrites(ctx, 1)
		if err != nil || len(rows) != 1 {
			t.Fatalf("bounded reconciliation failed: %v %v", rows, err)
		}
		seen[rows[0].TaskNo] = true
	}
	if !seen[input.TaskNo] || !seen[other.TaskNo] {
		t.Fatalf("uncertain head starved another fence: %v", seen)
	}
	if err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := service.ListQueuedWrites(ctx, 1)
		if !errors.Is(err, ErrOperationConflict) {
			t.Fatalf("rotation could hold fence before caller acquires parent: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{0, 101} {
		if _, err := service.ListQueuedWrites(ctx, limit); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("unbounded candidate request accepted: %d %v", limit, err)
		}
	}
}

func TestNativeQueuedWritePersistenceFailurePostgreSQL(t *testing.T) {
	service, input := nativeWriteFixture(t)
	ctx := context.Background()
	if err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := service.HoldQueuedWrite(ctx, input); err != nil {
			return err
		}
		return ErrOperationConflict // The source task claim also rolled back.
	}); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("source rollback not preserved: %v", err)
	}
	count, err := service.database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_native_server_write_fence`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("rolled back admission escaped shared transaction: %v %v", count, err)
	}
	if err := nativeWriteHold(ctx, service, input); err != nil {
		t.Fatal(err)
	}
	if _, err := service.database.Exec(ctx, `CREATE FUNCTION native_test_fail_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'injected receipt persistence failure'; END; $$;
CREATE TRIGGER native_test_fail_receipt BEFORE UPDATE ON plugin_idc_core_native_server_write_fence
FOR EACH ROW EXECUTE FUNCTION native_test_fail_receipt()`); err != nil {
		t.Fatal(err)
	}
	receipt := servercommercialcap.QueuedWriteCompletion{TaskNo: input.TaskNo, EntitlementNo: input.EntitlementNo, CustomerNo: input.CustomerNo,
		Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: strings.Repeat("f", 64)}
	if err := service.CompleteQueuedWrite(ctx, receipt); err == nil {
		t.Fatal("receipt fault was ignored")
	}
	state, err := service.database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_native_server_write_fence WHERE task_no=?`, input.TaskNo)
	if err != nil || state.String() != "executing" {
		t.Fatalf("failed persistence falsely released fence: %v %v", state, err)
	}
	if _, err := service.database.Exec(ctx, `DROP TRIGGER native_test_fail_receipt ON plugin_idc_core_native_server_write_fence`); err != nil {
		t.Fatal(err)
	}
	if err := service.CompleteQueuedWrite(ctx, receipt); err != nil {
		t.Fatalf("verified receipt could not recover after failure: %v", err)
	}
}
