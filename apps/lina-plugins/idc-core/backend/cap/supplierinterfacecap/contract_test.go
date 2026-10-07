package supplierinterfacecap

import "testing"

func TestConfigurationViewRequiresExactlyOneBoundedValueArm(t *testing.T) {
	t.Parallel()
	text := "value"
	integer := int64(1)
	valid := &ConfigurationView{Values: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &text}}}
	if err := ValidateConfigurationView(valid); err != nil {
		t.Fatalf("valid view: %v", err)
	}
	for name, view := range map[string]*ConfigurationView{
		"no arm":    {Values: []ConfigurationValue{{FieldKey: "endpoint"}}},
		"two arms":  {Values: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &text, IntegerValue: &integer}}},
		"duplicate": {Values: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &text}, {FieldKey: "endpoint", TextValue: &text}}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateConfigurationView(view); err == nil {
				t.Fatal("invalid view was accepted")
			}
		})
	}
}
