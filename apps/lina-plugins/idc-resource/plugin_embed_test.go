// These tests verify that official source binaries contain the purchased-page entry and scripts.
package idcresource

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedPurchasedPageIncludesBuiltAssets(t *testing.T) {
	page, err := EmbeddedFiles.ReadFile("frontend/dist/customer/index.html")
	if err != nil || !strings.Contains(string(page), "type=\"module\"") {
		t.Fatalf("purchased customer page must be embedded: %v", err)
	}
	assets, err := fs.Glob(EmbeddedFiles, "frontend/dist/customer/assets/*.js")
	if err != nil || len(assets) == 0 {
		t.Fatalf("purchased customer entry assets must be embedded: %v", err)
	}
}
