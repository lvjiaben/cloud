// This file proves relationship maps preserve numeric source IDs and ranges
// without opening a wildcard JSON projection for provider private data.
package composition

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfidentialRelationshipProjectionKeepsOnlyIDsAndNumericBounds(t *testing.T) {
	raw := []byte(`{"data":{"advanced":[{"config_id":10,"sub_id":{"20":{"qty_minimum":0,"qty_maximum":"100","password":"private-secret","note":"private-note"}},"token":"private-token"}]}}`)
	projected, err := projectConfidentialHTTPResponse(raw, []string{"data", "advanced", "config_id", "sub_id", "qty_minimum", "qty_maximum"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projected), `"20":{"qty_maximum":"100","qty_minimum":0}`) || strings.Contains(string(projected), "private") {
		t.Fatalf("relationship projection dropped constraints or leaked fields: %s", projected)
	}
	var data map[string]any
	if json.Unmarshal(projected, &data) != nil {
		t.Fatal("projection is not JSON")
	}
	for _, unsafe := range []string{`{"secret":{"qty_minimum":0}}`, `{"20":{"qty_minimum":"private"}}`, `{"20":{"qty_minimum":-1}}`, `{"20":{"qty_minimum":1.5}}`} {
		source := []byte(`{"data":{"sub_id":` + unsafe + `}}`)
		projected, err = projectConfidentialHTTPResponse(source, []string{"data", "sub_id", "qty_minimum", "qty_maximum"})
		if err == nil && strings.Contains(string(projected), "sub_id") {
			t.Fatalf("unsafe relationship survived: %s", projected)
		}
	}
}

func TestConfidentialCommonProjectionRetainsTypedCyclesAndArrayOptions(t *testing.T) {
	raw := []byte(`{"status":200,"data":{"configoptions":[{"id":10,"option_type":"quantity","qty_min":0,"qty_max":100,"subs":[{"id":21,"qty_min":0,"qty_max":100,"password":"private"}]}],"cycles":{"onetime":"0.00","free":0,"secret_cycle":"private"},"sub_id":42,"custom_cycles":[{"id":3,"cycle_amount":"47.50","cycle_amount_client_level_discount":"2.50"}]}}`)
	projected, err := projectConfidentialHTTPResponse(raw, []string{"status", "data", "configoptions", "id", "option_type", "qty_min", "qty_max", "subs", "cycles", "onetime", "free", "sub_id", "custom_cycles", "cycle_amount", "cycle_amount_client_level_discount"})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`"onetime":"0.00"`, `"free":0`, `"sub_id":42`, `"qty_min":0`, `"cycle_amount_client_level_discount":"2.50"`} {
		if !strings.Contains(string(projected), required) {
			t.Fatalf("required native field disappeared %s", projected)
		}
	}
	if strings.Contains(string(projected), "private") || strings.Contains(string(projected), "password") || strings.Contains(string(projected), "secret_cycle") {
		t.Fatalf("native projection widened private field access %s", projected)
	}
}
