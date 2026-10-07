package backend

import "testing"

type productAutomationRuntimeCloserStub struct{ calls int }

func (c *productAutomationRuntimeCloserStub) Close() { c.calls++ }

func TestCloseProductAutomationRuntimeUnlessOwned(t *testing.T) {
	codec := &productAutomationRuntimeCloserStub{}
	audience := &productAutomationRuntimeCloserStub{}
	owned := false
	closeProductAutomationRuntimeUnlessOwned(&owned, codec, audience)
	if codec.calls != 1 || audience.calls != 1 {
		t.Fatalf("failed startup cleanup codec=%d audience=%d", codec.calls, audience.calls)
	}

	codec.calls = 0
	audience.calls = 0
	owned = true
	closeProductAutomationRuntimeUnlessOwned(&owned, codec, audience)
	if codec.calls != 0 || audience.calls != 0 {
		t.Fatalf("successful startup released owned runtime codec=%d audience=%d", codec.calls, audience.calls)
	}
}
