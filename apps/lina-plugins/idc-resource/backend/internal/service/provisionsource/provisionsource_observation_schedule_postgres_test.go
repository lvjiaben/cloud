// This file verifies real durable scheduling uses the owner's database clock,
// while long worker leases protect execution without delaying read-only queries.

package provisionsource

import (
	"context"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

func TestProvisionObservationScheduleUsesDatabaseTimeNotLeaseExpiry(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	source, err := New(database, newProvisionSecretSource(t), principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	created, err := source.CreateTask(ctx, frozenProvisionSourceRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := source.tasks.Claim(ctx, resourcetask.ClaimInput{WorkerID: "schedule-test", Limit: 1, LeaseDuration: 2 * time.Minute})
	if err != nil || len(claims) != 1 {
		t.Fatalf("write claim count=%d err=%v", len(claims), err)
	}
	claim := claims[0]
	if err = source.tasks.Transition(ctx, resourcetask.TransitionInput{
		Lease: resourcetask.LeaseRef{TaskNo: claim.TaskNo, Token: claim.LeaseToken, AttemptNo: claim.AttemptNo},
		To:    resourcetask.StatusAccepted, Evidence: resourcetask.TransitionEvidence{AcceptanceProven: true},
		ResourceRef: "41", Upstream: &resourcetask.UpstreamTask{TaskRef: "71", ResourceRef: "41", Status: resourcecap.TaskStatusAccepted},
	}); err != nil {
		t.Fatalf("accepted task must request owner-clock scheduling: %v", err)
	}
	assertShortObservationSchedule(t, ctx, database, created.ProviderTaskNo, 5, 6)
	// Move only this random-schema fixture's clock precondition, without
	// changing acceptance, counters, bindings or any business outcome.
	makeObservationDue(t, ctx, database, created.ProviderTaskNo)
	observations, err := source.tasks.ClaimObservations(ctx, resourcetask.ObservationClaimInput{WorkerID: "schedule-test", Limit: 1, LeaseDuration: 2 * time.Minute})
	if err != nil || len(observations) != 1 {
		t.Fatalf("observation claim count=%d err=%v", len(observations), err)
	}
	observation := observations[0]
	lease := resourcetask.ObservationLeaseRef{TaskNo: observation.TaskNo, Token: observation.LeaseToken, Version: observation.LeaseVersion}
	if err = source.tasks.ObserveTask(ctx, resourcetask.ObservationInput{Lease: lease, Result: resourcecap.OperationResult{Status: resourcecap.TaskStatusRunning, ExternalTaskRef: "71", ExternalResourceRef: "41"}}); err != nil {
		t.Fatalf("running observation must retain a short owner-clock schedule: %v", err)
	}
	assertShortObservationSchedule(t, ctx, database, created.ProviderTaskNo, 5, 6)
	makeObservationDue(t, ctx, database, created.ProviderTaskNo)
	observations, err = source.tasks.ClaimObservations(ctx, resourcetask.ObservationClaimInput{WorkerID: "schedule-test", Limit: 1, LeaseDuration: 2 * time.Minute})
	if err != nil || len(observations) != 1 {
		t.Fatalf("second observation claim count=%d err=%v", len(observations), err)
	}
	observation = observations[0]
	lease = resourcetask.ObservationLeaseRef{TaskNo: observation.TaskNo, Token: observation.LeaseToken, Version: observation.LeaseVersion}
	foreign := lease
	foreign.Version++
	if err = source.tasks.RescheduleObservation(ctx, resourcetask.ObservationRescheduleInput{Lease: foreign}); err == nil {
		t.Fatal("foreign lease changed the schedule")
	}
	past := time.Now().UTC().Add(-time.Minute)
	if err = source.tasks.RescheduleObservation(ctx, resourcetask.ObservationRescheduleInput{Lease: lease, NextAt: past}); err == nil {
		t.Fatal("explicit expired schedule was accepted")
	}
	if err = source.tasks.RescheduleObservation(ctx, resourcetask.ObservationRescheduleInput{Lease: lease}); err != nil {
		t.Fatalf("query failure must use current database time: %v", err)
	}
	assertShortObservationSchedule(t, ctx, database, created.ProviderTaskNo, 5, 6)
	makeObservationDue(t, ctx, database, created.ProviderTaskNo)
	observations, err = source.tasks.ClaimObservations(ctx, resourcetask.ObservationClaimInput{WorkerID: "schedule-test", Limit: 1, LeaseDuration: 2 * time.Minute})
	if err != nil || len(observations) != 1 {
		t.Fatalf("consecutive failure claim count=%d err=%v", len(observations), err)
	}
	observation = observations[0]
	lease = resourcetask.ObservationLeaseRef{TaskNo: observation.TaskNo, Token: observation.LeaseToken, Version: observation.LeaseVersion}
	if err = source.tasks.RescheduleObservation(ctx, resourcetask.ObservationRescheduleInput{Lease: lease}); err != nil {
		t.Fatal(err)
	}
	assertShortObservationSchedule(t, ctx, database, created.ProviderTaskNo, 10, 12)
}

func makeObservationDue(t *testing.T, ctx context.Context, database gdb.DB, taskNo string) {
	t.Helper()
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_task SET next_observe_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE task_no=?`, taskNo); err != nil {
		t.Fatal(err)
	}
}

func assertShortObservationSchedule(t *testing.T, ctx context.Context, database gdb.DB, taskNo string, minimum, maximum int) {
	t.Helper()
	var row struct {
		Delay         float64
		Status        string
		WriteAttempts int
		Unlocked      bool
	}
	if err := database.GetScan(ctx, &row, `SELECT EXTRACT(EPOCH FROM next_observe_at-CURRENT_TIMESTAMP) AS delay,status_code AS status,attempt_count AS write_attempts,(locked_by='' AND lease_expires_at IS NULL) AS unlocked FROM plugin_idc_resource_task WHERE task_no=?`, taskNo); err != nil {
		t.Fatal(err)
	}
	if row.Delay < float64(minimum)-1 || row.Delay > float64(maximum) || !row.Unlocked || row.WriteAttempts != 1 || (row.Status != string(resourcetask.StatusAccepted) && row.Status != string(resourcetask.StatusRunning)) {
		t.Fatalf("query schedule contains a lease delay or changed the write: %+v", row)
	}
}
