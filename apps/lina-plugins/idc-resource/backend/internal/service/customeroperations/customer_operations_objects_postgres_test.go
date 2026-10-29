package customeroperations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
	"lina-plugin-idc-resource/backend/internal/service/nativecommercial"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

type deletionOwner struct {
	servercommercialcap.Service
	db      gdb.DB
	calls   int
	pending bool
}

func (o *deletionOwner) CompleteDeletion(ctx context.Context, input servercommercialcap.Deletion) error {
	o.calls++
	if gdb.TXFromCtx(ctx, o.db.GetGroup()) != nil {
		return errors.New("receipt wrapped payment-close transaction")
	}
	if _, err := o.db.Exec(ctx, `INSERT INTO test_deletion_progress(task_no)VALUES(?) ON CONFLICT DO NOTHING`, input.TaskNo); err != nil {
		return err
	}
	if o.pending {
		return errors.New("payment close pending")
	}
	return nil
}

func TestNativeObjectRestoreEligibilityAndDeletionReceiptPostgreSQL(t *testing.T) {
	for _, action := range []string{"snapshot_restore", "backup_restore", "snapshot_delete", "backup_delete"} {
		t.Run(action, func(t *testing.T) {
			f := newManagementFixture(t)
			kind, capability := resourcecap.AccessoryKindSnapshot, resourcecap.CapabilityManageSnapshot
			if strings.HasPrefix(action, "backup") {
				kind, capability = resourcecap.AccessoryKindBackup, resourcecap.CapabilityBackup
			}
			if _, err := f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_capability_snapshot_item SET support_code='conditional',constraints='[{"name":"action","required":true,"allowedValues":["create","delete","restore"]},{"name":"snapshot_ref","minimum":1}]' WHERE capability_code=?`, capability); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec(f.ctx, `CREATE TABLE test_deletion_progress(task_no TEXT PRIMARY KEY)`); err != nil {
				t.Fatal(err)
			}
			directory, err := accessorydirectory.New(f.db)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			published, err := directory.Publish(f.ctx, accessorydirectory.PublishInput{MappingNo: "mapping-1", LocalResourceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", ExternalResourceRef: "external-1", Snapshot: resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(time.Minute), Items: []resourcecap.AccessoryItem{{Kind: kind, State: resourcecap.AccessoryStateAvailable, ExternalRef: "77", DisplayName: "Owned snapshot", Data: json.RawMessage(`{}`)}}}})
			if err != nil {
				t.Fatal(err)
			}
			values := map[string]string{"objectNo": published.Items[0].ObjectNo}
			restore := strings.HasSuffix(action, "restore")
			if restore {
				values["confirmedServiceNo"] = f.serverNo
				f.provider.state = resourcecap.ResourceStateStopped
			}
			parameters, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			command := Command{ServerNo: f.serverNo, Action: action, Parameters: parameters, IdempotencyKey: "free-object-action", Confirmed: true}
			objects := f.service.objects.(*nativeObjectOwner)
			if restore {
				objects.blocked = true
				if _, err = f.service.Create(f.ctx, command); err == nil {
					t.Fatal("unpaid/expired object restored")
				}
				objects.blocked = false
			}
			if _, err = f.service.Create(f.ctx, command); err != nil {
				t.Fatal(err)
			}
			store, err := resourcetask.NewStore(f.db)
			if err != nil {
				t.Fatal(err)
			}
			tasks, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "object-worker", Limit: 1, LeaseDuration: time.Minute})
			if err != nil || len(tasks) != 1 {
				t.Fatal("task claim", err)
			}
			task := tasks[0]
			gate, err := NewWriteAdmission(f.db, f.customers, f.source, objects)
			if err != nil {
				t.Fatal(err)
			}
			request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: capability, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters)}
			writes := 0
			invoke := func(context.Context) error { writes++; return nil }
			if restore {
				objects.blocked = true
				if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 0 {
					t.Fatal("queued restore bypassed object payment expiry")
				}
				objects.blocked = false
			}
			if err = gate(f.ctx, task, request, f.provider, invoke); err != nil || writes != 1 {
				t.Fatal("free object dispatch", err)
			}
			if restore {
				return
			}
			verifyDeletionReceipt(t, f, task.TaskNo)
		})
	}
}

func verifyDeletionReceipt(t *testing.T, f *managementFixture, taskNo string) {
	t.Helper()
	owner := &deletionOwner{db: f.db, pending: true}
	worker, err := nativecommercial.New(f.db, owner, f.source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='result_unknown',result_unknown=true WHERE task_no=?`, taskNo); err != nil {
		t.Fatal(err)
	}
	if err = worker.RunOutcomes(f.ctx); err != nil || owner.calls != 0 {
		t.Fatal("unknown deletion stopped renewal", err)
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='succeeded',result_unknown=false,finished_at=CURRENT_TIMESTAMP WHERE task_no=?`, taskNo); err != nil {
		t.Fatal(err)
	}
	if err = worker.RunOutcomes(f.ctx); err == nil || owner.calls != 1 {
		t.Fatal("pending close was not retried")
	}
	progress, err := f.db.GetValue(f.ctx, `SELECT COUNT(*) FROM test_deletion_progress`)
	if err != nil || progress.Int() != 1 {
		t.Fatal("payment-close progress rolled back", err)
	}
	owner.pending = false
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_native_accessory_receipt SET next_report_at=CURRENT_TIMESTAMP`); err != nil {
		t.Fatal(err)
	}
	if err = worker.RunOutcomes(f.ctx); err != nil || owner.calls != 2 {
		t.Fatal("confirmed deletion was not delivered", err)
	}
	if err = worker.RunOutcomes(f.ctx); err != nil || owner.calls != 2 {
		t.Fatal("completed deletion receipt repeated", err)
	}
}
