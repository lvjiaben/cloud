// These isolated database tests require exact immutable owner proof, reject
// conflicting bindings, and retain the existing receipt and credential fences.
package provisionsource

import (
	"errors"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestNonSubmissionRecoveryRequiresBoundProofPostgreSQL(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "different task", "different statement", "accepted binding"} {
		t.Run(mode, func(t *testing.T) {
			ctx, db := newProvisionSourceDatabase(t)
			seedProvisionRoute(t, ctx, db)
			source, err := New(db, newProvisionSecretSource(t), principalReady, nil, nil, provisionManagedBuilder{})
			if err != nil {
				t.Fatal(err)
			}
			request := frozenProvisionSourceRequest(t)
			old, err := source.CreateTask(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code='failed',failure_code=?,result_unknown=FALSE,finished_at=CURRENT_TIMESTAMP,version=version+1 WHERE task_no=?`, provisionresourcecap.FailureRequestNotSent, old.ProviderTaskNo); err != nil {
				t.Fatal(err)
			}
			proof, err := resourcecap.NewProvisionNonSubmissionEvidence(old.ProviderTaskNo, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if mode == "different task" {
				proof.Reference = "task_another_invocation"
			}
			if mode == "different statement" {
				proof.Summary = "No response was recorded"
			}
			if mode != "missing" {
				if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_task_evidence(tenant_id,evidence_no,task_no,source_kind,evidence_kind,reference,summary,observed_at) VALUES(0,'evidence_owner_no_send',?,'provider_resource',?,?,?,?)`, old.ProviderTaskNo, proof.Kind, proof.Reference, proof.Summary, proof.ObservedAt); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "accepted binding" {
				if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_upstream_task_binding(tenant_id,binding_no,task_no,channel_no,external_task_ref,last_observed_at) VALUES(0,'binding_owner_no_send',?,?,'accepted-71',CURRENT_TIMESTAMP)`, old.ProviderTaskNo, old.ChannelNo); err != nil {
					t.Fatal(err)
				}
			}
			read := provisionresourcecap.TaskReadRequest{ProviderTaskNo: old.ProviderTaskNo, Request: request}
			failed, err := source.ReadTask(ctx, read)
			if mode != "valid" {
				if !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
					t.Fatalf("unproven failure became recoverable: %+v %v", failed, err)
				}
				if _, err = source.RecoverRejected(ctx, read); !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
					t.Fatal("invalid proof recovered")
				}
				return
			}
			if err != nil || failed.Status != provisionresourcecap.TaskStatusFailed {
				t.Fatalf("owner-proven failure unavailable: %+v %v", failed, err)
			}
			if _, mutationErr := db.Exec(ctx, `UPDATE plugin_idc_resource_task_evidence SET summary='changed proof' WHERE task_no=? AND evidence_kind=?`, old.ProviderTaskNo, resourcecap.EvidenceKindRequestNotSent); mutationErr == nil {
				t.Fatal("owner non-submission proof was mutable")
			}
			if _, err = source.RecoverRejected(ctx, read); !errors.Is(err, provisionresourcecap.ErrTaskConflict) {
				t.Fatal("unreported failure recovered")
			}
			if err = source.ReportOutcome(ctx, provisionresourcecap.OutcomeReport{ProviderTaskNo: old.ProviderTaskNo, Request: request, Status: provisionresourcecap.OutcomeStatusFailed, TaskEvidenceDigest: failed.EvidenceDigest, CoreEvidenceDigest: strings.Repeat("e", 64)}); err != nil {
				t.Fatal(err)
			}
			next, err := source.RecoverRejected(ctx, read)
			if err != nil || next.ProviderTaskNo == old.ProviderTaskNo {
				t.Fatalf("reported no-send failure could not recover: %+v %v", next, err)
			}
			replay, err := source.RecoverRejected(ctx, read)
			if err != nil || replay.ProviderTaskNo != next.ProviderTaskNo {
				t.Fatal("recovery was not idempotent")
			}
		})
	}
}
