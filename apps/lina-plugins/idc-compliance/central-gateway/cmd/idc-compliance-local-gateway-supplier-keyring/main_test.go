// This file verifies safe generation and rotation of the private supplier-
// identity keyring file.

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
)

func TestGenerateAndRotateOneSupplierKeyFamily(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "supplier-identity-1.json")
	if err := run(first, "", ""); err != nil {
		t.Fatalf("generate supplier keyring: %v", err)
	}
	generated := readFixture(t, first)
	if generated.Encryption.ActiveVersion != 1 || generated.Lookup.ActiveVersion != 1 ||
		generated.Idempotency.ActiveVersion != 1 {
		t.Fatalf("unexpected generated versions: %+v", generated)
	}
	second := filepath.Join(root, "supplier-identity-2.json")
	if err := run(second, first, string(supplieridentity.PurposeSubjectLookup)); err != nil {
		t.Fatalf("rotate supplier lookup key: %v", err)
	}
	rotated := readFixture(t, second)
	if rotated.Lookup.ActiveVersion != 2 || len(rotated.Lookup.Keys) != 2 ||
		rotated.Encryption.ActiveVersion != 1 || rotated.Idempotency.ActiveVersion != 1 {
		t.Fatalf("unexpected rotated versions: %+v", rotated)
	}
	if err := run(second, "", ""); err == nil {
		t.Fatal("existing supplier keyring was overwritten")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(second)
		if err != nil {
			t.Fatalf("stat supplier keyring: %v", err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("supplier keyring permissions=%v", info.Mode().Perm())
		}
	}
}

func TestSupplierKeyringRotationRejectsUnknownFamilyAndInPlaceWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "supplier-identity.json")
	if err := run(path, "", ""); err != nil {
		t.Fatalf("generate supplier keyring: %v", err)
	}
	if err := run(filepath.Join(root, "next.json"), path, "unknown"); err == nil {
		t.Fatal("unknown supplier key family was accepted")
	}
	if err := run(path, path, string(supplieridentity.PurposeSubjectLookup)); err == nil {
		t.Fatal("in-place supplier keyring rotation was accepted")
	}
}

func readFixture(t *testing.T, path string) supplieridentity.File {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read supplier keyring: %v", err)
	}
	defer clearBytes(content)
	file, err := supplieridentity.Parse(content)
	if err != nil {
		t.Fatalf("parse supplier keyring: %v", err)
	}
	return file
}
