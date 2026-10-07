package billing

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/billingcap"
)

type ChannelPaymentCapabilityInvoker struct {
	mutex   sync.RWMutex
	service billingcap.Service
}

type channelPaymentSourceClient struct {
	invoker *ChannelPaymentCapabilityInvoker
}

var _ billingcap.Service = (*channelPaymentSourceClient)(nil)
var _ capregistry.Invoker = (*ChannelPaymentCapabilityInvoker)(nil)

func NewChannelPaymentCapabilityInvoker() *ChannelPaymentCapabilityInvoker {
	return &ChannelPaymentCapabilityInvoker{}
}

func NewChannelPaymentSourceClient(
	invoker *ChannelPaymentCapabilityInvoker,
) (billingcap.Service, error) {
	if invoker == nil {
		return nil, gerror.New("channel payment source client requires an owner invoker")
	}
	return &channelPaymentSourceClient{invoker: invoker}, nil
}

func (c *channelPaymentSourceClient) CurrentPaymentActor(ctx context.Context) (*billingcap.PaymentActor, error) {
	var result billingcap.PaymentActor
	if err := c.invoke(ctx, billingcap.MethodPaymentActorCurrent, struct{}{}, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePaymentActor(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) PrepareChannelPayment(ctx context.Context, input billingcap.PrepareChannelPayment) (*billingcap.PreparedChannelPayment, error) {
	if err := billingcap.ValidatePrepareChannelPayment(input); err != nil {
		return nil, err
	}
	var result billingcap.PreparedChannelPayment
	if err := c.invoke(ctx, billingcap.MethodChannelPaymentPrepare, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePreparedChannelPayment(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) ConfirmChannelPayment(
	ctx context.Context,
	input billingcap.ConfirmedChannelPayment,
) (*billingcap.ChannelPaymentResult, error) {
	if c == nil || c.invoker == nil {
		return nil, gerror.New("channel payment source client is unavailable")
	}
	if err := billingcap.ValidateConfirmedChannelPayment(input); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, gerror.Wrap(err, "encode channel payment source request")
	}
	if len(payload) > billingcap.MaximumChannelPaymentPayloadBytes {
		return nil, gerror.New("channel payment source request exceeds its payload bound")
	}
	invoked, err := c.invoker.Invoke(ctx, capregistry.Invocation{
		CallerPluginID: billingcap.PaymentSourcePluginID,
		OwnerPluginID:  billingcap.OwnerPluginID, Service: billingcap.ServiceName,
		Version: string(billingcap.ContractVersion),
		Method:  string(billingcap.MethodChannelPaymentConfirm), Payload: payload,
	})
	if err != nil {
		return nil, err
	}
	if invoked == nil || invoked.Status != 0 || len(invoked.Payload) == 0 {
		return nil, gerror.New("channel payment source client received an invalid owner result")
	}
	var result billingcap.ChannelPaymentResult
	if err = json.Unmarshal(invoked.Payload, &result); err != nil {
		return nil, gerror.Wrap(err, "decode channel payment source result")
	}
	if err = billingcap.ValidateChannelPaymentResult(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) ReleaseChannelPayment(ctx context.Context, input billingcap.ReleaseChannelPayment) (*billingcap.ReleasedChannelPayment, error) {
	if err := billingcap.ValidateReleaseChannelPayment(input); err != nil {
		return nil, err
	}
	var result billingcap.ReleasedChannelPayment
	if err := c.invoke(ctx, billingcap.MethodChannelPaymentRelease, input, &result); err != nil {
		return nil, err
	}
	if result.PaymentAttemptNo != input.PaymentAttemptNo || result.BalanceHoldNo != input.BalanceHoldNo {
		return nil, gerror.New("channel payment release result is invalid")
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) AllocateExceptionFund(ctx context.Context, input billingcap.AllocateExceptionFund) (*billingcap.ExceptionFundAllocation, error) {
	if err := billingcap.ValidateAllocateExceptionFund(input); err != nil {
		return nil, err
	}
	var result billingcap.ExceptionFundAllocation
	if err := c.invoke(ctx, billingcap.MethodExceptionFundAllocate, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateExceptionFundAllocation(result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *channelPaymentSourceClient) ClaimPaymentClosure(ctx context.Context) (*billingcap.PaymentClosureWork, error) {
	var result billingcap.PaymentClosureWork
	if err := c.invoke(ctx, billingcap.MethodPaymentClosureClaim, struct{}{}, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePaymentClosureWork(result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *channelPaymentSourceClient) ReportPaymentClosure(ctx context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	if err := billingcap.ValidateReportPaymentClosure(input); err != nil {
		return nil, err
	}
	var result billingcap.PaymentClosureReport
	if err := c.invoke(ctx, billingcap.MethodPaymentClosureReport, input, &result); err != nil {
		return nil, err
	}
	if result.CancellationNo != input.CancellationNo || result.OutcomeCode != input.OutcomeCode {
		return nil, gerror.New("payment closure report result is invalid")
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) PrepareCreditRepayment(ctx context.Context, input billingcap.PrepareCreditRepayment) (*billingcap.PreparedCreditRepayment, error) {
	if err := billingcap.ValidatePrepareCreditRepayment(input); err != nil {
		return nil, err
	}
	var result billingcap.PreparedCreditRepayment
	if err := c.invoke(ctx, billingcap.MethodCreditRepaymentPrepare, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePreparedCreditRepayment(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) ConfirmCreditRepayment(ctx context.Context, input billingcap.ConfirmedCreditRepayment) (*billingcap.CreditRepaymentResult, error) {
	if err := billingcap.ValidateConfirmedCreditRepayment(input); err != nil {
		return nil, err
	}
	var result billingcap.CreditRepaymentResult
	if err := c.invoke(ctx, billingcap.MethodCreditRepaymentConfirm, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateCreditRepaymentResult(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) PrepareBalanceRecharge(ctx context.Context, input billingcap.PrepareBalanceRecharge) (*billingcap.PreparedBalanceRecharge, error) {
	if err := billingcap.ValidatePrepareBalanceRecharge(input); err != nil {
		return nil, err
	}
	var result billingcap.PreparedBalanceRecharge
	if err := c.invoke(ctx, billingcap.MethodBalanceRechargePrepare, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePreparedBalanceRecharge(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) ConfirmBalanceRecharge(ctx context.Context, input billingcap.ConfirmedBalanceRecharge) (*billingcap.BalanceRechargeResult, error) {
	if err := billingcap.ValidateConfirmedBalanceRecharge(input); err != nil {
		return nil, err
	}
	var result billingcap.BalanceRechargeResult
	if err := c.invoke(ctx, billingcap.MethodBalanceRechargeConfirm, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateBalanceRechargeResult(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) ClaimRefundExecution(
	ctx context.Context,
	input billingcap.ClaimRefundExecution,
) (*billingcap.RefundExecutionWork, error) {
	if err := billingcap.ValidateClaimRefundExecution(input); err != nil {
		return nil, err
	}
	var result billingcap.RefundExecutionWork
	if err := c.invoke(ctx, billingcap.MethodRefundExecutionClaim, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateRefundExecutionWork(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) ReportRefundExecution(
	ctx context.Context,
	input billingcap.ReportRefundExecution,
) (*billingcap.RefundExecutionReport, error) {
	if err := billingcap.ValidateReportRefundExecution(input); err != nil {
		return nil, err
	}
	var result billingcap.RefundExecutionReport
	if err := c.invoke(ctx, billingcap.MethodRefundExecutionReport, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateRefundExecutionReport(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *channelPaymentSourceClient) invoke(ctx context.Context, method billingcap.Method, input any, output any) error {
	if c == nil || c.invoker == nil {
		return gerror.New("channel payment source client is unavailable")
	}
	payload, err := json.Marshal(input)
	if err != nil || len(payload) == 0 || len(payload) > billingcap.MaximumChannelPaymentPayloadBytes {
		return gerror.New("channel payment source request is invalid")
	}
	invoked, err := c.invoker.Invoke(ctx, capregistry.Invocation{CallerPluginID: billingcap.PaymentSourcePluginID, OwnerPluginID: billingcap.OwnerPluginID, Service: billingcap.ServiceName, Version: string(billingcap.ContractVersion), Method: string(method), Payload: payload})
	if err != nil {
		return err
	}
	if invoked == nil || invoked.Status != 0 || len(invoked.Payload) == 0 {
		return gerror.New("channel payment source client received an invalid owner result")
	}
	if err = json.Unmarshal(invoked.Payload, output); err != nil {
		return gerror.Wrap(err, "decode channel payment source result")
	}
	return nil
}

func (i *ChannelPaymentCapabilityInvoker) Bind(service billingcap.Service) error {
	if i == nil || service == nil {
		return gerror.New("channel payment capability invoker requires a service")
	}
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if i.service != nil {
		return gerror.New("channel payment capability invoker is already bound")
	}
	i.service = service
	return nil
}

func (i *ChannelPaymentCapabilityInvoker) Invoke(
	ctx context.Context,
	invocation capregistry.Invocation,
) (*capregistry.InvocationResult, error) {
	if i == nil {
		return nil, gerror.New("channel payment capability invoker is unavailable")
	}
	i.mutex.RLock()
	service := i.service
	i.mutex.RUnlock()
	if service == nil {
		return nil, gerror.New("channel payment capability invoker is unavailable")
	}
	if invocation.CallerPluginID != billingcap.PaymentSourcePluginID ||
		invocation.OwnerPluginID != billingcap.OwnerPluginID ||
		invocation.Service != billingcap.ServiceName ||
		invocation.Version != string(billingcap.ContractVersion) ||
		!isChannelPaymentCapabilityMethod(billingcap.Method(invocation.Method)) ||
		invocation.ResourceRef != "" || invocation.Table != "" {
		return nil, gerror.New("channel payment capability invocation is not authorized")
	}
	if len(invocation.Payload) == 0 ||
		len(invocation.Payload) > billingcap.MaximumChannelPaymentPayloadBytes {
		return nil, gerror.New("channel payment capability payload is outside its size bound")
	}
	result, err := invokeChannelPaymentCapability(
		ctx, service, billingcap.Method(invocation.Method), invocation.Payload,
	)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, gerror.Wrap(err, "encode channel payment capability result")
	}
	return &capregistry.InvocationResult{Payload: payload}, nil
}

func isChannelPaymentCapabilityMethod(method billingcap.Method) bool {
	switch method {
	case billingcap.MethodPaymentActorCurrent:
	case billingcap.MethodChannelPaymentPrepare:
	case billingcap.MethodChannelPaymentConfirm:
	case billingcap.MethodChannelPaymentRelease:
	case billingcap.MethodExceptionFundAllocate:
	case billingcap.MethodPaymentClosureClaim:
	case billingcap.MethodPaymentClosureReport:
	case billingcap.MethodCreditRepaymentPrepare:
	case billingcap.MethodCreditRepaymentConfirm:
	case billingcap.MethodBalanceRechargePrepare:
	case billingcap.MethodBalanceRechargeConfirm:
	case billingcap.MethodRefundExecutionClaim:
	case billingcap.MethodRefundExecutionReport:
	default:
		return false
	}
	return true
}

func invokeChannelPaymentCapability(
	ctx context.Context,
	service billingcap.Service,
	method billingcap.Method,
	payload []byte,
) (any, error) {
	switch method {
	case billingcap.MethodPaymentActorCurrent,
		billingcap.MethodChannelPaymentPrepare,
		billingcap.MethodChannelPaymentConfirm,
		billingcap.MethodChannelPaymentRelease:
		return invokeChannelPayment(ctx, service, method, payload)
	case billingcap.MethodExceptionFundAllocate, billingcap.MethodPaymentClosureClaim, billingcap.MethodPaymentClosureReport:
		return invokeExceptionFundAndClosure(ctx, service, method, payload)
	case billingcap.MethodCreditRepaymentPrepare, billingcap.MethodCreditRepaymentConfirm:
		return invokeCreditRepayment(ctx, service, method, payload)
	case billingcap.MethodBalanceRechargePrepare, billingcap.MethodBalanceRechargeConfirm:
		return invokeBalanceRecharge(ctx, service, method, payload)
	case billingcap.MethodRefundExecutionClaim, billingcap.MethodRefundExecutionReport:
		return invokeRefundExecution(ctx, service, method, payload)
	default:
		return nil, gerror.New("channel payment capability method is unavailable")
	}
}

func invokeChannelPayment(
	ctx context.Context,
	service billingcap.Service,
	method billingcap.Method,
	payload []byte,
) (any, error) {
	switch method {
	case billingcap.MethodPaymentActorCurrent:
		actor, err := service.CurrentPaymentActor(ctx)
		if err != nil {
			return nil, err
		}
		if actor == nil || billingcap.ValidatePaymentActor(*actor) != nil {
			return nil, gerror.New("channel payment owner returned an invalid actor")
		}
		return actor, nil
	case billingcap.MethodChannelPaymentPrepare:
		var input billingcap.PrepareChannelPayment
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidatePrepareChannelPayment(input) != nil {
			return nil, gerror.New("decode channel payment preparation request")
		}
		prepared, err := service.PrepareChannelPayment(ctx, input)
		if err != nil {
			return nil, err
		}
		if prepared == nil || billingcap.ValidatePreparedChannelPayment(*prepared) != nil {
			return nil, gerror.New("channel payment owner returned an invalid preparation")
		}
		return prepared, nil
	case billingcap.MethodChannelPaymentConfirm:
		var input billingcap.ConfirmedChannelPayment
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateConfirmedChannelPayment(input) != nil {
			return nil, gerror.New("decode channel payment confirmation request")
		}
		confirmed, err := service.ConfirmChannelPayment(ctx, input)
		if err != nil {
			return nil, err
		}
		if confirmed == nil || billingcap.ValidateChannelPaymentResult(*confirmed) != nil {
			return nil, gerror.New("channel payment owner returned an invalid result")
		}
		return confirmed, nil
	case billingcap.MethodChannelPaymentRelease:
		var input billingcap.ReleaseChannelPayment
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateReleaseChannelPayment(input) != nil {
			return nil, gerror.New("decode channel payment release request")
		}
		released, err := service.ReleaseChannelPayment(ctx, input)
		if err != nil {
			return nil, err
		}
		if released == nil || released.PaymentAttemptNo != input.PaymentAttemptNo || released.BalanceHoldNo != input.BalanceHoldNo {
			return nil, gerror.New("channel payment owner returned an invalid release")
		}
		return released, nil
	default:
		return nil, gerror.New("channel payment method is unavailable")
	}
}

func invokeExceptionFundAndClosure(
	ctx context.Context,
	service billingcap.Service,
	method billingcap.Method,
	payload []byte,
) (any, error) {
	switch method {
	case billingcap.MethodExceptionFundAllocate:
		var input billingcap.AllocateExceptionFund
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateAllocateExceptionFund(input) != nil {
			return nil, gerror.New("decode exception fund allocation request")
		}
		allocated, err := service.AllocateExceptionFund(ctx, input)
		if err != nil {
			return nil, err
		}
		if allocated == nil || billingcap.ValidateExceptionFundAllocation(*allocated) != nil {
			return nil, gerror.New("channel payment owner returned an invalid exception fund allocation")
		}
		return allocated, nil
	case billingcap.MethodPaymentClosureClaim:
		work, err := service.ClaimPaymentClosure(ctx)
		if err != nil {
			return nil, err
		}
		if work == nil || billingcap.ValidatePaymentClosureWork(*work) != nil {
			return nil, gerror.New("payment closure owner returned invalid work")
		}
		return work, nil
	case billingcap.MethodPaymentClosureReport:
		var input billingcap.ReportPaymentClosure
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateReportPaymentClosure(input) != nil {
			return nil, gerror.New("decode payment closure report")
		}
		report, err := service.ReportPaymentClosure(ctx, input)
		if err != nil {
			return nil, err
		}
		if report == nil || report.CancellationNo != input.CancellationNo || report.OutcomeCode != input.OutcomeCode {
			return nil, gerror.New("payment closure owner returned invalid report")
		}
		return report, nil
	default:
		return nil, gerror.New("exception fund and closure method is unavailable")
	}
}

func invokeCreditRepayment(
	ctx context.Context,
	service billingcap.Service,
	method billingcap.Method,
	payload []byte,
) (any, error) {
	switch method {
	case billingcap.MethodCreditRepaymentPrepare:
		var input billingcap.PrepareCreditRepayment
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidatePrepareCreditRepayment(input) != nil {
			return nil, gerror.New("decode credit repayment preparation request")
		}
		prepared, err := service.PrepareCreditRepayment(ctx, input)
		if err != nil {
			return nil, err
		}
		if prepared == nil || billingcap.ValidatePreparedCreditRepayment(*prepared) != nil {
			return nil, gerror.New("credit repayment owner returned an invalid preparation")
		}
		return prepared, nil
	case billingcap.MethodCreditRepaymentConfirm:
		var input billingcap.ConfirmedCreditRepayment
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateConfirmedCreditRepayment(input) != nil {
			return nil, gerror.New("decode credit repayment confirmation request")
		}
		confirmed, err := service.ConfirmCreditRepayment(ctx, input)
		if err != nil {
			return nil, err
		}
		if confirmed == nil || billingcap.ValidateCreditRepaymentResult(*confirmed) != nil {
			return nil, gerror.New("credit repayment owner returned an invalid result")
		}
		return confirmed, nil
	default:
		return nil, gerror.New("credit repayment method is unavailable")
	}
}

func invokeBalanceRecharge(
	ctx context.Context,
	service billingcap.Service,
	method billingcap.Method,
	payload []byte,
) (any, error) {
	switch method {
	case billingcap.MethodBalanceRechargePrepare:
		var input billingcap.PrepareBalanceRecharge
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidatePrepareBalanceRecharge(input) != nil {
			return nil, gerror.New("decode balance recharge preparation request")
		}
		prepared, err := service.PrepareBalanceRecharge(ctx, input)
		if err != nil {
			return nil, err
		}
		if prepared == nil || billingcap.ValidatePreparedBalanceRecharge(*prepared) != nil {
			return nil, gerror.New("balance recharge owner returned an invalid preparation")
		}
		return prepared, nil
	case billingcap.MethodBalanceRechargeConfirm:
		var input billingcap.ConfirmedBalanceRecharge
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateConfirmedBalanceRecharge(input) != nil {
			return nil, gerror.New("decode balance recharge confirmation request")
		}
		confirmed, err := service.ConfirmBalanceRecharge(ctx, input)
		if err != nil {
			return nil, err
		}
		if confirmed == nil || billingcap.ValidateBalanceRechargeResult(*confirmed) != nil {
			return nil, gerror.New("balance recharge owner returned an invalid result")
		}
		return confirmed, nil
	default:
		return nil, gerror.New("balance recharge method is unavailable")
	}
}

func invokeRefundExecution(
	ctx context.Context,
	service billingcap.Service,
	method billingcap.Method,
	payload []byte,
) (any, error) {
	switch method {
	case billingcap.MethodRefundExecutionClaim:
		var input billingcap.ClaimRefundExecution
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateClaimRefundExecution(input) != nil {
			return nil, gerror.New("decode refund execution claim")
		}
		claimed, err := service.ClaimRefundExecution(ctx, input)
		if err != nil {
			return nil, err
		}
		if claimed == nil || billingcap.ValidateRefundExecutionWork(*claimed) != nil {
			return nil, gerror.New("refund owner returned invalid work")
		}
		return claimed, nil
	case billingcap.MethodRefundExecutionReport:
		var input billingcap.ReportRefundExecution
		if err := json.Unmarshal(payload, &input); err != nil || billingcap.ValidateReportRefundExecution(input) != nil {
			return nil, gerror.New("decode refund execution report")
		}
		reported, err := service.ReportRefundExecution(ctx, input)
		if err != nil {
			return nil, err
		}
		if reported == nil || billingcap.ValidateRefundExecutionReport(*reported) != nil {
			return nil, gerror.New("refund owner returned invalid report result")
		}
		return reported, nil
	default:
		return nil, gerror.New("refund execution method is unavailable")
	}
}
