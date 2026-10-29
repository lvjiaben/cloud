package customeroperations

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

type outsideTransactionProvider struct {
	resourcecap.Provider
	db gdb.DB
	t  *testing.T
}

func (p outsideTransactionProvider) QueryResource(ctx context.Context, q resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
	if gdb.TXFromCtx(ctx, p.db.GetGroup()) != nil {
		p.t.Fatal("supplier observation ran inside owner transaction")
	}
	return p.Provider.QueryResource(ctx, q)
}

func TestNativeWriteClaimCommitRollbackAndReceiptsPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	command := Command{ServerNo: f.serverNo, Action: "reboot", IdempotencyKey: "durable-write-fence", Confirmed: true}
	if _, err := f.service.Create(f.ctx, command); err != nil {
		t.Fatal(err)
	}
	store, err := resourcetask.NewStore(f.db)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "fence-worker", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(tasks) != 1 {
		t.Fatal("claim", err)
	}
	task := tasks[0]
	owner := f.service.objects.(*nativeObjectOwner)
	var frozen servercommercialcap.QueuedWrite
	failHold := true
	owner.hold = func(ctx context.Context, input servercommercialcap.QueuedWrite) error {
		if gdb.TXFromCtx(ctx, f.db.GetGroup()) == nil {
			t.Fatal("core fence outside shared transaction")
		}
		claim, readErr := f.db.GetValue(ctx, `SELECT owner_metadata->>'nativeWriteClaim' FROM plugin_idc_resource_task WHERE task_no=?`, task.TaskNo)
		if readErr != nil || claim.Int() != task.AttemptNo {
			t.Fatal("core hold preceded durable source claim", readErr)
		}
		frozen = input
		if failHold {
			return errors.New("refund acquired parent first")
		}
		return nil
	}
	gate, err := NewWriteAdmission(f.db, f.customers, f.source, owner)
	if err != nil {
		t.Fatal(err)
	}
	request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: resourcecap.CapabilityReboot, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters)}
	provider := outsideTransactionProvider{Provider: f.provider, db: f.db, t: t}
	writes := 0
	invoke := func(ctx context.Context) error {
		if gdb.TXFromCtx(ctx, f.db.GetGroup()) != nil {
			t.Fatal("provider write held owner transaction")
		}
		// An independent connection can lock the task while the cloud call runs.
		if lockErr := f.db.Transaction(context.Background(), func(other context.Context, _ gdb.TX) error {
			_, lockErr := f.db.GetValue(other, `SELECT task_no FROM plugin_idc_resource_task WHERE task_no=? FOR UPDATE NOWAIT`, task.TaskNo)
			return lockErr
		}); lockErr != nil {
			t.Fatal("provider write retained task lock", lockErr)
		}
		writes++
		return nil
	}
	if err = gate(f.ctx, task, request, provider, invoke); err == nil || writes != 0 {
		t.Fatal("failed core fence invoked supplier")
	}
	claim, err := f.db.GetValue(f.ctx, `SELECT jsonb_exists(owner_metadata,'nativeWriteClaim') FROM plugin_idc_resource_task WHERE task_no=?`, task.TaskNo)
	if err != nil || claim.Bool() {
		t.Fatal("failed core hold did not roll back source claim", err)
	}
	failHold = false
	if err = gate(f.ctx, task, request, provider, invoke); err != nil || writes != 1 {
		t.Fatal("valid committed claim", err)
	}
	if err = gate(f.ctx, task, request, provider, invoke); !errors.Is(err, resourcetask.ErrAdmissionOutcomeUncertain) || writes != 1 {
		t.Fatal("same task invoked twice or produced false non-acceptance")
	}
	metadata, err := validateNativeTask(task, request)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.OperationNo != metadata.OperationNo || frozen.RequestDigest != hex.EncodeToString(metadata.RequestDigest[:]) || frozen.CustomerNo != "customer-a" || frozen.ExpectedEntitlementVersion != 1 {
		t.Fatal("core fence lost frozen identity")
	}
	owner.writes = []servercommercialcap.QueuedWrite{frozen}
	reconciler, err := NewWriteReconciler(f.db, owner)
	if err != nil {
		t.Fatal(err)
	}
	if err = reconciler.Run(f.ctx); err != nil || len(owner.receipts) != 0 {
		t.Fatal("unfinished task released core fence", err)
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='result_unknown',result_unknown=true WHERE task_no=?`, task.TaskNo); err != nil {
		t.Fatal(err)
	}
	if err = reconciler.Run(f.ctx); err != nil || len(owner.receipts) != 1 || owner.receipts[0].Status != servercommercialcap.CompletionUnknown {
		t.Fatal("unknown task was not retained", err)
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='succeeded',result_unknown=false,finished_at=CURRENT_TIMESTAMP,version=version+1 WHERE task_no=?`, task.TaskNo); err != nil {
		t.Fatal(err)
	}
	if err = reconciler.Run(f.ctx); err != nil || len(owner.receipts) != 2 || owner.receipts[1].Status != servercommercialcap.CompletionSucceeded || owner.receipts[1].EvidenceDigest == owner.receipts[0].EvidenceDigest {
		t.Fatal("confirmed receipt not reported", err)
	}
	owner.writes[0].RequestDigest = "wrong-frozen-digest"
	if err = reconciler.Run(f.ctx); err != nil || len(owner.receipts) != 2 {
		t.Fatal("mismatched claim released fence", err)
	}
}

