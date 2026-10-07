package billingcap

import (
	"context"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capregistry"
)

func TestBillingCapabilityContractIsExactAndExecuteRisk(t *testing.T) {
	t.Parallel()
	dependency, err := NewDependency()
	if err != nil || dependency.OwnerPluginID != OwnerPluginID ||
		dependency.Service != ServiceName || len(dependency.Methods) != 13 {
		t.Fatalf("unexpected billing dependency=%+v err=%v", dependency, err)
	}
	dependency.OwnerPluginID = "other-owner"
	if err = ValidateDependency(dependency); err == nil {
		t.Fatal("foreign billing owner accepted")
	}
	descriptor, err := NewDescriptor(billingCapabilityTestInvoker{})
	if err != nil || len(descriptor.Methods) != 13 ||
		descriptor.Methods[1].Risk != capregistry.RiskLevelExecute ||
		descriptor.Methods[1].Capability != CapabilityChannelPaymentConfirm {
		t.Fatalf("unexpected billing descriptor=%+v err=%v", descriptor, err)
	}
}

func TestRefundExecutionContractRequiresPreflightBeforeMoney(t *testing.T) {
	t.Parallel()
	report := ReportRefundExecution{
		WorkerNo: "payment-refund-worker", CommandNo: "rfc_0123456789abcdef",
		RequestDigest: strings.Repeat("a", 64), Phase: RefundExecutionPhasePreflight,
		Status: RefundExecutionResultReady, ResultCode: "preflight_ready",
		EvidenceDigest: strings.Repeat("b", 64),
	}
	if err := ValidateReportRefundExecution(report); err != nil {
		t.Fatalf("valid refund preflight rejected: %v", err)
	}
	report.Status = RefundExecutionResultManualConfirmation
	report.ResultCode = "payment_context_mismatch"
	if err := ValidateReportRefundExecution(report); err != nil {
		t.Fatalf("valid refund preflight mismatch rejected: %v", err)
	}
	report.Status = RefundExecutionResultSucceeded
	report.ResultCode = "refund_succeeded"
	if err := ValidateReportRefundExecution(report); err == nil {
		t.Fatal("refund money result accepted during preflight")
	}
}

func TestChannelPaymentContractRejectsCallerControlledFinancialDrift(t *testing.T) {
	t.Parallel()
	input := ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_0123456789abcdef", InvoiceNo: "INV_0123456789abcdef",
		CurrencyCode: "CNY", AmountMinor: 29900, TotalAmountMinor: 29900, EvidenceDigest: strings.Repeat("a", 64),
	}
	if err := ValidateConfirmedChannelPayment(input); err != nil {
		t.Fatalf("valid channel payment rejected: %v", err)
	}
	changed := input
	changed.AmountMinor = 0
	if err := ValidateConfirmedChannelPayment(changed); err == nil {
		t.Fatal("zero channel payment accepted")
	}
	result := ChannelPaymentResult{
		PaymentAttemptNo: input.PaymentAttemptNo,
		TransactionNo:    "btx_0123456789abcdef0123456789abcdef",
		AllocationNo:     "bpa_0123456789abcdef0123456789abcdef",
		InvoiceNo:        input.InvoiceNo, OrderNo: "ORD_0123456789abcdef",
		CurrencyCode: "CNY", AmountMinor: 29900, TotalAmountMinor: 29900, PaidAmountMinor: 29900,
		InvoiceStatus: "paid", SettlementNo: "fst_0123456789abcdef0123456789abcdef",
	}
	if err := ValidateChannelPaymentResult(result); err != nil {
		t.Fatalf("valid channel payment result rejected: %v", err)
	}
	result.InvoiceStatus = "credit_settled"
	if err := ValidateChannelPaymentResult(result); err == nil {
		t.Fatal("credit settlement was accepted as channel cash")
	}
}

type billingCapabilityTestInvoker struct{}

func (billingCapabilityTestInvoker) Invoke(context.Context, capregistry.Invocation) (*capregistry.InvocationResult, error) {
	return &capregistry.InvocationResult{}, nil
}
