// This file verifies observation claim validation and authoritative result mapping.

package resourcetask

import (
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

func TestObservationClaimInputUsesWriteClaimBounds(t *testing.T) {
	valid := ObservationClaimInput{WorkerID: "observer-1", Limit: 32, LeaseDuration: time.Minute}
	if err := validateObservationClaimInput(valid); err != nil {
		t.Fatalf("valid observation claim was rejected: %v", err)
	}
	for _, input := range []ObservationClaimInput{
		{},
		{WorkerID: "observer/1", Limit: 1, LeaseDuration: time.Minute},
		{WorkerID: "observer-1", Limit: maximumClaimBatchSize + 1, LeaseDuration: time.Minute},
		{WorkerID: "observer-1", Limit: 1, LeaseDuration: maximumLeaseDuration + time.Second},
	} {
		if err := validateObservationClaimInput(input); err == nil {
			t.Fatalf("invalid observation claim was accepted: %+v", input)
		}
	}
}

// TestObservationBackoffIsDeterministicAndBounded verifies read-only retries
// grow independently of Provider write attempts and never exceed the cap.
func TestObservationBackoffIsDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	previous := time.Duration(0)
	for attempt := 1; attempt <= 20; attempt++ {
		first := nextObservationAt("task-observation-backoff", attempt, now)
		second := nextObservationAt("task-observation-backoff", attempt, now)
		if !first.Equal(second) {
			t.Fatalf("attempt %d produced unstable schedule %s and %s", attempt, first, second)
		}
		delay := first.Sub(now)
		if delay < minimumObservationBackoff || delay > maximumObservationBackoff {
			t.Fatalf("attempt %d delay %s is outside [%s,%s]", attempt, delay,
				minimumObservationBackoff, maximumObservationBackoff)
		}
		if delay < previous {
			t.Fatalf("attempt %d delay regressed from %s to %s", attempt, previous, delay)
		}
		previous = delay
	}
	if got := nextObservationAt("task-observation-backoff", 100, now).Sub(now); got != maximumObservationBackoff {
		t.Fatalf("large attempt delay=%s, want cap %s", got, maximumObservationBackoff)
	}
}

func TestObservationResultRequiresExactIdentityAndSchedule(t *testing.T) {
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	next := now.Add(time.Minute)
	row := &entity.Task{TaskNo: "task-1", StatusCode: string(StatusAccepted)}
	binding := &entity.UpstreamTaskBinding{
		TaskNo: "task-1", ExternalTaskRef: "upstream-task-1", ExternalResourceRef: "resource-1",
	}
	input := ObservationInput{
		Result: resourcecap.OperationResult{
			Status: resourcecap.TaskStatusRunning, ExternalTaskRef: "upstream-task-1",
			ExternalResourceRef: "resource-1",
		},
		NextAt: &next,
	}
	to, err := validateObservationResult(input, row, binding, now)
	if err != nil || to != StatusRunning {
		t.Fatalf("valid observation rejected: to=%q err=%v", to, err)
	}

	wrongTask := input
	wrongTask.Result.ExternalTaskRef = "upstream-task-other"
	if _, err = validateObservationResult(wrongTask, row, binding, now); err == nil {
		t.Fatal("mismatched upstream task reference was accepted")
	}
	missingTask := input
	missingTask.Result.ExternalTaskRef = ""
	if _, err = validateObservationResult(missingTask, row, binding, now); err == nil {
		t.Fatal("missing observed upstream task reference was accepted")
	}
	wrongResource := input
	wrongResource.Result.ExternalResourceRef = "resource-other"
	if _, err = validateObservationResult(wrongResource, row, binding, now); err == nil {
		t.Fatal("mismatched upstream resource reference was accepted")
	}
	missingSchedule := input
	missingSchedule.NextAt = nil
	if _, err = validateObservationResult(missingSchedule, row, binding, now); err == nil {
		t.Fatal("non-terminal observation without a schedule was accepted")
	}
}

func TestObservationSameStateRequiresValidObservationEvidence(t *testing.T) {
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	next := now.Add(time.Minute)
	row := &entity.Task{TaskNo: "task-1", StatusCode: string(StatusAccepted)}
	binding := &entity.UpstreamTaskBinding{TaskNo: "task-1", ExternalTaskRef: "upstream-task-1"}
	to, err := validateObservationResult(ObservationInput{
		Result: resourcecap.OperationResult{
			Status: resourcecap.TaskStatusAccepted, ExternalTaskRef: "upstream-task-1",
		},
		NextAt: &next,
	}, row, binding, now)
	if err != nil || to != StatusAccepted {
		t.Fatalf("validated same-state observation rejected: to=%q err=%v", to, err)
	}
	if err = ValidateTransition(StatusAccepted, StatusAccepted, TransitionEvidence{
		ObservationProven: true,
	}); err == nil {
		t.Fatal("blind same-state transition bypassed the observation path")
	}
}

func TestUnknownObservationPreservesLastKnownLocalState(t *testing.T) {
	failure := &resourcecap.ProviderFailure{
		Code: "TASK_QUERY_UNKNOWN", Message: "provider task state is unavailable", ResultUnknown: true,
	}
	for _, current := range []Status{StatusAccepted, StatusRunning, StatusResultUnknown} {
		to, err := localStatusForObservation(current, resourcecap.TaskStatusUnknown)
		if err != nil || to != current {
			t.Fatalf("unknown observation changed %q to %q: %v", current, to, err)
		}
		code, summary := observationFailure(resourcecap.OperationResult{
			Status: resourcecap.TaskStatusUnknown, Failure: failure,
		})
		if code != failure.Code || summary != failure.Message {
			t.Fatalf("unknown observation failure projection diverged: %q %q", code, summary)
		}
	}
}

func TestAcceptedObservationDoesNotRegressRunningTask(t *testing.T) {
	to, err := localStatusForObservation(StatusRunning, resourcecap.TaskStatusAccepted)
	if err != nil || to != StatusRunning {
		t.Fatalf("accepted observation regressed running task: to=%q err=%v", to, err)
	}
}

func TestTerminalObservationRejectsFurtherSchedule(t *testing.T) {
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	next := now.Add(time.Minute)
	row := &entity.Task{TaskNo: "task-1", StatusCode: string(StatusRunning)}
	binding := &entity.UpstreamTaskBinding{TaskNo: "task-1", ExternalTaskRef: "upstream-task-1"}
	input := ObservationInput{
		Result: resourcecap.OperationResult{
			Status: resourcecap.TaskStatusSucceeded, ExternalTaskRef: "upstream-task-1",
		},
		NextAt: &next,
	}
	if _, err := validateObservationResult(input, row, binding, now); err == nil {
		t.Fatal("terminal observation accepted another schedule")
	}
	input.NextAt = nil
	to, err := validateObservationResult(input, row, binding, now)
	if err != nil || to != StatusSucceeded {
		t.Fatalf("terminal observation rejected: to=%q err=%v", to, err)
	}
}
