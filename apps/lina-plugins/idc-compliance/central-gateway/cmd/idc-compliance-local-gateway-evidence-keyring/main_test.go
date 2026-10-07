// This file verifies safe cross-platform callback-evidence keyring creation.

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

func TestRunCreatesPrivateKeyringWithoutOverwrite(t *testing.T) {
	output := filepath.Join(t.TempDir(), "callback-evidence.json")
	if err := run(output); err != nil {
		t.Fatalf("generate callback evidence keyring: %v", err)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read callback evidence keyring: %v", err)
	}
	defer clear(content)
	if _, err = callbacksecurity.Parse(content); err != nil {
		t.Fatalf("parse callback evidence keyring: %v", err)
	}
	if err = run(output); err == nil {
		t.Fatal("existing callback evidence keyring was overwritten")
	}
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(output)
		if statErr != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("callback evidence keyring permissions=%v err=%v", info.Mode().Perm(), statErr)
		}
	}
}
