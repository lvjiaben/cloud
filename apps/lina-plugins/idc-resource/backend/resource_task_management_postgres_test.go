// This file verifies safe Provider task takeover against real PostgreSQL.
package backend

import (
	"context"
	"errors"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

func assertResourceTaskManagement(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no,
     status_code, sanitized_parameters, attempt_count, max_attempts,
     next_observe_at, result_unknown, version)
VALUES
    ('task-management-unknown', repeat('8', 64), 'server.start', 'resource-management',
     'channel-test', 'result_unknown', '{"private_parameter":"must_not_escape"}', 1, 8,
     CURRENT_TIMESTAMP, TRUE, 3),
    ('task-management-audit', repeat('9', 64), 'server.stop', 'resource-management',
     'channel-test', 'running', '{}', 1, 8, CURRENT_TIMESTAMP, FALSE, 2)`); err != nil {
		t.Fatalf("seed Provider task management rows failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task_evidence
    (evidence_no, task_no, source_kind, evidence_kind, reference, summary, observed_at)
VALUES
    ('evidence_11111111111111111111111111111111', 'task-management-unknown',
     'provider_task', 'task_observation', 'upstream-1001', 'task.unknown', CURRENT_TIMESTAMP),
    ('evidence_22222222222222222222222222222222', 'task-management-audit',
     'provider_task', 'task_observation', 'upstream-1002', 'task.running', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("seed Provider task evidence failed: %v", err)
	}

	authorizer := &providerCapacityManagementAuthorizer{actor: 5201}
	audit := &providerCapacityAuditWriter{}
	store, err := resourcetask.NewStore(database)
	if err != nil {
		t.Fatalf("construct Provider task store failed: %v", err)
	}
	management, err := resourcetask.NewManagement(store, managementcap.Runtime{
		Authorizer: authorizer, Audit: audit,
	})
	if err != nil {
		t.Fatalf("construct Provider task management failed: %v", err)
	}
	if _, invalidErr := management.List(ctx, resourcetask.TaskListInput{
		Status: "force_success", Limit: 25,
	}); !bizerr.Is(invalidErr, resourcetask.CodeTaskManagementInvalid) ||
		len(authorizer.permissions) != 1 ||
		authorizer.permissions[0] != "idc:admin:resource:task:list" ||
		authorizer.targets[0] != "provider-task-list" {
		t.Fatalf("invalid task list did not authorize first permissions=%v targets=%v err=%v",
			authorizer.permissions, authorizer.targets, invalidErr)
	}
	page, err := management.List(ctx, resourcetask.TaskListInput{
		Status: resourcetask.StatusResultUnknown, Limit: 100,
	})
	var managementTaskFound bool
	if err == nil {
		for _, item := range page.Items {
			if item.TaskNo == "task-management-unknown" && item.Version == 3 {
				managementTaskFound = true
				break
			}
		}
	}
	if err != nil || !managementTaskFound || page.NextCursor != "" {
		t.Fatalf("list Provider tasks page=%+v err=%v", page, err)
	}
	detail, err := management.Detail(ctx, "task-management-unknown")
	if err != nil || len(detail.Evidence) != 1 || detail.Evidence[0].Summary != "task.unknown" ||
		detail.Task.Takeover != nil {
		t.Fatalf("query Provider task detail=%+v err=%v", detail, err)
	}
	if len(authorizer.permissions) < 3 ||
		authorizer.permissions[1] != "idc:admin:resource:task:list" ||
		authorizer.permissions[2] != "idc:admin:resource:task:query" ||
		authorizer.targets[2] != "task-management-unknown" {
		t.Fatalf("task read authorization permissions=%v targets=%v", authorizer.permissions, authorizer.targets)
	}
	if _, err = management.TakeOver(ctx, resourcetask.ManualTakeoverInput{
		TaskNo: "task-management-audit", ExpectedVersion: 2,
		ReasonCode:         resourcetask.TakeoverReasonProviderTaskStalled,
		EvidenceNos:        []string{"evidence_22222222222222222222222222222222"},
		TransportViolation: true,
	}); !bizerr.Is(err, resourcetask.CodeTaskManagementInvalid) ||
		authorizer.permissions[len(authorizer.permissions)-1] != "idc:admin:resource:task:manual-takeover" ||
		authorizer.targets[len(authorizer.targets)-1] != "task-management-audit" {
		t.Fatalf("unsafe transport did not authorize before rejection permissions=%v targets=%v err=%v",
			authorizer.permissions, authorizer.targets, err)
	}
	if _, err = management.TakeOver(ctx, resourcetask.ManualTakeoverInput{
		TaskNo: "task-management-audit", ExpectedVersion: 2,
		ReasonCode:  resourcetask.TakeoverReasonProviderTaskStalled,
		EvidenceNos: []string{"evidence_11111111111111111111111111111111"},
	}); !bizerr.Is(err, resourcetask.CodeTaskManagementConflict) {
		t.Fatalf("cross-task evidence takeover returned %v", err)
	}
	input := resourcetask.ManualTakeoverInput{
		TaskNo: "task-management-unknown", ExpectedVersion: 3,
		ReasonCode:  resourcetask.TakeoverReasonProviderResultUnresolved,
		EvidenceNos: []string{"evidence_11111111111111111111111111111111"},
	}
	taken, err := management.TakeOver(ctx, input)
	if err != nil || taken.Task.Status != resourcetask.StatusManualTakeover ||
		taken.Task.Version != 4 || taken.Task.ResultUnknown || taken.Task.Takeover == nil ||
		taken.Task.Takeover.AssignedUserID != 5201 || len(audit.transitions) != 1 ||
		audit.transitions[0].ResultCode != "succeeded" ||
		audit.transitions[0].RequiredPermission != "idc:admin:resource:task:manual-takeover" {
		t.Fatalf("take over Provider task detail=%+v audit=%+v err=%v", taken, audit.transitions, err)
	}
	closedState, err := database.GetOne(ctx, `
SELECT next_observe_at, lease_expires_at, observation_claimed_at, locked_by
FROM plugin_idc_resource_task
WHERE task_no = 'task-management-unknown'`)
	if err != nil || !closedState["next_observe_at"].IsNil() ||
		!closedState["lease_expires_at"].IsNil() ||
		!closedState["observation_claimed_at"].IsNil() ||
		closedState["locked_by"].String() != "" {
		t.Fatalf("manual takeover retained automation claims state=%v err=%v", closedState, err)
	}
	if replay, replayErr := management.TakeOver(ctx, input); replayErr != nil ||
		replay.Task.Takeover == nil || replay.Task.Takeover.TakeoverNo != taken.Task.Takeover.TakeoverNo ||
		len(audit.transitions) != 1 {
		t.Fatalf("replay Provider task takeover detail=%+v audit=%d err=%v", replay, len(audit.transitions), replayErr)
	}
	if _, err = management.TakeOver(ctx, resourcetask.ManualTakeoverInput{
		TaskNo: input.TaskNo, ExpectedVersion: input.ExpectedVersion,
		ReasonCode:  resourcetask.TakeoverReasonCompensationRequired,
		EvidenceNos: input.EvidenceNos,
	}); !bizerr.Is(err, resourcetask.CodeTaskManagementConflict) {
		t.Fatalf("changed Provider task takeover returned %v", err)
	}

	failingAuditManagement, err := resourcetask.NewManagement(store, managementcap.Runtime{
		Authorizer: authorizer, Audit: &providerCapacityAuditWriter{err: errors.New("audit unavailable")},
	})
	if err != nil {
		t.Fatalf("construct failing-audit task management failed: %v", err)
	}
	if _, err = failingAuditManagement.TakeOver(ctx, resourcetask.ManualTakeoverInput{
		TaskNo: "task-management-audit", ExpectedVersion: 2,
		ReasonCode:  resourcetask.TakeoverReasonProviderTaskStalled,
		EvidenceNos: []string{"evidence_22222222222222222222222222222222"},
	}); !bizerr.Is(err, resourcetask.CodeTaskManagementUnavailable) {
		t.Fatalf("audit failure Provider task takeover returned %v", err)
	}
	state, err := database.GetOne(ctx, `
SELECT status_code, version FROM plugin_idc_resource_task
WHERE task_no = 'task-management-audit'`)
	if err != nil || state["status_code"].String() != "running" || state["version"].Int64() != 2 {
		t.Fatalf("audit failure did not roll back task state=%v err=%v", state, err)
	}
	takeoverCount, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_resource_task_manual_takeover
WHERE task_no = 'task-management-audit'`)
	if err != nil || takeoverCount.Int() != 0 {
		t.Fatalf("audit failure retained takeover count=%d err=%v", takeoverCount.Int(), err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task_evidence SET summary = 'mutated'
WHERE evidence_no = 'evidence_11111111111111111111111111111111'`); err == nil {
		t.Fatal("immutable Provider task evidence accepted an update")
	}
	if _, err = database.Exec(ctx, `
DELETE FROM plugin_idc_resource_task_manual_takeover
WHERE task_no = 'task-management-unknown'`); err == nil {
		t.Fatal("immutable Provider task takeover accepted a delete")
	}
}
