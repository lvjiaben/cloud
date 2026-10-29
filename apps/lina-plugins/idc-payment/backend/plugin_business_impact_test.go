// This file verifies that payment lifecycle checks have a registered trusted owner.

package backend

import (
	"testing"

	"lina-core/pkg/plugin/pluginhost"
)

// TestBusinessImpactHandlerRegistered prevents a missing-owner rejection after
// the managed payment release has passed business-extension authorization.
func TestBusinessImpactHandlerRegistered(t *testing.T) {
	definition, ok := pluginhost.GetSourcePlugin("idc-payment")
	if !ok || definition == nil || definition.GetBusinessImpactHandler() == nil {
		t.Fatal("trusted payment business impact handler not registered")
	}
}
