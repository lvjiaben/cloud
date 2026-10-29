// These tests distinguish terminal local preparation from an attempted call,
// retaining retry budgets and binding the final no-call evidence to its task.
package resourcetask

import (
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestLocalProvisionFailureRetainsTaskBoundNonSubmissionProof(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		store := &executorTaskService{}
		executor := newExecutorFixture(t, store, &executorProvider{})
		task := ClaimedTask{TaskNo: "task_local_preparation", CapabilityCode: string(resourcecap.CapabilityProvision), AttemptNo: 1, MaxAttempts: 3, LeaseExpiresAt: time.Now().Add(time.Minute)}
		if exhausted {
			task.AttemptNo = task.MaxAttempts
		}
		if err := executor.persistBeforeCallFailure(t.Context(), task, LeaseRef{TaskNo: task.TaskNo}, providerUnavailableCode, providerUnavailableSummary); err != nil {
			t.Fatal(err)
		}
		if len(store.transitions) != 1 {
			t.Fatal("local failure did not persist exactly once")
		}
		transition := store.transitions[0]
		if exhausted {
			if transition.To != StatusFailed || transition.FailureCode != provisionresourcecap.FailureRequestNotSent || len(transition.ProviderFacts) != 2 || resourcecap.ValidateProvisionNonSubmissionEvidence(transition.ProviderFacts[1], task.TaskNo) != nil {
				t.Fatalf("final local failure proof missing: %+v", transition)
			}
		} else if transition.To != StatusRetryWait || transition.FailureCode != providerUnavailableCode || len(transition.ProviderFacts) != 0 {
			t.Fatal("local retry budget changed or premature proof emitted")
		}
	}
}
