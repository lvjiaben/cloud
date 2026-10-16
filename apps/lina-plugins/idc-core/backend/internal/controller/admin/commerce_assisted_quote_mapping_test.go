// These tests keep opaque typed selections intact at the assisted HTTP boundary.
package admin

import (
	"encoding/json"
	"testing"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func TestAssistedQuoteTypedValuesSurviveDTOAndMapping(t *testing.T) {
	var request v1.CommerceAssistedQuoteCreateReq
	if err := json.Unmarshal([]byte(`{"items":[{"quantity":1,"publicSlug":"certificate","configurationRevision":"revision","billingTermToken":"term_one","purchaseCycles":1,"values":[{"fieldToken":"field_domain","type":"single_line","textValue":"example.com"},{"fieldToken":"field_count","type":"integer","integerValue":"9007199254740993"}]}]}`), &request); err != nil {
		t.Fatal(err)
	}
	rows, err := toCommerceAssistedQuoteItems(request.Items, "CNY")
	if err != nil || len(rows) != 1 || len(rows[0].Purchase.Values) != 2 {
		t.Fatalf("rows=%+v error=%v", rows, err)
	}
	if *rows[0].Purchase.Values[0].TextValue != "example.com" || *rows[0].Purchase.Values[1].IntegerValue != "9007199254740993" {
		t.Fatal("typed values changed")
	}
	request.Items[0].Values = append(request.Items[0].Values, nil)
	if _, err = toCommerceAssistedQuoteItems(request.Items, "CNY"); err == nil {
		t.Fatal("nil configuration value accepted")
	}
}
