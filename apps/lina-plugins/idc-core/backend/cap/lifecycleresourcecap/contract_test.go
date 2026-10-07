// This file verifies the versioned resource lifecycle contract validation boundary.

package lifecycleresourcecap

import (
	"testing"
	"time"
)

func TestLifecycleResourceContractFailsClosed(t *testing.T) {
	t.Parallel()
	request := TaskRequest{
		LifecycleTaskNo: "LCT_0123456789abcdef0123456789abcdef",
		ServiceNo:       "service-1", ChannelNo: "channel-1", PoolNo: "pool-1",
		Action: ActionSuspend, ExpectedServiceVersion: 2,
		ExpectedExpiresAt: time.Now().UTC(),
		IdempotencyDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	if err := ValidateTaskRequest(request); err != nil {
		t.Fatalf("ValidateTaskRequest() error = %v", err)
	}
	request.Action = "delete"
	if err := ValidateTaskRequest(request); err == nil {
		t.Fatal("ValidateTaskRequest() accepted an unsafe action")
	}
}
