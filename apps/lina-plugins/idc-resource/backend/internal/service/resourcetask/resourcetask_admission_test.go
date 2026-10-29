// This file proves local authority rejection never poisons supplier health or
// enters uncertain-result reconciliation when no upstream write was attempted.
package resourcetask

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
	"time"
)

func TestLocalAdmissionFailureIsFinalWithoutProviderHealthFailure(t *testing.T) {
	store := &executorTaskService{claimedWrites: []ClaimedTask{{TaskNo: "task-write-accepted", CapabilityCode: string(resourcecap.CapabilityReinstall), LocalResourceNo: "resource-1", ChannelNo: "channel-1", PoolNo: "pool-1", SanitizedParameters: `{"os":"101"}`, AttemptNo: 1, MaxAttempts: 8, LeaseToken: "worker:0123456789abcdef0123456789abcdef", LeaseExpiresAt: time.Now().Add(time.Minute)}}}
	provider := &executorProvider{execute: func(context.Context, resourcecap.OperationRequest) (*resourcecap.OperationResult, error) {
		t.Fatal("local rejection issued cloud write")
		return nil, nil
	}}
	health := &executorHealthService{}
	executor := newExecutorFixtureWithHealth(t, store, provider, health)
	executor.admit = func(context.Context, ClaimedTask, resourcecap.OperationRequest, resourcecap.Provider, func(context.Context) error) error {
		return gerror.New("local owner changed")
	}
	report, err := executor.RunWrites(t.Context(), ClaimInput{WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || report.Persisted != 1 || len(store.transitions) != 1 {
		t.Fatalf("admission transition: %+v %v", report, err)
	}
	transition := store.transitions[0]
	if transition.To != StatusFailed || !transition.Evidence.NonAcceptanceProven || transition.FailureCode != "owner_admission_denied" || transition.NextAt != nil {
		t.Fatalf("local rejection became retry/unknown: %+v", transition)
	}
	if len(health.observations) != 0 {
		t.Fatal("local rejection degraded upstream health")
	}
}

func TestExistingOwnerExecutionClaimNeverBecomesNonAcceptance(t *testing.T) {
	store := &executorTaskService{claimedWrites: []ClaimedTask{{TaskNo: "task-write-accepted", CapabilityCode: string(resourcecap.CapabilityReboot), LocalResourceNo: "resource-1", ChannelNo: "channel-1", PoolNo: "pool-1", SanitizedParameters: `{"mode":"graceful"}`, AttemptNo: 1, MaxAttempts: 8, LeaseToken: "worker:0123456789abcdef0123456789abcdef", LeaseExpiresAt: time.Now().Add(time.Minute)}}}
	provider := &executorProvider{execute: func(context.Context, resourcecap.OperationRequest) (*resourcecap.OperationResult, error) {
		t.Fatal("durably claimed task sent again")
		return nil, nil
	}}
	health := &executorHealthService{}
	executor := newExecutorFixtureWithHealth(t, store, provider, health)
	executor.admit = func(context.Context, ClaimedTask, resourcecap.OperationRequest, resourcecap.Provider, func(context.Context) error) error {
		return ErrAdmissionOutcomeUncertain
	}
	report, err := executor.RunWrites(t.Context(), ClaimInput{WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || report.Persisted != 1 || len(store.transitions) != 1 {
		t.Fatal("uncertain admission was not persisted", err)
	}
	transition := store.transitions[0]
	if transition.To != StatusResultUnknown || transition.Evidence.NonAcceptanceProven || transition.FailureCode != "owner_execution_unconfirmed" {
		t.Fatalf("existing claim became false rejection: %+v", transition)
	}
	if len(health.observations) != 0 {
		t.Fatal("local execution recovery degraded provider health")
	}
}
