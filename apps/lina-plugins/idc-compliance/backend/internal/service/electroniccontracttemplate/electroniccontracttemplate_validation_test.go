package electroniccontracttemplate

import "testing"

func TestTemplateNumberRejectsLegacyDemoTemplate(t *testing.T) {
	t.Parallel()
	if templateNoPattern.MatchString("demo_contract_template_local") {
		t.Fatal("legacy demo template number must be rejected")
	}
	if templateNoPattern.MatchString("demo_contract_template_other") {
		t.Fatal("arbitrary demo template numbers must not bypass validation")
	}
}
