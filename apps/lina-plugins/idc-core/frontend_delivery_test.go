package idccore

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"
)

func TestFrontendDeliveryOwnsBuildAndHostedMount(t *testing.T) {
	manifest, err := fs.ReadFile(EmbeddedFiles, "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded plugin manifest: %v", err)
	}
	if !bytes.Contains(manifest, []byte("source: frontend/dist")) {
		t.Fatal("plugin manifest must publish the plugin-owned frontend/dist directory")
	}

	mountBundle, err := fs.ReadFile(EmbeddedFiles, "frontend/dist/mount.js")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skip("run the plugin-owned frontend build before checking the embedded bundle")
		}
		t.Fatalf("read embedded frontend mount bundle: %v", err)
	}
	if !bytes.Contains(mountBundle, []byte("mount")) {
		t.Fatal("frontend bundle must export the LinaPro mount contract")
	}
	if !bytes.Contains(mountBundle, []byte("credit-management-panel")) {
		t.Fatal("frontend bundle must include the plugin-owned credit management workspace")
	}
	for _, forbidden := range [][]byte{[]byte("web-antd"), []byte("@vben/")} {
		if bytes.Contains(mountBundle, forbidden) {
			t.Fatalf("frontend bundle crosses the host dependency boundary: %s", forbidden)
		}
	}
	if bytes.Contains(mountBundle, []byte("process.env")) {
		t.Fatal("frontend bundle must not require a Node.js process global in the browser")
	}
}
