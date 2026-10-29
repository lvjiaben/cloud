// This file verifies local owner coordination does not inherit remote waits,
// while stopped decisions and other providers retain their original policy.

package productfulfillment

import (
	"testing"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

func TestResourceOwnerProgressSchedulesOnlySafeLocalAdvances(t *testing.T) {
	tests := []struct {
		name              string
		owner             string
		action            productfulfillmentcap.Action
		externalReference string
		previousStatus    string
		status            productfulfillmentcap.ResultStatus
		scheduled         bool
		wantSeconds       int
	}{
		{name: "initial local absence advances immediately", owner: provisionresourcecap.OwnerID, action: productfulfillmentcap.ActionQuery, status: productfulfillmentcap.StatusNotFound, scheduled: true, wantSeconds: 0},
		{name: "accepted local task is read promptly", owner: provisionresourcecap.OwnerID, action: productfulfillmentcap.ActionSubmit, status: productfulfillmentcap.StatusAccepted, scheduled: true, wantSeconds: resourceOwnerProgressIntervalSeconds},
		{name: "running local task is read promptly", owner: provisionresourcecap.OwnerID, action: productfulfillmentcap.ActionQuery, externalReference: "task-existing", status: productfulfillmentcap.StatusRunning, scheduled: true, wantSeconds: resourceOwnerProgressIntervalSeconds},
		{name: "repeated running retains observation window", owner: provisionresourcecap.OwnerID, action: productfulfillmentcap.ActionQuery, externalReference: "task-existing", previousStatus: "running", status: productfulfillmentcap.StatusRunning, scheduled: true, wantSeconds: resourceOwnerProgressIntervalSeconds},
		{name: "local preparation retains bounded retry", owner: provisionresourcecap.OwnerID, action: productfulfillmentcap.ActionSubmit, scheduled: true, wantSeconds: resourceOwnerProgressIntervalSeconds},
		{name: "known task disappearance is never an immediate resubmit", owner: provisionresourcecap.OwnerID, action: productfulfillmentcap.ActionQuery, externalReference: "task-existing", status: productfulfillmentcap.StatusNotFound, scheduled: true, wantSeconds: resourceOwnerProgressIntervalSeconds},
		{name: "manual or unknown decision stays stopped", owner: provisionresourcecap.OwnerID, action: productfulfillmentcap.ActionQuery, status: productfulfillmentcap.StatusNotFound, wantSeconds: retryIntervalSeconds},
		{name: "remote connector retains original delay", owner: "remote-owner", action: productfulfillmentcap.ActionQuery, status: productfulfillmentcap.StatusNotFound, scheduled: true, wantSeconds: retryIntervalSeconds},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claim := &claimedFulfillment{EntitlementOwnerID: test.owner, Action: test.action, PreviousExternalReference: test.externalReference, PreviousStatus: test.previousStatus}
			var result *productfulfillmentcap.Result
			if test.status != "" {
				result = &productfulfillmentcap.Result{Status: test.status}
			}
			decision := workerDecision{ScheduleNext: test.scheduled}
			if got := resourceWorkerScheduleSeconds(claim, decision, result); got != test.wantSeconds {
				t.Fatalf("coordination delay=%d want=%d", got, test.wantSeconds)
			}
			if decision.ScheduleNext != test.scheduled || claim.PreviousExternalReference != test.externalReference {
				t.Fatal("scheduling changed a stopped decision or resource identity")
			}
		})
	}
}

// A supplier can legitimately need over a minute to build a VM. Fast local
// observation must not burn the unchanged finite query budget before then.
func TestSlowAcceptedResourceKeepsItsObservationWindow(t *testing.T) {
	claim := &claimedFulfillment{
		EntitlementOwnerID:        provisionresourcecap.OwnerID,
		Action:                    productfulfillmentcap.ActionQuery,
		PreviousExternalReference: "task-existing",
		PreviousStatus:            "accepted", PreviousPhase: "post_submit_query",
		SubmitAttempts: 1, QueryAttempts: 1,
	}
	elapsed := resourceOwnerProgressIntervalSeconds
	for claim.QueryAttempts < resourceOwnerMaximumQueryAttempts {
		claim.QueryAttempts++
		status := productfulfillmentcap.StatusRunning
		if elapsed >= 70 {
			status = productfulfillmentcap.StatusSucceeded
		}
		result := &productfulfillmentcap.Result{Status: status}
		decision := decideWorkerResult(claim.PreviousStatus, claim.PreviousPhase, claim.Action, result, false, true, false, false, resourceWorkerQueryAttempts(claim), claim.SubmitAttempts)
		if decision.IntentStatus == "completed" {
			return
		}
		if !decision.ScheduleNext {
			t.Fatalf("healthy running task exhausted observation at %ds (queries=%d)", elapsed, claim.QueryAttempts)
		}
		elapsed += resourceWorkerScheduleSeconds(claim, decision, result)
		claim.PreviousStatus, claim.PreviousPhase = decision.TaskStatus, decision.PhaseCode
	}
	t.Fatal("slow accepted resource was never observed as complete")
}

// Local owner reads have a bounded five-minute window even after many healthy
// observations, and a final known success remains observable at its boundary.
func TestResourceObservationBudgetStaysBounded(t *testing.T) {
	claim := &claimedFulfillment{EntitlementOwnerID: provisionresourcecap.OwnerID, PreviousPhase: "post_submit_query", PreviousStatus: "running", Action: productfulfillmentcap.ActionQuery, SubmitAttempts: 1}
	for _, count := range []int{1, maximumQueryAttempts, resourceOwnerMaximumQueryAttempts - 1} {
		claim.QueryAttempts = count
		d := decideWorkerResult(claim.PreviousStatus, claim.PreviousPhase, claim.Action, &productfulfillmentcap.Result{Status: productfulfillmentcap.StatusRunning}, false, true, false, false, resourceWorkerQueryAttempts(claim), 1)
		if !d.ScheduleNext {
			t.Fatalf("local read %d exhausted observation window", count)
		}
	}
	claim.QueryAttempts = resourceOwnerMaximumQueryAttempts
	d := decideWorkerResult(claim.PreviousStatus, claim.PreviousPhase, claim.Action, &productfulfillmentcap.Result{Status: productfulfillmentcap.StatusRunning}, false, true, false, false, resourceWorkerQueryAttempts(claim), 1)
	if d.ScheduleNext || d.IntentStatus != "manual_pending" {
		t.Fatal("unbounded local observation", d)
	}
	claim.EntitlementOwnerID = "another-owner"
	if resourceWorkerQueryAttempts(claim) != claim.QueryAttempts {
		t.Fatal("remote query budget changed")
	}
	claim.EntitlementOwnerID = provisionresourcecap.OwnerID
	claim.PreviousPhase = "pre_submit_query"
	if resourceWorkerQueryAttempts(claim) != claim.QueryAttempts {
		t.Fatal("preparation budget changed")
	}
}
