// This file verifies provider configuration preparation accepts an absent
// schema and strips file references without weakening malformed-input rejection.

package productfulfillment

import (
	"encoding/json"
	"testing"
)

func TestSafeProviderCustomerConfiguration(t *testing.T) {
	for _, scenario := range []struct {
		name, declaration, values, want string
		invalid                         bool
	}{
		{name: "empty declaration", declaration: "{}", values: "{}", want: "{}"},
		{name: "absent declaration", values: "{}", want: "{}"},
		{name: "strip file", declaration: `{"version":"v1","fields":[{"key":"attachment","type":"file"},{"key":"name","type":"text"}]}`, values: `{"attachment":["opaque-file"],"name":"example"}`, want: `{"name":"example"}`},
		{name: "bad declaration", declaration: "{", values: "{}", invalid: true},
		{name: "null values", declaration: "{}", values: "null", invalid: true},
		{name: "bad values", declaration: "{}", values: "[", invalid: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			got, err := safeProviderCustomerConfiguration(scenario.declaration, json.RawMessage(scenario.values))
			if scenario.invalid {
				if err == nil {
					t.Fatal("invalid input accepted")
				}
				return
			}
			if err != nil || string(got) != scenario.want {
				t.Fatalf("got=%s error=%v want=%s", got, err, scenario.want)
			}
		})
	}
}
