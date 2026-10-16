// This file verifies the versioned resource lifecycle contract validation boundary.

package lifecycleresourcecap

import (
	"strings"
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

func TestEntitlementLifecycleRouteCannotBeSuppliedByCaller(t *testing.T) {
	request := TaskRequest{
		LifecycleTaskNo: "LCT_0123456789abcdef0123456789abcdef",
		ServiceNo:       "ent_" + strings.Repeat("1", 32), CustomerNo: "customer-1",
		Action: ActionSuspend, ExpectedServiceVersion: 2,
		ExpectedExpiresAt: time.Now().UTC(), IdempotencyDigest: strings.Repeat("a", 64),
	}
	for _, action := range []Action{ActionSuspend, ActionResume} {
		valid := request
		valid.Action = action
		if err := ValidateTaskRequest(valid); err != nil {
			t.Fatalf("valid entitlement %s: %v", action, err)
		}
	}
	for _, change := range []func(*TaskRequest){
		func(value *TaskRequest) { value.CustomerNo = "" },
		func(value *TaskRequest) { value.CustomerNo = " customer-1" },
		func(value *TaskRequest) { value.ServiceNo = "legacy-service" },
		func(value *TaskRequest) { value.ChannelNo = "forged-channel" },
		func(value *TaskRequest) { value.PoolNo = "forged-pool" },
		func(value *TaskRequest) { value.Action = ActionTerminate },
	} {
		invalid := request
		change(&invalid)
		if err := ValidateTaskRequest(invalid); err == nil {
			t.Fatalf("accepted unfenced entitlement lifecycle request: %+v", invalid)
		}
	}
}
