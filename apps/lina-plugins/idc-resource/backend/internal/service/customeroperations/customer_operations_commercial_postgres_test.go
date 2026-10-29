// These tests use real resource tasks and migrations with a transactional
// commercial port fixture. The core owner's own payment tests remain separate.
package customeroperations

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/nativecommercial"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"strings"
	"sync"
	"testing"
	"time"
)

type commercialFixture struct {
	servercommercialcap.Service
	db        gdb.DB
	operation servercommercialcap.Operation
	allowed   bool
	receipts  []servercommercialcap.Completion
	rejected  []servercommercialcap.AdmissionFailure
}
type commercialTxKey struct{}

func (c *commercialFixture) ReconcileReady(context.Context, int) (int, error) { return 0, nil }

func (c *commercialFixture) RejectUnsent(ctx context.Context, input servercommercialcap.AdmissionFailure) error {
	updated, err := c.db.Exec(ctx, `UPDATE test_native_commercial_intent SET status='manual_review' WHERE number=? AND status='ready' AND task_no IS NULL`, input.OperationNo)
	if err != nil {
		return err
	}
	count, err := updated.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("cannot reject a bound commercial operation")
	}
	c.rejected = append(c.rejected, input)
	return nil
}

func (c *commercialFixture) ListReady(ctx context.Context, _ int) ([]string, error) {
	value, err := c.db.GetValue(ctx, `SELECT status FROM test_native_commercial_intent WHERE number=?`, c.operation.OperationNo)
	if err != nil {
		return nil, err
	}
	if value.String() == "ready" {
		return []string{c.operation.OperationNo}, nil
	}
	return []string{}, nil
}
func (c *commercialFixture) WithReadyOperation(ctx context.Context, _ string, enqueue func(context.Context, servercommercialcap.Operation) (string, error)) error {
	return c.db.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		value, err := c.db.GetValue(txctx, `SELECT status FROM test_native_commercial_intent WHERE number=? FOR UPDATE`, c.operation.OperationNo)
		if err != nil {
			return err
		}
		if value.String() == "executing" {
			return nil
		}
		no, err := enqueue(txctx, c.operation)
		if err != nil {
			return err
		}
		_, err = c.db.Exec(txctx, `UPDATE test_native_commercial_intent SET status='executing',task_no=? WHERE number=?`, no, c.operation.OperationNo)
		return err
	})
}
func (c *commercialFixture) WithQueuedOperation(ctx context.Context, number, task string, claim func(context.Context, servercommercialcap.Operation) error) error {
	if !c.allowed {
		return errors.New("settlement revoked")
	}
	return c.db.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		value, err := c.db.GetValue(txctx, `SELECT task_no FROM test_native_commercial_intent WHERE number=? AND status='executing' FOR UPDATE`, number)
		if err != nil || value.String() != task {
			return errors.New("wrong bound commercial task")
		}
		return claim(context.WithValue(txctx, commercialTxKey{}, true), c.operation)
	})
}
func (c *commercialFixture) Complete(_ context.Context, receipt servercommercialcap.Completion) error {
	c.receipts = append(c.receipts, receipt)
	return nil
}

