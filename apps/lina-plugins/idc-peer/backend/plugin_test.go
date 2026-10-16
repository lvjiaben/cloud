// These tests ensure missing runtime dependencies fail during initialization.

package backend

import "testing"

func TestGuestHandlerRejectsMissingExecutor(t *testing.T) {
	handler, err := NewGuestHandler(nil)
	if err == nil || handler != nil {
		t.Fatal("missing confidential executor must reject initialization")
	}
}
