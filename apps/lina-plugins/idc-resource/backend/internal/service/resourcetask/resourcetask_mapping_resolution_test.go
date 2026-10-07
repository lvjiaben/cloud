// This file verifies bounded and fail-closed resource mapping request validation.

package resourcetask

import (
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// TestResourceResolutionRequestsAreBoundedAndUnique verifies the resolver
// cannot be turned into an unbounded or ambiguous mapping query.
func TestResourceResolutionRequestsAreBoundedAndUnique(t *testing.T) {
	t.Parallel()
	requests := []ResourceResolutionRequest{
		{
			TaskNo: "task-1", Capability: resourcecap.CapabilityStart,
			LocalResourceNo: "resource-1", ChannelNo: "channel-1", PoolNo: "pool-1",
		},
		{
			TaskNo: "task-2", Capability: resourcecap.CapabilityStop,
			LocalResourceNo: "resource-1", ChannelNo: "channel-1", PoolNo: "pool-1",
		},
		{
			TaskNo: "task-3", Capability: resourcecap.CapabilityProvision,
			LocalResourceNo: "resource-3", ChannelNo: "channel-1", PoolNo: "pool-1",
		},
	}
	resources, err := validateResourceResolutionRequests(requests)
	if err != nil || len(resources) != 1 || resources[0] != "resource-1" {
		t.Fatalf("valid mapping batch was not deduplicated: resources=%v err=%v", resources, err)
	}

	duplicate := append([]ResourceResolutionRequest(nil), requests...)
	duplicate[1].TaskNo = duplicate[0].TaskNo
	if _, err = validateResourceResolutionRequests(duplicate); err == nil {
		t.Fatal("duplicate task mapping request was accepted")
	}

	oversized := make([]ResourceResolutionRequest, maximumClaimBatchSize+1)
	if _, err = validateResourceResolutionRequests(oversized); err == nil {
		t.Fatal("oversized mapping request batch was accepted")
	}
}
