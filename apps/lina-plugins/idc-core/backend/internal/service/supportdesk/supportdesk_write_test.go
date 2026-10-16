// This file verifies queue-only assignment without requiring role inference or
// legacy employee qualification storage.

package supportdesk

import (
	"context"
	"testing"
)

// TestInitialAssignmentAlwaysQueues verifies new, split and anonymous ticket
// creation cannot call old automatic-assignment storage or choose a person.
func TestInitialAssignmentAlwaysQueues(t *testing.T) {
	service := &serviceImpl{}
	for _, test := range []struct {
		department string
		mode       string
	}{
		{mode: "public_queue"},
		{department: "DEP_support", mode: "manual_claim"},
	} {
		userID, mode, err := service.resolveInitialAssignee(context.Background(), test.department)
		if err != nil || userID != 0 || mode != test.mode {
			t.Fatalf("department=%s user=%d mode=%s err=%v", test.department, userID, mode, err)
		}
	}
}
