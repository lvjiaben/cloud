// This file verifies downstream-channel plugin registration and its embedded manifest identity.

package backend

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/util/gmeta"

	"lina-core/pkg/plugin/pluginhost"
	adminv1 "lina-plugin-idc-channel/backend/api/admin/v1"
	channelv1 "lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// TestSourcePluginRegistrationAndManifest verifies the phase-one delivery contract.
func TestSourcePluginRegistrationAndManifest(t *testing.T) {
	if idccap.ContractVersion == "" {
		t.Fatal("compiled idccap contract version is empty")
	}
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	for _, fragment := range []string{
		"id: idc-channel",
		"version: v0.1.2",
		"distribution: builtin",
		"id: idc-core",
		"version: \">=0.1.0 <1.0.0\"",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("embedded manifest is missing %q", fragment)
		}
	}
}

// TestEmbeddedCapabilityLifecycleContract verifies the embedded lifecycle declaration.
func TestEmbeddedCapabilityLifecycleContract(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "manifest/config/capabilities.yaml")
	if err != nil {
		t.Fatalf("read embedded capability lifecycle: %v", err)
	}
	for _, fragment := range []string{
		"stop_new",
		"drain_active",
		"preserve_history",
		"resume_after_validation",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("embedded capability lifecycle is missing %q", fragment)
		}
	}
}

func TestEmbeddedChannelFrontendContract(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	manifest, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	for _, fragment := range []string{"public_assets:", "source: frontend/pages", "index: index.html", "component: system/plugin/dynamic-page"} {
		if !strings.Contains(string(manifest), fragment) {
			t.Fatalf("embedded manifest is missing %q", fragment)
		}
	}
	mount, err := fs.ReadFile(registered.GetEmbeddedFiles(), "frontend/pages/mount.js")
	if err != nil {
		t.Fatalf("channel management frontend is not embedded: %v", err)
	}
	for _, fragment := range []string{
		"/x/idc-channel/api/v1/admin/channel/capabilities",
		"/x/idc-channel/api/v1/admin/channel/catalog",
		"/x/idc-channel/api/v1/admin/channel/tokens",
		"channel-product-publication",
		"customer-token",
		"令牌只在创建时显示一次",
	} {
		if !strings.Contains(string(mount), fragment) {
			t.Fatalf("channel frontend is missing contract fragment %q", fragment)
		}
	}
	entry, err := fs.ReadFile(registered.GetEmbeddedFiles(), "frontend/pages/entry.js")
	if err != nil {
		t.Fatalf("channel sandbox entry is not embedded: %v", err)
	}
	for _, fragment := range []string{"linapro.plugin-page-bridge", "channel.admin.v1", "pluginId"} {
		if !strings.Contains(string(entry), fragment) {
			t.Fatalf("channel sandbox entry is missing bridge contract fragment %q", fragment)
		}
	}
}

// TestChannelRouteAuthorizationMetadata keeps native staff pages on exact host
// permissions while downstream endpoints remain exclusively under the channel
// bearer-token middleware.
func TestChannelRouteAuthorizationMetadata(t *testing.T) {
	adminRequests := []any{&adminv1.CapabilitiesReq{}, &adminv1.CatalogReq{}, &adminv1.ListTokensReq{}, &adminv1.CreateTokenReq{}, &adminv1.RevokeTokenReq{}}
	for _, request := range adminRequests {
		if permission := strings.TrimSpace(gmeta.Get(request, "permission").String()); permission != "idc:admin:channel:token" {
			t.Fatalf("page bridge request %T permission=%q, want idc:admin:channel:token", request, permission)
		}
		if capability := strings.TrimSpace(gmeta.Get(request, pluginhost.RouteMetaPageBridgeCapability).String()); capability != "" {
			t.Fatalf("native admin request %T leaked page-bridge capability %q", request, capability)
		}
		if host := strings.TrimSpace(gmeta.Get(request, pluginhost.RouteMetaPageBridgeHost).String()); host != "" {
			t.Fatalf("native admin request %T leaked page-bridge host %q", request, host)
		}
	}
	channelRequests := []any{
		&channelv1.CatalogReq{}, &channelv1.QuoteReq{}, &channelv1.CreateOrderReq{},
		&channelv1.GetOrderReq{}, &channelv1.LifecycleReq{}, &channelv1.CancelOrderReq{},
		&channelv1.CapabilitiesReq{}, &channelv1.GetWebhookReq{},
	}
	for _, request := range channelRequests {
		if permission := strings.TrimSpace(gmeta.Get(request, "permission").String()); permission != "" {
			t.Fatalf("downstream request %T leaked host permission %q", request, permission)
		}
		if capability := strings.TrimSpace(gmeta.Get(request, pluginhost.RouteMetaPageBridgeCapability).String()); capability != "" {
			t.Fatalf("downstream request %T leaked page-bridge capability %q", request, capability)
		}
	}
}
