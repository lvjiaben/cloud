// This file verifies certificate trust update semantics before database writes.
package providerconfiguration

import (
	"strings"
	"testing"
)

func TestResolveCertificateFingerprint(t *testing.T) {
	pin := strings.Repeat("a", 64)
	upper, empty, invalid := strings.ToUpper(pin), "", "bad"
	for _, test := range []struct {
		name    string
		input   *string
		changed bool
		want    string
		invalid bool
	}{
		{name: "omitted preserves", want: pin},
		{name: "endpoint change clears", changed: true},
		{name: "explicit clear", input: &empty},
		{name: "normalized explicit trust", input: &upper, changed: true, want: pin},
		{name: "malformed rejected", input: &invalid, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveCertificateFingerprint(test.input, pin, test.changed)
			if (err != nil) != test.invalid || got != test.want {
				t.Fatalf("value=%q err=%v", got, err)
			}
		})
	}
}
