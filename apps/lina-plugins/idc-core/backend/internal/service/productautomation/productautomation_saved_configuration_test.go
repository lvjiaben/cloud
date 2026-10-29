package productautomation

import "testing"

func TestSameSavedMerchantValuesPreservesTypedFacts(t *testing.T) {
	integer, text, changed := int64(22), "22", "23"
	saved := []ConfigurationValue{{FieldKey: "disk", IntegerValue: &integer}, {FieldKey: "region", OptionTokens: []string{"east"}}}
	cases := []struct {
		name  string
		input []ConfigurationValue
		want  bool
	}{
		{"wire integer and reordered fields", []ConfigurationValue{saved[1], {FieldKey: "disk", IntegerTextValue: &text}}, true},
		{"changed integer", []ConfigurationValue{saved[1], {FieldKey: "disk", IntegerTextValue: &changed}}, false},
		{"duplicate field", []ConfigurationValue{saved[1], saved[1]}, false},
		{"extra typed representation", []ConfigurationValue{saved[1], {FieldKey: "disk", IntegerTextValue: &text, IntegerValue: &integer}}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := sameSavedMerchantValues(saved, test.input); got != test.want {
				t.Fatalf("same=%v want=%v", got, test.want)
			}
		})
	}
}
