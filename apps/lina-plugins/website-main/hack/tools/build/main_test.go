// Verify actual copied artifacts and refuse destructive rebuilds.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrdinaryBuild(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		"frontend/public/index.html": "<html>actual source</html>",
		"gateway/server.mjs":         "// server",
		"gateway/commerce.mjs":       "// authenticated commerce",
		"gateway/site.mjs":           "// handler",
		"gateway/brand-identity.mjs": "// public identity",
		"DEPLOYMENT.md":              "Deployment",
		"DEPLOYMENT.zh-CN.md":        "部署",
		"plugin.yaml":                "must not ship",
		"dist/old.wasm":              "must not ship",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(root, "result")
	if err := build(root, output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(output, "public", "index.html"))
	if err != nil || string(data) != "<html>actual source</html>" {
		t.Fatalf("copied source: %q, %v", data, err)
	}
	for _, name := range []string{"plugin.yaml", "old.wasm", "frontend"} {
		if _, err := os.Stat(filepath.Join(output, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected output %s: %v", name, err)
		}
	}
	if err := build(root, output); err == nil {
		t.Fatal("must refuse existing output")
	}
}
