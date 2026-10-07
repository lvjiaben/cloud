// This file verifies lifecycle task status projection at the core-to-resource boundary.

package lifecyclesource

import (
	"testing"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

func TestProjectStatusCanonicalizesProviderFailureCode(t *testing.T) {
	t.Parallel()
	for _, status := range []resourcetask.Status{
		resourcetask.StatusFailed,
		resourcetask.StatusResultUnknown,
		resourcetask.StatusManualTakeover,
	} {
		projected, code := projectStatus(&taskRow{
			StatusCode: string(status), FailureCode: "UPSTREAM_REJECTED",
		})
		if code != "upstream_rejected" {
			t.Fatalf("status %q projected failure code %q", status, code)
		}
		if projected != lifecycleresourcecap.TaskStatusFailed &&
			projected != lifecycleresourcecap.TaskStatusResultUnknown &&
			projected != lifecycleresourcecap.TaskStatusManualReview {
			t.Fatalf("status %q produced invalid lifecycle status %q", status, projected)
		}
	}
}
