// This file verifies lifecycle impact registration survives source plugin startup.
package backend

import (
	"lina-core/pkg/plugin/pluginhost"
	"testing"
)

// TestBusinessImpactHandlerRegistered prevents missing-owner lifecycle failures.
func TestBusinessImpactHandlerRegistered(t *testing.T) {
	definition, ok := pluginhost.GetSourcePlugin("idc-core")
	if !ok || definition == nil || definition.GetBusinessImpactHandler() == nil {
		t.Fatal("trusted business impact owner handler not registered")
	}
}
