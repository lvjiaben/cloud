// This file verifies the connector-fact transport allowlist independently of
// the product owner's stricter declared-field and publication validation.
package resourceinterfaceprojectioncap

import (
	"strings"
	"testing"
)

func TestCatalogConfigurationAcceptsOnlyOneBoundedScalar(t *testing.T) {
	t.Parallel()
	text, decimal, option, integer, boolean := "label", "1.005", "101", int64(8), false
	valid := []CatalogConfigurationValue{
		{FieldKey: "text", TextValue: &text}, {FieldKey: "decimal", DecimalValue: &decimal},
		{FieldKey: "selection", OptionValue: &option}, {FieldKey: "integer", IntegerValue: &integer},
		{FieldKey: "boolean", BooleanValue: &boolean},
	}
	query := CatalogProductQuery{Reference: "101"}
	check := func(values []CatalogConfigurationValue) error {
		return (CatalogDetail{Product: CatalogProduct{Reference: "101", Name: "Server"}, ProductConfiguration: values}).Validate(query)
	}
	if err := check(valid); err != nil {
		t.Fatal(err)
	}
	control, longText, empty, exponent := "bad\x00", strings.Repeat("a", 8193), "", "1e3"
	cases := map[string][]CatalogConfigurationValue{
		"no scalar":       {{FieldKey: "cpu"}},
		"two scalars":     {{FieldKey: "cpu", TextValue: &text, IntegerValue: &integer}},
		"duplicate field": {valid[0], valid[0]},
		"empty key":       {{TextValue: &text}},
		"long key":        {{FieldKey: strings.Repeat("a", 65), TextValue: &text}},
		"control":         {{FieldKey: "text", TextValue: &control}},
		"long text":       {{FieldKey: "text", TextValue: &longText}},
		"empty option":    {{FieldKey: "selection", OptionValue: &empty}},
		"exponent":        {{FieldKey: "decimal", DecimalValue: &exponent}},
		"too many":        make([]CatalogConfigurationValue, 129),
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			if check(values) == nil {
				t.Fatal("invalid connector fact accepted")
			}
		})
	}
}
