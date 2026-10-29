// This file verifies local successor creation and rejection fences without any supplier HTTP.

package provisionsource

import (
	"errors"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"strings"
	"testing"
)

func TestRejectedRecoveryRetainsHistoryAndIsIdempotentPostgreSQL(t *testing.T) {
	ctx, db := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, db)
	secrets := newProvisionSecretSource(t)
	source, err := New(db, secrets, principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	old, err := source.CreateTask(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	read := provisionresourcecap.TaskReadRequest{ProviderTaskNo: old.ProviderTaskNo, Request: request}
	if _, err = source.RecoverRejected(ctx, read); !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
		t.Fatalf("accepted recovery: %v", err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code='failed',failure_code='UPSTREAM_REQUEST_REJECTED',result_unknown=FALSE,finished_at=CURRENT_TIMESTAMP,version=version+1 WHERE task_no=?`, old.ProviderTaskNo); err != nil {
		t.Fatal(err)
	}
	failed, err := source.ReadTask(ctx, read)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = source.RecoverRejected(ctx, read); !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
		t.Fatalf("unreported recovery: %v", err)
	}
	if err = source.ReportOutcome(ctx, provisionresourcecap.OutcomeReport{ProviderTaskNo: old.ProviderTaskNo, Request: request, Status: provisionresourcecap.OutcomeStatusFailed, TaskEvidenceDigest: failed.EvidenceDigest, CoreEvidenceDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	next, err := source.RecoverRejected(ctx, read)
	if err != nil || next.ProviderTaskNo == old.ProviderTaskNo {
		t.Fatalf("recover=%+v %v", next, err)
	}
	replay, err := source.RecoverRejected(ctx, read)
	if err != nil || replay.ProviderTaskNo != next.ProviderTaskNo {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	var evidence struct {
		Tasks, Services, Receipts   int
		OldStatus, CredentialStatus string
	}
	err = db.GetScan(ctx, &evidence, `SELECT COUNT(*) AS tasks,COUNT(DISTINCT local_resource_no) AS services,(SELECT COUNT(*) FROM plugin_idc_resource_provision_outcome_receipt) AS receipts,(SELECT status_code FROM plugin_idc_resource_task WHERE task_no=?) AS old_status,(SELECT status_code FROM plugin_idc_resource_provision_credential WHERE task_no=?) AS credential_status FROM plugin_idc_resource_task`, old.ProviderTaskNo, old.ProviderTaskNo)
	if err != nil || evidence.Tasks != 2 || evidence.Services != 1 || evidence.Receipts != 1 || evidence.OldStatus != "failed" || evidence.CredentialStatus != "revoked" || secrets.createCalls() != 2 {
		t.Fatalf("history=%+v secrets=%d err=%v", evidence, secrets.createCalls(), err)
	}
}
