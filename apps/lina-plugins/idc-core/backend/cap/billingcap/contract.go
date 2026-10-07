// Package billingcap publishes the stable, owner-aware payment-to-billing contract.
package billingcap

import (
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

type Version string
type Method string

const (
	OwnerPluginID                        = "idc-core"
	PaymentSourcePluginID                = "idc-payment"
	ServiceName                          = "billing"
	SourceContractPackage                = "lina-plugin-idc-core/backend/cap/billingcap"
	DynamicContractPackage               = SourceContractPackage + "/bridge"
	ContractVersion              Version = "v1"
	MethodChannelPaymentPrepare  Method  = "channel-payment.prepare"
	MethodChannelPaymentConfirm  Method  = "channel-payment.confirm"
	MethodChannelPaymentRelease  Method  = "channel-payment.release"
	MethodExceptionFundAllocate  Method  = "exception-fund.allocate"
	MethodPaymentClosureClaim    Method  = "payment-closure.claim"
	MethodPaymentClosureReport   Method  = "payment-closure.report"
	MethodPaymentActorCurrent    Method  = "payment-actor.current"
	MethodCreditRepaymentPrepare Method  = "credit-repayment.prepare"
	MethodCreditRepaymentConfirm Method  = "credit-repayment.confirm"
	MethodBalanceRechargePrepare Method  = "balance-recharge.prepare"
	MethodBalanceRechargeConfirm Method  = "balance-recharge.confirm"
	MethodRefundExecutionClaim   Method  = "refund-execution.claim"
	MethodRefundExecutionReport  Method  = "refund-execution.report"
)

type Dependency struct {
	OwnerPluginID string   `json:"ownerPluginId"`
	Service       string   `json:"service"`
	Version       Version  `json:"version"`
	Methods       []Method `json:"methods"`
}

func NewDependency(methods ...Method) (Dependency, error) {
	if len(methods) == 0 {
		methods = SupportedMethods()
	}
	dependency := Dependency{
		OwnerPluginID: OwnerPluginID, Service: ServiceName,
		Version: ContractVersion, Methods: append([]Method(nil), methods...),
	}
	if err := ValidateDependency(dependency); err != nil {
		return Dependency{}, err
	}
	return dependency, nil
}

func SupportedMethods() []Method {
	return []Method{MethodChannelPaymentPrepare, MethodChannelPaymentConfirm, MethodChannelPaymentRelease, MethodExceptionFundAllocate, MethodPaymentClosureClaim, MethodPaymentClosureReport, MethodPaymentActorCurrent, MethodCreditRepaymentPrepare, MethodCreditRepaymentConfirm, MethodBalanceRechargePrepare, MethodBalanceRechargeConfirm, MethodRefundExecutionClaim, MethodRefundExecutionReport}
}

func RequireCompatibleVersion(version Version) error {
	if version != ContractVersion {
		return gerror.Newf(
			"billingcap: incompatible contract version: required=%s available=%s",
			version, ContractVersion,
		)
	}
	return nil
}

func ValidateDependency(dependency Dependency) error {
	if dependency.OwnerPluginID != strings.TrimSpace(dependency.OwnerPluginID) ||
		dependency.OwnerPluginID != OwnerPluginID {
		return gerror.New("billingcap: incompatible owner plugin")
	}
	if dependency.Service != strings.TrimSpace(dependency.Service) ||
		dependency.Service != ServiceName {
		return gerror.New("billingcap: incompatible service")
	}
	if err := RequireCompatibleVersion(dependency.Version); err != nil {
		return err
	}
	if len(dependency.Methods) == 0 {
		return gerror.New("billingcap: dependency must require at least one method")
	}
	seen := make(map[Method]struct{}, len(dependency.Methods))
	for _, method := range dependency.Methods {
		if method != Method(strings.TrimSpace(string(method))) ||
			(method != MethodChannelPaymentPrepare && method != MethodChannelPaymentConfirm && method != MethodChannelPaymentRelease && method != MethodExceptionFundAllocate && method != MethodPaymentClosureClaim && method != MethodPaymentClosureReport && method != MethodPaymentActorCurrent && method != MethodCreditRepaymentPrepare && method != MethodCreditRepaymentConfirm && method != MethodBalanceRechargePrepare && method != MethodBalanceRechargeConfirm && method != MethodRefundExecutionClaim && method != MethodRefundExecutionReport) {
			return gerror.New("billingcap: dependency contains an unsupported method")
		}
		if _, exists := seen[method]; exists {
			return gerror.New("billingcap: dependency contains a duplicate method")
		}
		seen[method] = struct{}{}
	}
	return nil
}
