// This file tests bounded product-automation list input validation.

package productautomation

import (
	"strings"
	"testing"
)

func TestNormalizePageRejectsUnsafeProviderSelectionToken(t *testing.T) {
	for _, token := range []string{" token", "token ", strings.Repeat("a", 4097)} {
		if _, err := normalizePage(PageInput{ProviderSelectionToken: token}); err == nil {
			t.Fatalf("expected provider selection token %q to be rejected", token)
		}
	}
}

func TestNormalizePageAcceptsBoundedProviderSelectionToken(t *testing.T) {
	page, err := normalizePage(PageInput{ProviderSelectionToken: "opaque-provider-token"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.PageSize != defaultPageSize || page.ProviderSelectionToken != "opaque-provider-token" {
		t.Fatalf("unexpected normalized page: %+v", page)
	}
}
