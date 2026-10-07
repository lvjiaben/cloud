// This file verifies lease-fenced transition validation and confidential result boundaries.

package resourcetask

import (
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

func TestTransitionDetailsAreStateSpecificAndBounded(t *testing.T) {
	now := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	next := now.Add(time.Minute)
	row := &entity.Task{AttemptCount: 1, MaxAttempts: 2}
	acceptedUpstream := &UpstreamTask{TaskRef: "upstream-1", Status: resourcecap.TaskStatusAccepted}
	tests := []struct {
		name    string
		input   TransitionInput
		allowed bool
	}{
		{name: "accepted observation schedule", input: TransitionInput{
			To: StatusAccepted, NextAt: &next, Upstream: acceptedUpstream,
		}, allowed: true},
		{name: "accepted without schedule", input: TransitionInput{To: StatusAccepted}, allowed: false},
		{name: "success without diagnostic", input: TransitionInput{To: StatusSucceeded}, allowed: true},
		{name: "success with diagnostic", input: TransitionInput{
			To: StatusSucceeded, FailureCode: "Unexpected", FailureText: "not accepted",
		}, allowed: false},
		{name: "retry with redacted failure", input: TransitionInput{
			To: StatusRetryWait, FailureCode: "UpstreamBusy", FailureText: "provider rejected the request", NextAt: &next,
		}, allowed: true},
		{name: "retry with credential diagnostic", input: TransitionInput{
			To: StatusRetryWait, FailureCode: "UpstreamBusy", FailureText: "token=must-not-pass", NextAt: &next,
		}, allowed: false},
		{name: "unknown without observation schedule", input: TransitionInput{
			To: StatusResultUnknown, FailureCode: "WriteAmbiguous", FailureText: "provider result is unavailable",
		}, allowed: false},
		{name: "schedule too far", input: TransitionInput{
			To: StatusAccepted, NextAt: timePointer(now.Add(maximumTransitionScheduleDelay + time.Second)),
		}, allowed: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTransitionDetails(test.input, row, now)
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%v want=%v err=%v", err == nil, test.allowed, err)
			}
		})
	}
}

func TestUpstreamTaskMustMatchLocalTransition(t *testing.T) {
	accepted := TransitionInput{
		To: StatusAccepted, Upstream: &UpstreamTask{
			TaskRef: "upstream-1", Status: resourcecap.TaskStatusAccepted,
		},
	}
	if err := validateUpstreamTask(accepted); err != nil {
		t.Fatalf("matching upstream task rejected: %v", err)
	}
	accepted.Upstream.Status = resourcecap.TaskStatusSucceeded
	if err := validateUpstreamTask(accepted); err == nil {
		t.Fatal("mismatched upstream task status was accepted")
	}
	accepted.Upstream.TaskRef = ""
	if err := validateUpstreamTask(accepted); err == nil {
		t.Fatal("missing upstream task reference was accepted")
	}
}

func TestRetryIsRejectedAfterAttemptBudgetIsExhausted(t *testing.T) {
	now := time.Now().UTC()
	next := now.Add(time.Minute)
	err := validateTransitionDetails(TransitionInput{
		To: StatusRetryWait, FailureCode: "UpstreamBusy", FailureText: "provider rejected the request", NextAt: &next,
	}, &entity.Task{AttemptCount: 2, MaxAttempts: 2}, now)
	if err == nil {
		t.Fatal("retry was accepted after its attempt budget was exhausted")
	}
}

// TestTransitionSecretOutputsAreCapabilityBound verifies that durable output
// bindings cannot bypass capability, state, uniqueness, or policy validation.
func TestTransitionSecretOutputsAreCapabilityBound(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	next := now.Add(time.Minute)
	handle, err := secretcap.NewHandle("output-secret-ref")
	if err != nil {
		t.Fatalf("construct output handle failed: %v", err)
	}
	output := resourcecap.SecretOutput{
		Name:   "generatedCredential",
		Policy: resourcecap.SecretOutputPolicyOneTimeCredential,
		Handle: handle,
	}
	row := &entity.Task{
		TaskNo:         "task-output-1",
		CapabilityCode: string(resourcecap.CapabilityReinstall),
		AttemptCount:   1,
		MaxAttempts:    2,
	}
	accepted := TransitionInput{
		To:     StatusAccepted,
		NextAt: &next,
		Upstream: &UpstreamTask{
			TaskRef: "upstream-1",
			Status:  resourcecap.TaskStatusAccepted,
		},
		SecretOutputs: []resourcecap.SecretOutput{output},
	}
	if err = validateTransitionDetails(accepted, row, now); err != nil {
		t.Fatalf("valid confidential output rejected: %v", err)
	}

	duplicate := accepted
	duplicate.SecretOutputs = []resourcecap.SecretOutput{output, output}
	if err = validateTransitionDetails(duplicate, row, now); err == nil {
		t.Fatal("duplicate confidential output was accepted")
	}

	failed := TransitionInput{
		To:            StatusFailed,
		FailureCode:   "UPSTREAM_REJECTED",
		FailureText:   "provider rejected the request",
		SecretOutputs: []resourcecap.SecretOutput{output},
	}
	if err = validateTransitionDetails(failed, row, now); err == nil {
		t.Fatal("failed task accepted a confidential output")
	}

	wrongCapability := *row
	wrongCapability.CapabilityCode = string(resourcecap.CapabilityStart)
	if err = validateTransitionDetails(accepted, &wrongCapability, now); err == nil {
		t.Fatal("capability without confidential output permission was accepted")
	}
}

// TestProvisioningTransitionRequiresResourceAndSafeEvidence verifies that a
// successful provision cannot commit without the identity needed for mapping convergence.
func TestProvisioningTransitionRequiresResourceAndSafeEvidence(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	row := &entity.Task{
		TaskNo: "task-provision-1", CapabilityCode: string(resourcecap.CapabilityProvision),
		LocalResourceNo: "resource-provision-1", ChannelNo: "channel-1", PoolNo: "pool-1",
		AttemptCount: 1, MaxAttempts: 2,
	}
	input := TransitionInput{To: StatusSucceeded}
	if err := validateTransitionDetails(input, row, now); err == nil {
		t.Fatal("successful provision without an upstream resource identity was accepted")
	}
	input.ResourceRef = "external-provision-1"
	input.ProviderFacts = []resourcecap.Evidence{{
		Kind: "provisionResult", Reference: input.ResourceRef, Summary: "resource created",
		ObservedAt: now,
	}}
	if err := validateTransitionDetails(input, row, now); err != nil {
		t.Fatalf("successful provision with bounded evidence was rejected: %v", err)
	}
	input.ProviderFacts[0].Summary = "token=must-not-pass"
	if err := validateTransitionDetails(input, row, now); err == nil {
		t.Fatal("successful provision accepted credential-bearing evidence")
	}
}

func TestLeaseReferenceRequiresExactAttemptToken(t *testing.T) {
	valid := LeaseRef{
		TaskNo: "task-1", Token: "worker-1:00000000000000000000000000000000", AttemptNo: 1,
	}
	if err := validateLeaseRef(valid); err != nil {
		t.Fatalf("valid lease reference rejected: %v", err)
	}
	for _, invalid := range []LeaseRef{
		{},
		{TaskNo: "task-1", Token: valid.Token, AttemptNo: 0},
		{TaskNo: "task/1", Token: valid.Token, AttemptNo: 1},
		{TaskNo: "task-1", Token: "worker-1:not-hex", AttemptNo: 1},
	} {
		if err := validateLeaseRef(invalid); err == nil {
			t.Fatalf("invalid lease reference accepted: %+v", invalid)
		}
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}
