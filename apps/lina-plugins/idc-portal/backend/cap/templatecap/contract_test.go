// This file verifies framework-neutral entries cannot escape published plugin assets.
package templatecap

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

// TestResolvePublicEntry ensures the active release identity owns every generated URL.
func TestResolvePublicEntry(t *testing.T) {
	provider := &capmodel.BusinessExtensionProviderEvidence{PluginID: "my-template", ReleaseVersion: "v1.2.3", ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"entry": "index.html", "spa": true}}}
	entry, err := Resolve(provider)
	if err != nil || entry.URL != "/x-assets/my-template/v1.2.3/index.html" || entry.AssetBase != "/x-assets/my-template/v1.2.3/" || !entry.SPA {
		t.Fatalf("entry: %#v %v", entry, err)
	}
	for _, bad := range []any{"%2e%2e/secret.html", "../secret.html", "/index.html", "https://evil.test/index.html", "a/../../index.html", "index.html?x=1", "index.js", true} {
		provider.ProviderMetadata.Values["entry"] = bad
		if err := Validate(provider.ProviderMetadata); err == nil {
			t.Fatalf("unsafe entry accepted: %#v", bad)
		}
	}
}
