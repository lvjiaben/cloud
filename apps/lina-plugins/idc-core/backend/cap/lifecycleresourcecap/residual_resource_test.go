package lifecycleresourcecap

import (
	"strings"
	"testing"
	"time"
)

func TestLifecycleResidualResourceProjectionFailsClosed(t *testing.T) {
	task := Task{ProviderTaskNo: "provider-task-1", Status: TaskStatusResultUnknown,
		FailureCode: "partial_reclaim", CapabilityEvidenceNo: "capability-1",
		CapabilityEvidenceDigest: strings.Repeat("a", 64), EvidenceDigest: strings.Repeat("b", 64),
		ObservedAt: time.Now().UTC(), ResidualResources: []ResidualResource{{ResourceKind: "volume",
			ResourceRef: "volume_600100", Status: "release_failed", ReasonCode: "provider_rejected"}}}
	if err := ValidateTask(task); err != nil {
		t.Fatalf("valid lifecycle residual projection rejected: %v", err)
	}
	succeeded := task
	succeeded.Status = TaskStatusSucceeded
	succeeded.FailureCode = ""
	if err := ValidateTask(succeeded); err == nil {
		t.Fatal("successful lifecycle task accepted residual resources")
	}
	duplicated := task
	duplicated.ResidualResources = append(duplicated.ResidualResources, duplicated.ResidualResources[0])
	if err := ValidateTask(duplicated); err == nil {
		t.Fatal("duplicated lifecycle residual resource was accepted")
	}
}
