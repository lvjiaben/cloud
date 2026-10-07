package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

func TestRunGeneratesAndRotatesWithoutOverwrite(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.json")
	installationID := filepath.Join(directory, "installation-id")
	if err := run(first, installationID, "", ""); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := run(first, filepath.Join(directory, "other-id"), "", ""); err == nil {
		t.Fatal("existing keyring was overwritten")
	}
	second := filepath.Join(directory, "second.json")
	if err := run(second, "", first, ""); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	firstFile := parseTestKeyring(t, first)
	secondFile := parseTestKeyring(t, second)
	if firstFile.ActiveKeyID == secondFile.ActiveKeyID || len(secondFile.Keys) != 2 {
		t.Fatalf("first=%+v second=%+v", firstFile, secondFile)
	}
	content, err := os.ReadFile(installationID)
	if err != nil {
		t.Fatalf("read installation id: %v", err)
	}
	if _, err = processauth.ParseInstallationID(content); err != nil {
		t.Fatalf("parse installation id: %v", err)
	}
}

func TestRunRejectsRelativeAndRevocationWithoutRotation(t *testing.T) {
	if err := run("relative.json", "relative-id", "", ""); err == nil {
		t.Fatal("relative output was accepted")
	}
	second := filepath.Join(t.TempDir(), "second.json")
	if err := run(second, "", "", "lgk_1111111111111111"); err == nil {
		t.Fatal("revocation without rotation was accepted")
	}
}

func TestRunRequiresFreshInstallationIdentifierAndPreservesItOnRotation(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.json")
	if err := run(first, "", "", ""); err == nil {
		t.Fatal("fresh keyring without installation identifier was accepted")
	}
	installationID := filepath.Join(directory, "installation-id")
	if err := run(first, installationID, "", ""); err != nil {
		t.Fatalf("generate: %v", err)
	}
	second := filepath.Join(directory, "second.json")
	if err := run(second, filepath.Join(directory, "replacement-id"), first, ""); err == nil {
		t.Fatal("rotation generated a replacement installation identifier")
	}
}

func parseTestKeyring(t *testing.T, path string) processauth.File {
	t.Helper()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatalf("chmod keyring: %v", err)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read keyring: %v", err)
	}
	file, err := processauth.Parse(content)
	if err != nil {
		t.Fatalf("parse keyring: %v", err)
	}
	return file
}
