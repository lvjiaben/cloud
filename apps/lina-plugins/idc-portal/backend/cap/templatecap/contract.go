// Package templatecap defines framework-neutral public template assets.
package templatecap

import (
	"net/url"
	"path"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

// Entry describes a static HTML application published by a template plugin.
// URLs always derive from the host-frozen provider identity, never client input.
type Entry struct {
	URL       string `json:"url" dc:"Host-published HTML entry URL" eg:"/x-assets/my-template/v0.1.0/index.html"`
	AssetBase string `json:"assetBase" dc:"Absolute public asset base URL" eg:"/x-assets/my-template/v0.1.0/"`
	SPA       bool   `json:"spa" dc:"Whether deep links use the same HTML entry" eg:"true"`
}

// Validate accepts an optional relative HTML entry; absent means an owner-native renderer.
func Validate(metadata *capmodel.BusinessExtensionProviderMetadata) error {
	if metadata == nil || metadata.Values["entry"] == nil {
		return nil
	}
	entry, ok := metadata.Values["entry"].(string)
	if !ok || !validEntryPath(entry) {
		return gerror.New("template HTML entry is invalid")
	}
	if spa, exists := metadata.Values["spa"]; exists {
		if _, valid := spa.(bool); !valid {
			return gerror.New("template SPA flag is invalid")
		}
	}
	return nil
}

func validEntryPath(entry string) bool {
	parsed, err := url.Parse(entry)
	if err != nil || entry == "" || strings.HasPrefix(entry, "/") || parsed.IsAbs() || parsed.Host != "" {
		return false
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(entry, "%\\\r\n") {
		return false
	}
	return path.Clean(entry) == entry && !strings.HasPrefix(entry, "../") && strings.HasSuffix(strings.ToLower(entry), ".html")
}

// Resolve projects an entry only from a validated release-frozen provider declaration.
func Resolve(provider *capmodel.BusinessExtensionProviderEvidence) (*Entry, error) {
	if provider == nil {
		return nil, gerror.New("template provider is unavailable")
	}
	if err := Validate(provider.ProviderMetadata); err != nil {
		return nil, err
	}
	if provider.ProviderMetadata == nil || provider.ProviderMetadata.Values["entry"] == nil {
		return nil, nil
	}
	entry, ok := provider.ProviderMetadata.Values["entry"].(string)
	if !ok {
		return nil, gerror.New("template HTML entry is invalid")
	}
	if strings.TrimSpace(provider.PluginID) == "" || strings.TrimSpace(provider.ReleaseVersion) == "" {
		return nil, gerror.New("template release identity is missing")
	}
	base := pluginhost.HostedAssetURLPrefix + url.PathEscape(provider.PluginID) + "/" + url.PathEscape(provider.ReleaseVersion) + "/"
	spa, _ := provider.ProviderMetadata.Values["spa"].(bool)
	return &Entry{URL: base + entry, AssetBase: base + strings.TrimPrefix(path.Dir(entry)+"/", "./"), SPA: spa}, nil
}
