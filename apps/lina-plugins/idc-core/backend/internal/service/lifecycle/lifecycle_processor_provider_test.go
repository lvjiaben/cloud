// This file verifies destructive lifecycle eligibility and resume provenance rules.

package lifecycle

import (
	"testing"
	"time"
)

func TestProviderLifecycleEligibleRestrictsTerminationToStableRuntime(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	policyNo := "lifecycle-policy-1"
	policyVersion := int64(1)
	expiresAt := now.Add(-time.Hour)
	base := taskState{
		ActionCode:                    actionTerminate,
		PolicyNo:                      policyNo,
		PolicyVersion:                 policyVersion,
		ExpectedServiceVersion:        3,
		ServiceVersion:                3,
		ExpectedEntitlementExpiresAt:  expiresAt,
		EntitlementExpiresAt:          &expiresAt,
		EntitlementStatusCode:         entitlementStatusExpired,
		CurrentLifecyclePolicyNo:      &policyNo,
		CurrentLifecyclePolicyVersion: &policyVersion,
		ChannelNo:                     pointerTo("channel-1"),
	}

	for _, status := range []runtimeStatus{
		runtimeStatusRunning, runtimeStatusStopped, runtimeStatusSuspended,
	} {
		state := base
		state.RuntimeStatusCode = status
		if !providerLifecycleEligible(&state, now) {
			t.Fatalf("stable runtime %q was rejected for termination", status)
		}
	}
	for _, status := range []runtimeStatus{
		"pending_provision", "provisioning", "resuming", "changing",
		runtimeStatusTerminating, runtimeStatusTerminated, "unknown",
	} {
		state := base
		state.RuntimeStatusCode = status
		if providerLifecycleEligible(&state, now) {
			t.Fatalf("unstable runtime %q was accepted for termination", status)
		}
	}
}

func TestProviderLifecycleEligibleRequiresCurrentAutomaticSuspendForResume(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	policyNo := "lifecycle-policy-1"
	policyVersion := int64(1)
	expiresAt := now.Add(time.Hour)
	state := taskState{
		ActionCode:                    actionResume,
		PolicyNo:                      policyNo,
		PolicyVersion:                 policyVersion,
		ExpectedServiceVersion:        3,
		ServiceVersion:                4,
		ExpectedEntitlementExpiresAt:  expiresAt,
		EntitlementExpiresAt:          &expiresAt,
		EntitlementStatusCode:         entitlementStatusActive,
		RuntimeStatusCode:             runtimeStatusSuspended,
		CurrentLifecyclePolicyNo:      &policyNo,
		CurrentLifecyclePolicyVersion: &policyVersion,
		ChannelNo:                     pointerTo("channel-1"),
	}
	if providerLifecycleEligible(&state, now) {
		t.Fatal("resume accepted a suspension without current lifecycle provenance")
	}
	state.LifecycleSuspended = true
	if !providerLifecycleEligible(&state, now) {
		t.Fatal("resume rejected the current authoritative lifecycle suspension")
	}
}

func pointerTo(value string) *string { return &value }
