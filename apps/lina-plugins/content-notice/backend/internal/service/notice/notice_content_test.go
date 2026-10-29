// This file verifies executable announcement content is removed without losing ordinary images and formatting.

package notice

import (
	"strings"
	"testing"
)

// TestSanitizeContentKeepsImagesAndRemovesExecutableHTML covers the HTML boundary shared by all announcement clients.
func TestSanitizeContentKeepsImagesAndRemovesExecutableHTML(t *testing.T) {
	input := `<h2>Maintenance</h2><p style="text-align: center; color: red">Tonight <strong>please wait</strong></p><img src="/api/v1/files/image.png" onerror="alert(1)"><script>alert(2)</script><a href="javascript:alert(3)">bad link</a><img src="data:image/svg+xml,<svg onload='alert(4)'/>">`
	output := sanitizeContent(input)
	for _, want := range []string{`<h2>Maintenance</h2>`, `<strong>please wait</strong>`, `src="/api/v1/files/image.png"`, `text-align: center`} {
		if !strings.Contains(output, want) {
			t.Fatalf("content lost %q: %s", want, output)
		}
	}
	for _, bad := range []string{"onerror", "<script", "javascript:", "data:", "onload", "alert("} {
		if strings.Contains(output, bad) {
			t.Fatalf("unsafe content %q survived: %s", bad, output)
		}
	}
	if twice := sanitizeContent(output); twice != output {
		t.Fatalf("sanitizer is not stable: %s", twice)
	}
}