func TestNativeCommercialSingleTaskAndDurableAdmissionPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	if _, err := f.db.Exec(f.ctx, `CREATE TABLE test_native_commercial_intent(number text PRIMARY KEY,status text NOT NULL,task_no text);INSERT INTO test_native_commercial_intent VALUES('commercial-resize-1','ready',NULL);
 UPDATE plugin_idc_resource_capability_snapshot_item SET support_code='conditional',constraints='[{"name":"cpu","required":false,"minimum":1}]' WHERE capability_code='server.resize'`); err != nil {
		t.Fatal(err)
	}
	owner := &commercialFixture{db: f.db, allowed: true, operation: servercommercialcap.Operation{OperationNo: "commercial-resize-1", EntitlementNo: "ent_" + strings.Repeat("1", 32), CustomerNo: "customer-a", LocalResourceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", PoolNo: "pool-main", ExpectedEntitlementVersion: 1, Action: serveroperationresourcecap.ActionResize, Parameters: json.RawMessage(`{"cpu":4}`), ContentDigest: strings.Repeat("b", 64)}}
	worker, err := nativecommercial.New(f.db, owner, f.source)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); failures <- worker.RunReady(f.ctx) }()
	}
	wg.Wait()
	close(failures)
	for err = range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	count, err := f.db.GetValue(f.ctx, `SELECT COUNT(*) FROM plugin_idc_resource_native_commercial_task`)
	if err != nil || count.Int() != 1 {
		t.Fatal("duplicate native commercial task", err)
	}
	store, err := resourcetask.NewStore(f.db)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "commercial-worker", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("commercial claim: %v", err)
	}
	task := tasks[0]
	request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: resourcecap.CapabilityResize, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters)}
	gate := worker.Admission(func(context.Context, resourcetask.ClaimedTask, resourcecap.OperationRequest, resourcecap.Provider, func(context.Context) error) error {
		t.Fatal("commercial task fell through to legacy gate")
		return nil
	})
	writes := 0
	invoke := func(ctx context.Context) error {
		if ctx.Value(commercialTxKey{}) != nil {
			t.Fatal("commercial provider call held core transaction")
		}
		writes++
		return nil
	}
	owner.allowed = false
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 0 {
		t.Fatal("revoked financial gate reached provider")
	}
	owner.allowed = true
	request.ResourceRef = "external-other"
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 0 {
		t.Fatal("retargeted commercial task reached another resource")
	}
	request.ResourceRef = "external-1"
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_native_commercial_task SET parameters='{"cpu":8}' WHERE task_no=?`, task.TaskNo); err != nil {
		t.Fatal(err)
	}
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 0 {
		t.Fatal("changed frozen parameters reached provider")
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_native_commercial_task SET parameters='{"cpu":4}' WHERE task_no=?`, task.TaskNo); err != nil {
		t.Fatal(err)
	}
	if err = gate(f.ctx, task, request, f.provider, invoke); err != nil || writes != 1 {
		t.Fatalf("durable commercial admission: %v", err)
	}
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 1 {
		t.Fatal("same admitted lease sent twice")
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='succeeded',finished_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE task_no=?`, task.TaskNo); err != nil {
		t.Fatal(err)
	}
	if err = worker.RunOutcomes(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = worker.RunOutcomes(f.ctx); err != nil {
		t.Fatal(err)
	}
	if len(owner.receipts) != 1 || owner.receipts[0].Status != servercommercialcap.CompletionSucceeded || owner.receipts[0].TaskNo != task.TaskNo {
		t.Fatalf("commercial durable receipt: %+v", owner.receipts)
	}
}

func TestNativeCommercialRemovedCapabilityClosesUnsentAdmissionPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	if _, err := f.db.Exec(f.ctx, `CREATE TABLE test_native_commercial_intent(number text PRIMARY KEY,status text NOT NULL,task_no text);INSERT INTO test_native_commercial_intent VALUES('commercial-resize-removed','ready',NULL);
UPDATE plugin_idc_resource_capability_snapshot_item SET support_code='unsupported',unsupported_reason='Capability removed',constraints='[]' WHERE capability_code='server.resize'`); err != nil {
		t.Fatal(err)
	}
	owner := &commercialFixture{db: f.db, operation: servercommercialcap.Operation{OperationNo: "commercial-resize-removed", EntitlementNo: "ent_" + strings.Repeat("1", 32), CustomerNo: "customer-a", LocalResourceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", PoolNo: "pool-main", ExpectedEntitlementVersion: 1, Action: serveroperationresourcecap.ActionResize, Parameters: json.RawMessage(`{"cpu":4}`), ContentDigest: strings.Repeat("b", 64)}}
	worker, err := nativecommercial.New(f.db, owner, f.source)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.RunReady(f.ctx); err == nil {
		t.Fatal("removed capability silently accepted")
	}
	if len(owner.rejected) != 1 || owner.rejected[0].ReasonCode != "capability_unavailable" || len(owner.rejected[0].EvidenceDigest) != 64 {
		t.Fatalf("missing permanent owner outcome: %+v", owner.rejected)
	}
	count, err := f.db.GetValue(f.ctx, `SELECT COUNT(*) FROM plugin_idc_resource_native_commercial_task`)
	if err != nil || count.Int() != 0 {
		t.Fatal("unsent rejection manufactured provider task", err)
	}
	if err = worker.RunReady(f.ctx); err != nil || len(owner.rejected) != 1 {
		t.Fatal("rejected intent retried", err)
	}
}
