package contractfulfillmentcap

import "testing"

func TestValidateRequirementsRejectsUnsafeBatches(t *testing.T) {
	valid := []Requirement{{OrderNo: "ORDER_1", RequiresContract: true}}
	if err := ValidateRequirements(valid); err != nil {
		t.Fatalf("valid requirements rejected: %v", err)
	}
	for name, values := range map[string][]Requirement{
		"empty":     nil,
		"duplicate": {{OrderNo: "ORDER_1"}, {OrderNo: "ORDER_1"}},
		"unsafe":    {{OrderNo: "../ORDER_1"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateRequirements(values); err == nil {
				t.Fatal("unsafe requirements were accepted")
			}
		})
	}
}

func TestValidateDecisionsKeepsClosedVocabularyAndFailClosedRules(t *testing.T) {
	requirements := []Requirement{
		{OrderNo: "ORDER_OPTIONAL", RequiresContract: false},
		{OrderNo: "ORDER_SIGNED", RequiresContract: true},
		{OrderNo: "ORDER_PENDING", RequiresContract: true},
	}
	valid := []Decision{
		{OrderNo: "ORDER_OPTIONAL", Ready: true, ReasonCode: ReasonNotRequired},
		{OrderNo: "ORDER_SIGNED", Ready: true, ReasonCode: ReasonCompleted},
		{OrderNo: "ORDER_PENDING", Ready: false, ReasonCode: ReasonNotReady},
	}
	if err := ValidateDecisions(requirements, valid); err != nil {
		t.Fatalf("valid decisions rejected: %v", err)
	}
	invalid := append([]Decision(nil), valid...)
	invalid[2] = Decision{OrderNo: "ORDER_PENDING", Ready: true, ReasonCode: ReasonNotReady}
	if err := ValidateDecisions(requirements, invalid); err == nil {
		t.Fatal("required order without authoritative completion was accepted")
	}
}
