// This file separates commercial quote eligibility from free write authority.
package customeroperations

import (
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
	"time"
)

func TestCommercialCapabilitiesNeverAuthorizeFreeWrites(t *testing.T) {
	actions := []serveroperationresourcecap.Action{serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionTrafficAdd, serveroperationresourcecap.ActionChangeIP, serveroperationresourcecap.ActionResize}
	now := time.Now()
	capabilities := []serveroperationresourcecap.CapabilityProjection{{Actions: actions, ObservedAt: now}}
	choices := &Options{Accessories: &serveroperationresourcecap.AccessoryProjection{SnapshotNo: "complete", Actions: actions, ExpiresAt: now.Add(time.Minute)}}
	observation := &resourcecap.ResourceSnapshot{State: resourcecap.ResourceStateRunning}
	cases := []struct {
		name             string
		canOperate, busy bool
		runtime          resourcecap.ResourceState
		reason           string
	}{
		{"eligible", true, false, resourcecap.ResourceStateRunning, ""},
		{"stopped", true, false, resourcecap.ResourceStateStopped, ""},
		{"unknown", true, false, resourcecap.ResourceStateUnknown, "runtime_unconfirmed"},
		{"busy", true, true, resourcecap.ResourceStateRunning, "operation_in_progress"},
		{"inactive", false, false, resourcecap.ResourceStateRunning, "entitlement_unavailable"},
		{"suspension reason not proved", true, false, resourcecap.ResourceStateSuspended, "runtime_state_conflict"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			observation.State = test.runtime
			items := commercialCapabilities(test.canOperate, test.busy, observation, nil, capabilities, nil, choices)
			if len(items) != 5 {
				t.Fatal("missing commercial entry")
			}
			for _, item := range items {
				if !item.Supported || item.CanQuote != (test.reason == "") || item.ReasonCode != test.reason {
					t.Fatalf("wrong quote eligibility: %#v", item)
				}
				if allowedState(test.runtime, item.Action) {
					t.Fatal("commercial action became a free executable action")
				}
			}
		})
	}
	observation.State = resourcecap.ResourceStateRunning
	for _, test := range []struct {
		capabilities []serveroperationresourcecap.CapabilityProjection
		choices      *Options
		reason       string
	}{
		{nil, choices, "capability_unavailable"},
		{[]serveroperationresourcecap.CapabilityProjection{{ObservedAt: now}}, choices, "unsupported"},
		{capabilities, nil, "choices_unavailable"},
		{capabilities, &Options{Accessories: &serveroperationresourcecap.AccessoryProjection{SnapshotNo: "expired", Actions: actions, ExpiresAt: now.Add(-time.Second)}}, "choices_unavailable"},
	} {
		for _, item := range commercialCapabilities(true, false, observation, nil, test.capabilities, nil, test.choices) {
			if item.CanQuote || item.ReasonCode != test.reason {
				t.Fatal("unverified commercial input advertised", item)
			}
		}
	}
}