func TestNativeWriteRetryRequiresPersistedNonAcceptancePostgreSQL(t *testing.T) {
	for _, proven := range []bool{true, false} {
		t.Run(map[bool]string{true: "proven_retry", false: "unproven_retry"}[proven], func(t *testing.T) {
			f := newManagementFixture(t)
			if _, err := f.service.Create(f.ctx, Command{ServerNo: f.serverNo, Action: "reboot", IdempotencyKey: "native-safe-retry", Confirmed: true}); err != nil {
				t.Fatal(err)
			}
			store, err := resourcetask.NewStore(f.db)
			if err != nil {
				t.Fatal(err)
			}
			claim := func() resourcetask.ClaimedTask {
				t.Helper()
				rows, claimErr := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "safe-retry", Limit: 1, LeaseDuration: time.Minute})
				if claimErr != nil || len(rows) != 1 {
					t.Fatal("claim", claimErr)
				}
				return rows[0]
			}
			first := claim()
			gate, err := NewWriteAdmission(f.db, f.customers, f.source, f.service.objects)
			if err != nil {
				t.Fatal(err)
			}
			request := resourcecap.OperationRequest{RequestID: first.TaskNo, Capability: resourcecap.CapabilityReboot, ResourceRef: "external-1", Parameters: []byte(first.SanitizedParameters)}
			writes := 0
			invoke := func(context.Context) error { writes++; return nil }
			if err = gate(f.ctx, first, request, f.provider, invoke); err != nil {
				t.Fatal(err)
			}
			if proven {
				next := time.Now().Add(time.Second)
				err = store.Transition(f.ctx, resourcetask.TransitionInput{Lease: resourcetask.LeaseRef{TaskNo: first.TaskNo, Token: first.LeaseToken, AttemptNo: first.AttemptNo}, To: resourcetask.StatusRetryWait, Evidence: resourcetask.TransitionEvidence{NonAcceptanceProven: true}, FailureCode: "request_not_accepted", FailureText: "verified rejection before acceptance", NextAt: &next})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='retry_wait',locked_by='',lease_expires_at=NULL WHERE task_no=?`, first.TaskNo); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET available_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE task_no=?`, first.TaskNo); err != nil {
				t.Fatal(err)
			}
			second := claim()
			if second.AttemptNo != first.AttemptNo+1 {
				t.Fatal("retry lost attempt identity")
			}
			err = gate(f.ctx, second, request, f.provider, invoke)
			if proven {
				if err != nil || writes != 2 {
					t.Fatalf("verified non-acceptance was stranded: writes=%d err=%v", writes, err)
				}
			} else if !errors.Is(err, resourcetask.ErrAdmissionOutcomeUncertain) || writes != 1 {
				t.Fatal("unproven retry reached cloud", err)
			}
		})
	}
}
