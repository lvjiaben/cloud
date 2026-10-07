package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

func TestReadBoundedFileAndLocalKeyring(t *testing.T) {
	root := t.TempDir()
	file, err := processauth.Generate(bytes.NewReader(bytes.Repeat([]byte{7}, 128)))
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	content, err := processauth.Marshal(file)
	if err != nil {
		t.Fatalf("marshal keyring: %v", err)
	}
	path := filepath.Join(root, "process-auth.json")
	if err = os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write keyring: %v", err)
	}
	loaded, err := readBoundedFile(path, processauth.MaximumKeyringSize, true)
	if err != nil {
		t.Fatalf("read keyring: %v", err)
	}
	defer clearRuntimeBytes(loaded)
	parsed, err := processauth.Parse(loaded)
	if err != nil || parsed.ActiveKeyID != file.ActiveKeyID {
		t.Fatalf("parsed=%+v err=%v", parsed, err)
	}
	if _, err = readBoundedFile(root, processauth.MaximumKeyringSize, false); err == nil {
		t.Fatal("directory was accepted as a regular configuration file")
	}
}
