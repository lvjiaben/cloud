// These tests exercise the source-only task seam with actual financial and
// accessory constraints; the callback stands for resource-owned local writes.
package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"strings"
	"sync"
	"testing"
	"time"
)

func nativeFreeExecution(t *testing.T) (nativeCommercialFixture, *servercommercialcap.Purchase) {
	t.Helper()
	f := newNativeCommercialFixture(t)
	q, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionSnapshotCreate, Parameters: json.RawMessage(`{"name":"execution-test"}`), IdempotencyKey: "execution-native-quote"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: q.QuoteNo, IdempotencyKey: "execution-native-checkout"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.database.Exec(f.ctx, `CREATE TABLE test_native_task(operation_no TEXT UNIQUE,task_no TEXT UNIQUE,claimed BOOLEAN NOT NULL DEFAULT FALSE)`); err != nil {
		t.Fatal(err)
	}
	return f, p
}

func TestNativeCommercialTaskAtomicityPostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	failed := errors.New("resource local persistence failed")
	err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(ctx context.Context, op servercommercialcap.Operation) (string, error) {
		if gdb.TXFromCtx(ctx, f.database.GetGroup()) == nil {
			t.Error("resource callback lost shared transaction")
		}
		_, err := f.database.Exec(ctx, `INSERT INTO test_native_task(operation_no,task_no) VALUES(?,?)`, op.OperationNo, "task_rolled_back")
		if err != nil {
			return "", err
		}
		return "", failed
	})
	if err == nil {
		t.Fatal("failed enqueue committed")
	}
	f.assertCount(t, `SELECT count(*) FROM test_native_task`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='ready'`, 1)
	const task = "task_native_execution"
	var wait sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errs <- f.native.WithReadyOperation(f.ctx, p.OperationNo, func(ctx context.Context, op servercommercialcap.Operation) (string, error) {
				if op.EntitlementNo != f.target.EntitlementNo || op.LocalResourceNo != f.target.LocalResourceNo || op.Action != serveroperationresourcecap.ActionSnapshotCreate {
					return "", errors.New("wrong frozen target")
				}
				_, err := f.database.Exec(ctx, `INSERT INTO test_native_task(operation_no,task_no) VALUES(?,?)`, op.OperationNo, task)
				return task, err
			})
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent enqueue: %+v", err)
		}
	}
	f.assertCount(t, `SELECT count(*) FROM test_native_task`, 1)
	claim := func(ctx context.Context, op servercommercialcap.Operation) error {
		_, err := f.database.Exec(ctx, `UPDATE test_native_task SET claimed=TRUE WHERE operation_no=? AND task_no=?`, op.OperationNo, task)
		return err
	}
	if err = f.native.WithQueuedOperation(f.ctx, p.OperationNo, "wrong_task", claim); err == nil {
		t.Fatal("wrong task claimed")
	}
	if err = f.native.WithQueuedOperation(f.ctx, p.OperationNo, task, claim); err != nil {
		t.Fatalf("queued claim: %+v", err)
	}
	f.assertCount(t, `SELECT count(*) FROM test_native_task WHERE claimed`, 1)
	unknown := servercommercialcap.Completion{OperationNo: p.OperationNo, EntitlementNo: f.target.EntitlementNo, TaskNo: task, EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC(), Status: servercommercialcap.CompletionUnknown}
	if err = f.native.Complete(f.ctx, unknown); err != nil {
		t.Fatalf("unknown receipt: %+v", err)
	}
	if err = f.native.Complete(f.ctx, unknown); err != nil {
		t.Fatalf("unknown replay: %+v", err)
	}
	if err = f.native.WithQueuedOperation(f.ctx, p.OperationNo, task, claim); err == nil {
		t.Fatal("unknown outcome allowed another provider claim")
	}
	success := unknown
	success.Status = servercommercialcap.CompletionSucceeded
	success.ObjectNo = "snapshot_local_created"
	success.EvidenceDigest = strings.Repeat("b", 64)
	if err = f.native.Complete(f.ctx, success); err != nil {
		t.Fatalf("reconciled receipt: %+v", err)
	}
	if err = f.native.Complete(f.ctx, success); err != nil {
		t.Fatalf("success replay: %+v", err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='active' AND object_no='snapshot_local_created'`, 1)
	changed := success
	changed.ObjectNo = "snapshot_other"
	if err = f.native.Complete(f.ctx, changed); err == nil {
		t.Fatal("divergent terminal receipt accepted")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_instance`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_operation_execution`, 0)
}

func TestNativeCommercialQueuedVersionFencePostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) { return "task_version_fence", nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_service_entitlement SET version=version+1 WHERE entitlement_no=?`, f.target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := f.native.WithQueuedOperation(f.ctx, p.OperationNo, "task_version_fence", func(context.Context, servercommercialcap.Operation) error { called = true; return nil }); err == nil || called {
		t.Fatal("stale entitlement admitted queued task")
	}
}
