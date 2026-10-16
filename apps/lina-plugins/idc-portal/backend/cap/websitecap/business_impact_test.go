// This file verifies website lifecycle impact does not block safe template disable.
package websitecap

import (
	"context"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
)

// TestWebsiteBusinessImpact verifies template providers have no transactional blockers.
func TestWebsiteBusinessImpact(t *testing.T) {
	for _, id := range []string{"website-default", "website-minimal", "third-party-template"} {
		items, err := BusinessImpact(context.Background(), pluginhost.PluginBusinessImpactInput{TargetPluginID: id})
		if err != nil || items == nil || len(items) != 0 {
			t.Fatalf("unexpected template blockers: %#v %v", items, err)
		}
	}
	if _, err := BusinessImpact(context.Background(), pluginhost.PluginBusinessImpactInput{}); err == nil {
		t.Fatal("empty target accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BusinessImpact(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "website-default"}); err == nil {
		t.Fatal("canceled inspection accepted")
	}
}
