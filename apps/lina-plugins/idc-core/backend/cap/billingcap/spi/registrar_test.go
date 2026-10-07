package spi

import (
	"context"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/billingcap"
)

type inertBillingService struct{}

func (inertBillingService) ClaimRefundExecution(context.Context, billingcap.ClaimRefundExecution) (*billingcap.RefundExecutionWork, error) {
	return nil, nil
}
func (inertBillingService) ReportRefundExecution(context.Context, billingcap.ReportRefundExecution) (*billingcap.RefundExecutionReport, error) {
	return nil, nil
}

func (inertBillingService) CurrentPaymentActor(context.Context) (*billingcap.PaymentActor, error) {
	return nil, nil
}
func (inertBillingService) PrepareCreditRepayment(context.Context, billingcap.PrepareCreditRepayment) (*billingcap.PreparedCreditRepayment, error) {
	return nil, nil
}
func (inertBillingService) ConfirmCreditRepayment(context.Context, billingcap.ConfirmedCreditRepayment) (*billingcap.CreditRepaymentResult, error) {
	return nil, nil
}
func (inertBillingService) PrepareBalanceRecharge(context.Context, billingcap.PrepareBalanceRecharge) (*billingcap.PreparedBalanceRecharge, error) {
	return nil, nil
}
func (inertBillingService) ConfirmBalanceRecharge(context.Context, billingcap.ConfirmedBalanceRecharge) (*billingcap.BalanceRechargeResult, error) {
	return nil, nil
}

func (inertBillingService) PrepareChannelPayment(context.Context, billingcap.PrepareChannelPayment) (*billingcap.PreparedChannelPayment, error) {
	return nil, nil
}

func (inertBillingService) ConfirmChannelPayment(
	context.Context,
	billingcap.ConfirmedChannelPayment,
) (*billingcap.ChannelPaymentResult, error) {
	return nil, nil
}

func (inertBillingService) ReleaseChannelPayment(context.Context, billingcap.ReleaseChannelPayment) (*billingcap.ReleasedChannelPayment, error) {
	return nil, nil
}
func (inertBillingService) AllocateExceptionFund(context.Context, billingcap.AllocateExceptionFund) (*billingcap.ExceptionFundAllocation, error) {
	return nil, nil
}
func (inertBillingService) ClaimPaymentClosure(context.Context) (*billingcap.PaymentClosureWork, error) {
	return &billingcap.PaymentClosureWork{}, nil
}
func (inertBillingService) ReportPaymentClosure(context.Context, billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	return nil, nil
}

func inertBillingFactory(context.Context) (billingcap.Service, error) {
	return inertBillingService{}, nil
}

func TestRegistrarFreezesOneBillingOwnerFactory(t *testing.T) {
	t.Parallel()
	r := &registrar{}
	if err := r.declare(inertBillingFactory); err != nil {
		t.Fatalf("declare billing source factory: %v", err)
	}
	if err := r.declare(inertBillingFactory); err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("duplicate billing declaration error=%v", err)
	}
	snapshot, err := r.freeze()
	if err != nil || snapshot.Factory() == nil {
		t.Fatalf("freeze billing source factory: factory=%v err=%v", snapshot.Factory(), err)
	}
	if err = r.declare(inertBillingFactory); err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("post-freeze billing declaration error=%v", err)
	}
}
