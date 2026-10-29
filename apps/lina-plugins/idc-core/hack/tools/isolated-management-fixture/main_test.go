// These tests verify the database-mutating fixture cannot target the main app
// or escape the confined temporary directory through a parent or symlink.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureDestinationFences(t *testing.T) {
	valid := fixture{BaseURL: "http://127.0.0.1:41895", MockOrigin: "https://127.0.0.1:37227"}
	if err := validateOrigins(valid); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"http://127.0.0.1:6030", "http://127.0.0.1:6031", "http://127.0.0.1:6032", "http://127.0.0.1:6680", "http://example.test:41895", "https://127.0.0.1:41895"} {
		target := valid
		target.BaseURL = base
		if validateOrigins(target) == nil {
			t.Fatal("unsafe fixture destination accepted")
		}
	}
	target := valid
	target.MockOrigin = "https://example.test:37227"
	if validateOrigins(target) == nil {
		t.Fatal("real upstream accepted as mock")
	}
}
func TestFixturePathConfinesResolvedFiles(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	file := filepath.Join(root, "fixture.json")
	if err := os.WriteFile(file, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := confined(file, root); err != nil {
		t.Fatal(err)
	}
	if _, err := confined(outside, root); err == nil {
		t.Fatal("outside path accepted")
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink unavailable")
	}
	if _, err := confined(link, root); err == nil {
		t.Fatal("symlink escaped the fixture root")
	}
}
