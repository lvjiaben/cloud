package billing

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/billingcap"
)

type channelPaymentCapabilityServiceStub struct{ calls int }

func (s *channelPaymentCapabilityServiceStub) ClaimRefundExecution(_ context.Context, input billingcap.ClaimRefundExecution) (*billingcap.RefundExecutionWork, error) {
	s.calls++
	return &billingcap.RefundExecutionWork{}, nil
}
func (s *channelPaymentCapabilityServiceStub) ReportRefundExecution(_ context.Context, input billingcap.ReportRefundExecution) (*billingcap.RefundExecutionReport, error) {
	s.calls++
	return &billingcap.RefundExecutionReport{CommandNo: input.CommandNo, RefundNo: "rfd_capability_0001", CommandStatusCode: "succeeded", RefundStatusCode: "executing"}, nil
}

func (s *channelPaymentCapabilityServiceStub) CurrentPaymentActor(context.Context) (*billingcap.PaymentActor, error) {
	s.calls++
	return &billingcap.PaymentActor{PrincipalRefDigest: strings.Repeat("d", 64)}, nil
}
func (s *channelPaymentCapabilityServiceStub) PrepareCreditRepayment(_ context.Context, input billingcap.PrepareCreditRepayment) (*billingcap.PreparedCreditRepayment, error) {
	s.calls++
	return &billingcap.PreparedCreditRepayment{PaymentAttemptNo: input.PaymentAttemptNo, StatementNo: input.StatementNo, CustomerRefDigest: strings.Repeat("e", 64), CurrencyCode: "CNY", AmountMinor: input.AmountMinor, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (s *channelPaymentCapabilityServiceStub) ConfirmCreditRepayment(_ context.Context, input billingcap.ConfirmedCreditRepayment) (*billingcap.CreditRepaymentResult, error) {
	s.calls++
	return &billingcap.CreditRepaymentResult{PaymentAttemptNo: input.PaymentAttemptNo, StatementNo: input.StatementNo, CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor, TransactionNo: "btx_2123456789abcdef0123456789abcdef", RepaymentNo: "CRY_capability_0001", StatementStatusCode: "paid"}, nil
}
func (s *channelPaymentCapabilityServiceStub) PrepareBalanceRecharge(_ context.Context, input billingcap.PrepareBalanceRecharge) (*billingcap.PreparedBalanceRecharge, error) {
	s.calls++
	return &billingcap.PreparedBalanceRecharge{PaymentAttemptNo: input.PaymentAttemptNo, RechargeContextNo: "BRC_capability_0001", CustomerRefDigest: strings.Repeat("f", 64), CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (s *channelPaymentCapabilityServiceStub) ConfirmBalanceRecharge(_ context.Context, input billingcap.ConfirmedBalanceRecharge) (*billingcap.BalanceRechargeResult, error) {
	s.calls++
	return &billingcap.BalanceRechargeResult{PaymentAttemptNo: input.PaymentAttemptNo, RechargeContextNo: input.RechargeContextNo, CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor, TransactionNo: "btx_4123456789abcdef0123456789abcdef", BalanceMinor: input.AmountMinor, AvailableMinor: input.AmountMinor}, nil
}

func (s *channelPaymentCapabilityServiceStub) PrepareChannelPayment(_ context.Context, input billingcap.PrepareChannelPayment) (*billingcap.PreparedChannelPayment, error) {
	s.calls++
	return &billingcap.PreparedChannelPayment{PaymentAttemptNo: input.PaymentAttemptNo, PaymentContextNo: input.PaymentContextNo, InvoiceNo: "INV_capability_0001", OrderNo: "ORD_capability_0001", CustomerRefDigest: strings.Repeat("c", 64), CurrencyCode: "CNY", TotalAmountMinor: 29900, ChannelAmountMinor: 29900, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (s *channelPaymentCapabilityServiceStub) ConfirmChannelPayment(
	_ context.Context,
	input billingcap.ConfirmedChannelPayment,
) (*billingcap.ChannelPaymentResult, error) {
	s.calls++
	return &billingcap.ChannelPaymentResult{
		PaymentAttemptNo: input.PaymentAttemptNo,
		TransactionNo:    "btx_0123456789abcdef0123456789abcdef",
		AllocationNo:     "bpa_0123456789abcdef0123456789abcdef",
		InvoiceNo:        input.InvoiceNo, OrderNo: "ORD_capability_0001",
		CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor,
		TotalAmountMinor: input.TotalAmountMinor, PaidAmountMinor: input.TotalAmountMinor, InvoiceStatus: "paid",
		SettlementNo: "fst_0123456789abcdef0123456789abcdef",
	}, nil
}

func (s *channelPaymentCapabilityServiceStub) ReleaseChannelPayment(_ context.Context, input billingcap.ReleaseChannelPayment) (*billingcap.ReleasedChannelPayment, error) {
	s.calls++
	return &billingcap.ReleasedChannelPayment{PaymentAttemptNo: input.PaymentAttemptNo, BalanceHoldNo: input.BalanceHoldNo, Status: "released"}, nil
}
func (s *channelPaymentCapabilityServiceStub) AllocateExceptionFund(_ context.Context, input billingcap.AllocateExceptionFund) (*billingcap.ExceptionFundAllocation, error) {
	s.calls++
	return &billingcap.ExceptionFundAllocation{ExceptionNo: input.ExceptionNo, PaymentAttemptNo: input.PaymentAttemptNo, TargetOrderNo: input.TargetOrderNo, InvoiceNo: "INV_capability_0001", AllocationNo: "bpa_0123456789abcdef0123456789abcdef", SettlementNo: "fst_0123456789abcdef0123456789abcdef", CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor}, nil
}
func(s *channelPaymentCapabilityServiceStub)ClaimPaymentClosure(context.Context)(*billingcap.PaymentClosureWork,error){s.calls++;return &billingcap.PaymentClosureWork{},nil}
func(s *channelPaymentCapabilityServiceStub)ReportPaymentClosure(_ context.Context,input billingcap.ReportPaymentClosure)(*billingcap.PaymentClosureReport,error){s.calls++;return &billingcap.PaymentClosureReport{CancellationNo:input.CancellationNo,StatusCode:"completed",OutcomeCode:input.OutcomeCode},nil}

func TestChannelPaymentCapabilityAcceptsOnlyPaymentOwner(t *testing.T) {
	t.Parallel()
	input := billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_capability_0001", InvoiceNo: "INV_capability_0001",
		CurrencyCode: "CNY", AmountMinor: 29900, TotalAmountMinor: 29900, EvidenceDigest: strings.Repeat("a", 64),
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	invoker := NewChannelPaymentCapabilityInvoker()
	invocation := capregistry.Invocation{
		CallerPluginID: billingcap.PaymentSourcePluginID,
		OwnerPluginID:  billingcap.OwnerPluginID, Service: billingcap.ServiceName,
		Version: string(billingcap.ContractVersion),
		Method:  string(billingcap.MethodChannelPaymentConfirm), Payload: payload,
	}
	if _, err = invoker.Invoke(context.Background(), invocation); err == nil {
		t.Fatal("unbound billing capability accepted traffic")
	}
	service := &channelPaymentCapabilityServiceStub{}
	if err = invoker.Bind(service); err != nil {
		t.Fatalf("bind billing capability: %v", err)
	}
	if _, err = invoker.Invoke(context.Background(), invocation); err != nil || service.calls != 1 {
		t.Fatalf("invoke payment owner calls=%d err=%v", service.calls, err)
	}
	foreign := invocation
	foreign.CallerPluginID = "other-payment-plugin"
	if _, err = invoker.Invoke(context.Background(), foreign); err == nil || service.calls != 1 {
		t.Fatalf("foreign payment caller reached owner calls=%d err=%v", service.calls, err)
	}
	foreign = invocation
	foreign.ResourceRef = input.InvoiceNo
	if _, err = invoker.Invoke(context.Background(), foreign); err == nil || service.calls != 1 {
		t.Fatalf("caller-controlled resource reached owner calls=%d err=%v", service.calls, err)
	}
}

func TestChannelPaymentSourceClientUsesSameInvoker(t *testing.T) {
	t.Parallel()
	invoker := NewChannelPaymentCapabilityInvoker()
	service := &channelPaymentCapabilityServiceStub{}
	if err := invoker.Bind(service); err != nil {
		t.Fatal(err)
	}
	client, err := NewChannelPaymentSourceClient(invoker)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ConfirmChannelPayment(context.Background(), billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_source_0001", InvoiceNo: "INV_source_0001",
		CurrencyCode: "CNY", AmountMinor: 29900, TotalAmountMinor: 29900, EvidenceDigest: strings.Repeat("b", 64),
	})
	if err != nil || result == nil || result.InvoiceStatus != "paid" || service.calls != 1 {
		t.Fatalf("source billing result=%+v calls=%d err=%v", result, service.calls, err)
	}
}
