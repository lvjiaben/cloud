// These tests guard the anonymous cart's minimal display-only response boundary.
package public

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"strings"
	"testing"
)

func TestGuestCartViewExcludesPrivateConfiguration(t *testing.T) {
	cart := &commerce.Cart{CartNo: "CRT_test", CurrencyCode: "CNY", Version: 4, Items: []commerce.CartItem{{CartItemNo: "CIT_test", ConfigurationReference: "CFS_public_reference", PublicSlug: "cloud", ProductNameZhCN: "云服务器", ProductNameEnUS: "Cloud server", Quantity: 2, PurchaseCycles: 1, Version: 3, ConfigurationRevision: "private-revision-sentinel", IssueCode: "private-issue-sentinel"}}}
	data, err := json.Marshal(guestCartView(cart))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sentinel") || strings.Contains(string(data), "configurationRevision") || strings.Contains(string(data), "token") {
		t.Fatal("anonymous projection contains private facts")
	}
	if !strings.Contains(string(data), `"configurationReference":"CFS_public_reference"`) || !strings.Contains(string(data), `"quantity":2`) || !strings.Contains(string(data), `"version":4`) {
		t.Fatal("cart display or optimistic version missing")
	}
	if guestCartView(nil) != nil {
		t.Fatal("absent cart must remain absent")
	}
}
