package managementcap

import "testing"

func TestStaffRenewalInputsRejectCallerControlledBusinessFacts(t *testing.T) {
	quote := StaffRenewalQuoteInput{
		CustomerNo: "CUS_1", EntitlementNo: "ENT_1", RenewalCycles: 1,
		IdempotencyKey: "renewal-preview-1",
	}
	if err := ValidateStaffRenewalQuoteInput(quote); err != nil {
		t.Fatalf("valid staff renewal quote rejected: %v", err)
	}
	quote.RenewalCycles = 121
	if err := ValidateStaffRenewalQuoteInput(quote); err == nil {
		t.Fatal("unbounded renewal cycle count accepted")
	}
	confirm := StaffRenewalConfirmInput{
		CustomerNo: "CUS_1", QuoteNo: "QUO_1", ActorUserID: 7,
		IdempotencyKey: "renewal-confirm-1",
	}
	if err := ValidateStaffRenewalConfirmInput(confirm); err != nil {
		t.Fatalf("valid staff renewal confirmation rejected: %v", err)
	}
	confirm.ActorUserID = 0
	if err := ValidateStaffRenewalConfirmInput(confirm); err == nil {
		t.Fatal("staff renewal confirmation without an authorized actor accepted")
	}
}
