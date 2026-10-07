package billingcap

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
)

const (
	CapabilityChannelPaymentPrepare  = "host:idc:billing.channel-payment.prepare"
	CapabilityChannelPaymentConfirm  = "host:idc:billing.channel-payment.confirm"
	CapabilityChannelPaymentRelease  = "host:idc:billing.channel-payment.release"
	CapabilityExceptionFundAllocate  = "host:idc:billing.exception-fund.allocate"
	CapabilityPaymentClosureClaim    = "host:idc:billing.payment-closure.claim"
	CapabilityPaymentClosureReport   = "host:idc:billing.payment-closure.report"
	CapabilityPaymentActorCurrent    = "host:idc:billing.payment-actor.current"
	CapabilityCreditRepaymentPrepare = "host:idc:billing.credit-repayment.prepare"
	CapabilityCreditRepaymentConfirm = "host:idc:billing.credit-repayment.confirm"
	CapabilityBalanceRechargePrepare = "host:idc:billing.balance-recharge.prepare"
	CapabilityBalanceRechargeConfirm = "host:idc:billing.balance-recharge.confirm"
	CapabilityRefundExecutionClaim   = "host:idc:billing.refund-execution.claim"
	CapabilityRefundExecutionReport  = "host:idc:billing.refund-execution.report"
)

func NewDescriptor(invoker capregistry.Invoker) (capregistry.Descriptor, error) {
	if invoker == nil {
		return capregistry.Descriptor{}, gerror.New("billingcap: descriptor invoker is required")
	}
	return capregistry.Descriptor{
		OwnerPluginID: OwnerPluginID, Service: ServiceName,
		Version: string(ContractVersion), SourceContract: SourceContractPackage,
		DynamicContract: DynamicContractPackage, Invoker: invoker,
		Methods: []capregistry.MethodDescriptor{{
			Method:     string(MethodChannelPaymentPrepare),
			Capability: CapabilityChannelPaymentPrepare,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.PrepareChannelPayment",
			ResponsePayload: "billingcap.PreparedChannelPayment",
		}, {
			Method:     string(MethodChannelPaymentConfirm),
			Capability: CapabilityChannelPaymentConfirm,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.ConfirmedChannelPayment",
			ResponsePayload: "billingcap.ChannelPaymentResult",
		}, {
			Method:     string(MethodChannelPaymentRelease),
			Capability: CapabilityChannelPaymentRelease,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.ReleaseChannelPayment",
			ResponsePayload: "billingcap.ReleasedChannelPayment",
		}, {
			Method:     string(MethodExceptionFundAllocate),
			Capability: CapabilityExceptionFundAllocate,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.AllocateExceptionFund",
			ResponsePayload: "billingcap.ExceptionFundAllocation",
		}, {
			Method: string(MethodPaymentClosureClaim), Capability: CapabilityPaymentClosureClaim,
			Risk: capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload: "none", ResponsePayload: "billingcap.PaymentClosureWork",
		}, {
			Method: string(MethodPaymentClosureReport), Capability: CapabilityPaymentClosureReport,
			Risk: capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload: "billingcap.ReportPaymentClosure", ResponsePayload: "billingcap.PaymentClosureReport",
		}, {
			Method:     string(MethodPaymentActorCurrent),
			Capability: CapabilityPaymentActorCurrent,
			Risk:       capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "none",
			ResponsePayload: "billingcap.PaymentActor",
		}, {
			Method:     string(MethodCreditRepaymentPrepare),
			Capability: CapabilityCreditRepaymentPrepare,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.PrepareCreditRepayment",
			ResponsePayload: "billingcap.PreparedCreditRepayment",
		}, {
			Method:     string(MethodCreditRepaymentConfirm),
			Capability: CapabilityCreditRepaymentConfirm,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.ConfirmedCreditRepayment",
			ResponsePayload: "billingcap.CreditRepaymentResult",
		}, {
			Method:     string(MethodBalanceRechargePrepare),
			Capability: CapabilityBalanceRechargePrepare,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.PrepareBalanceRecharge",
			ResponsePayload: "billingcap.PreparedBalanceRecharge",
		}, {
			Method:     string(MethodBalanceRechargeConfirm),
			Capability: CapabilityBalanceRechargeConfirm,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.ConfirmedBalanceRecharge",
			ResponsePayload: "billingcap.BalanceRechargeResult",
		}, {
			Method:     string(MethodRefundExecutionClaim),
			Capability: CapabilityRefundExecutionClaim,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.ClaimRefundExecution",
			ResponsePayload: "billingcap.RefundExecutionWork",
		}, {
			Method:     string(MethodRefundExecutionReport),
			Capability: CapabilityRefundExecutionReport,
			Risk:       capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload:  "billingcap.ReportRefundExecution",
			ResponsePayload: "billingcap.RefundExecutionReport",
		}},
	}, nil
}
