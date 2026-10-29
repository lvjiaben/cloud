// This file verifies capability-specific resource reconciliation and evidence numbering.

package resourcetask

import (
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// TestResourceObservationConvergenceIsCapabilitySpecific rejects generic
// state-only convergence for parameter-specific or transient operations.
func TestResourceObservationConvergenceIsCapabilitySpecific(t *testing.T) {
	t.Parallel()
	cases := []struct {
		capability resourcecap.CapabilityCode
		parameters string
		snapshot   resourcecap.ResourceSnapshot
		want       bool
	}{
		{capability: resourcecap.CapabilityStart, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateRunning}, want: true},
		{capability: resourcecap.CapabilityResume, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateActive}, want: true},
		{capability: resourcecap.CapabilityStop, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateStopped}, want: true},
		{capability: resourcecap.CapabilitySuspend, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateSuspended}, want: true},
		{capability: resourcecap.CapabilityTerminate, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateTerminated}, want: true},
		{capability: resourcecap.CapabilityChangeIP, parameters: `{"ip_num":2}`, snapshot: resourcecap.ResourceSnapshot{
			State: resourcecap.ResourceStateRunning, Data: []byte(`{"ip_count":2}`),
		}, want: false},
		{capability: resourcecap.CapabilityChangeIP, parameters: `{"ip_num":2}`, snapshot: resourcecap.ResourceSnapshot{
			State: resourcecap.ResourceStateRunning, Data: []byte(`{"ip_count":1}`),
		}, want: false},
		{capability: resourcecap.CapabilityReboot, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateRunning}, want: false},
		{capability: resourcecap.CapabilityReinstall, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateRunning}, want: false},
		{capability: resourcecap.CapabilityResize, snapshot: resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateRunning}, want: false},
	}
	for _, testCase := range cases {
		if got := resourceObservationConverges(
			testCase.capability,
			testCase.parameters,
			testCase.snapshot,
		); got != testCase.want {
			t.Fatalf("convergence for %s/%s = %v, want %v", testCase.capability, testCase.snapshot.State, got, testCase.want)
		}
	}
}

// TestNewTaskEvidenceNumber verifies the stable opaque evidence number format.
func TestNewTaskEvidenceNumber(t *testing.T) {
	t.Parallel()
	number, err := newTaskEvidenceNumber(zeroReader{})
	if err != nil || number != "evidence_00000000000000000000000000000000" {
		t.Fatalf("unexpected evidence number: number=%q err=%v", number, err)
	}
}

// Equal counts (or a new-looking address list without the original task proof)
// cannot turn an unknown replacement into success and unlock another write.
func TestChangeIPObservationNeverProvesReplacementByCount(t *testing.T) {
	for _, data := range []string{`{"ip_count":2,"addresses":["192.0.2.1","192.0.2.2"]}`, `{"ip_count":2,"addresses":["192.0.2.3","192.0.2.4"]}`} {
		for _, state := range []resourcecap.ResourceState{resourcecap.ResourceStateRunning, resourcecap.ResourceStateStopped, resourcecap.ResourceStateSuspended} {
			if resourceObservationConverges(resourcecap.CapabilityChangeIP, `{"ip_num":2}`, resourcecap.ResourceSnapshot{State: state, Data: []byte(data)}) {
				t.Fatal("unbound address observation falsely proves an IP replacement")
			}
		}
	}
}
