package contracttemplatecap

import "testing"

func TestSelectionScopeValidation(t *testing.T) {
	template := Template{
		TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 1,
		Name: "Standard agreement", SigningOrderCode: "customer_first", LanguageCode: "en-US",
	}
	tests := []struct {
		name      string
		selection Selection
		scope     string
		valid     bool
	}{
		{"product accepts all customers", Selection{Level: SelectionLevelProduct}, "all_customers", true},
		{"product rejects personal", Selection{Level: SelectionLevelProduct}, "personal", false},
		{"enterprise accepts enterprise", Selection{Level: SelectionLevelEnterprise, CustomerType: CustomerTypeEnterprise}, "enterprise", true},
		{"enterprise accepts all customers", Selection{Level: SelectionLevelEnterprise, CustomerType: CustomerTypeEnterprise}, "all_customers", true},
		{"order individual accepts personal", Selection{Level: SelectionLevelOrder, CustomerType: CustomerTypeIndividual}, "personal", true},
		{"order individual rejects enterprise", Selection{Level: SelectionLevelOrder, CustomerType: CustomerTypeIndividual}, "enterprise", false},
		{"order enterprise accepts enterprise", Selection{Level: SelectionLevelOrder, CustomerType: CustomerTypeEnterprise}, "enterprise", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := template
			candidate.ScopeCode = test.scope
			err := ValidateTemplate(candidate, test.selection)
			if test.valid && err != nil {
				t.Fatalf("ValidateTemplate() error = %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("ValidateTemplate() accepted an incompatible scope")
			}
		})
	}
}

func TestSelectionValidation(t *testing.T) {
	if err := ValidateSelection(Selection{Level: SelectionLevelProduct}); err != nil {
		t.Fatalf("product selection error = %v", err)
	}
	if err := ValidateSelection(Selection{
		Level: SelectionLevelProduct, CustomerType: CustomerTypeEnterprise,
	}); err == nil {
		t.Fatal("product selection accepted a customer type")
	}
	if err := ValidateSelection(Selection{
		Level: SelectionLevelEnterprise, CustomerType: CustomerTypeIndividual,
	}); err == nil {
		t.Fatal("enterprise selection accepted an individual")
	}
}
