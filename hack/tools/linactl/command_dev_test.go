package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A clean checkout has no generated frontend assets. Development startup must
// run the dynamic plugin's build commands before WASM packing consumes them.
func TestBuildDevDynamicPluginsBuildsDeclaredAssetsBeforePacking(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.work"), "go 1.25.0\n")
	pluginDir := filepath.Join(root, "apps", "lina-plugins", "ssl-test")
	writeDynamicPluginManifest(t, pluginDir, "ssl-test")
	f, err := os.OpenFile(filepath.Join(pluginDir, "plugin.yaml"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("public_assets:\n  - source: frontend/dist\n    mount: /\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(pluginDir, "hack", "config.yaml"), "build:\n  commands:\n    - node build-pages\n")
	writeFile(t, filepath.Join(root, "apps", "lina-plugins", "source-test", "plugin.yaml"), "id: source-test\ntype: source\n")
	writeFile(t, filepath.Join(root, "apps", "lina-plugins", "source-test", "hack", "config.yaml"), "build:\n  commands:\n    - node unrelated\n")
	application := newApp(ioDiscard{}, ioDiscard{}, strings.NewReader(""))
	application.root = root
	application.lookPath = func(name string) (string, error) { return name, nil }
	calls := 0
	application.execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
		calls++
		if name != "node" || len(args) != 1 || args[0] != "build-pages" {
			t.Fatalf("unexpected build: %s %v", name, args)
		}
		writeFile(t, filepath.Join(pluginDir, "frontend", "dist", "index.html"), "<html>Certificate page</html>")
		return exec.Command(os.Args[0], "-test.run=TestHelperCommandSuccess", "--")
	}
	if err = buildDevDynamicPlugins(context.Background(), application, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one dynamic build, got %d", calls)
	}
	artifact, err := os.ReadFile(filepath.Join(root, "temp", "output", "ssl-test.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(artifact), "frontend/dist/index.html") {
		t.Fatal("generated page missing from WASM artifact")
	}
}
