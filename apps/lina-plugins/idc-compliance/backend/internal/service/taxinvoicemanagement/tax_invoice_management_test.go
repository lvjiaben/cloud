package taxinvoicemanagement

import (
	"context"
	"testing"
)

func TestClosedNuoNuoGateAndValidation(t *testing.T) {
	t.Parallel()
	capability, err := (ClosedNuoNuoGate{}).Current(context.Background())
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if capability.Ready || capability.ProviderCode != ProviderNuoNuo ||
		capability.ReasonCode != CapabilityExternalConfirmation ||
		len(capability.SupportedInvoiceTypes) != 0 {
		t.Fatalf("Current() = %#v", capability)
	}
	if _, valid := normalizeCapability(capability); !valid {
		t.Fatal("closed production capability was invalid")
	}
	ready, valid := normalizeCapability(Capability{
		ProviderCode: ProviderNuoNuo, Ready: true,
		SupportedInvoiceTypes: []string{InvoiceTypeVATElectronicNormal},
	})
	if !valid || !capabilitySupports(ready, InvoiceTypeVATElectronicNormal) {
		t.Fatalf("ready capability invalid: %#v valid=%v", ready, valid)
	}
}

func TestNormalizeCloseUsesOnlyFrozenReasons(t *testing.T) {
	t.Parallel()
	input := CloseInput{
		TaxInvoiceNo: "ti_test", ReasonCode: ReasonOther, InternalNote: "仅后台可见",
		ExpectedVersion: 1, IdempotencyKey: "idem-key-0001",
	}
	if _, _, err := normalizeClose("cancel", input); err != nil {
		t.Fatalf("normalizeClose() error = %v", err)
	}
	input.ReasonCode = "free_text_reason"
	if _, _, err := normalizeClose("cancel", input); err == nil {
		t.Fatal("normalizeClose() accepted an unfrozen reason")
	}
}
