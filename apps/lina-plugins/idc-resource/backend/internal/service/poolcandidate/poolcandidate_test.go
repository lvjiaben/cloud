package poolcandidate

import (
	"testing"

	"lina-plugin-idc-core/backend/cap/poolsourcecap"
)

func TestCandidateRevisionChangesWithOwnerVersion(t *testing.T) {
	row := projection{
		PoolNo: "pool_a", ChannelNo: "channel_a", DisplayName: "Pool A", ResourceKind: "virtual_server",
		PoolVersion: 3, ChannelName: "Channel", ProviderID: "zjmf-cloud",
		ProviderPluginID: "idc-zjmfcloud", ChannelVersion: 5,
	}
	first, err := projectCandidate(row)
	if err != nil {
		t.Fatalf("projectCandidate() error = %v", err)
	}
	if err = poolsourcecap.ValidateCandidate(first); err != nil {
		t.Fatalf("ValidateCandidate() error = %v", err)
	}
	row.PoolVersion++
	second, err := projectCandidate(row)
	if err != nil {
		t.Fatalf("projectCandidate() second error = %v", err)
	}
	if first.Revision == second.Revision {
		t.Fatal("candidate revision did not change with pool version")
	}
}
