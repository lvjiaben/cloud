package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
)

func TestBuiltinSourcePluginDeclarationAndManifest(t *testing.T) {
	definition, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok || definition == nil {
		t.Fatal("content-notice must register as a source plugin")
	}
	content, err := os.ReadFile(filepath.Join("..", "plugin.yaml"))
	if err != nil {
		t.Fatalf("read source manifest: %v", err)
	}
	for _, fragment := range []string{
		"id: content-notice",
		"version: v0.3.1",
		"type: source",
		"distribution: builtin",
		"default_install_mode: global",
		"path: /system/notice",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("source manifest is missing %q", fragment)
		}
	}
	for _, forbidden := range []string{"admin_surfaces:", "hostServices:", "type: dynamic"} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("builtin source manifest must not retain dynamic declaration %q", forbidden)
		}
	}
}

func TestBuiltinNoticePageUsesVbenSourcePage(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "frontend", "pages", "notice-management.vue"))
	if err != nil {
		t.Fatalf("read source notice page: %v", err)
	}
	text := string(content)
	for _, fragment := range []string{
		`routePath: "/system/notice"`,
		`useVbenVxeGrid`,
		`useVbenModal`,
		`noticeList`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("source notice page is missing %q", fragment)
		}
	}
	for _, forbidden := range []string{"window.parent.postMessage", "pluginSurfaceCapabilities", "<iframe"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("builtin notice page must not retain dynamic bridge markup %q", forbidden)
		}
	}
}
