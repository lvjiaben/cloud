// Package bridge provides the dynamic guest SDK for billingcap v1.
package bridge

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-core/backend/cap/billingcap"
)

type OwnerHostServiceInvoker func(
	owner string,
	service string,
	version string,
	method string,
	resourceRef string,
	table string,
	payload []byte,
) ([]byte, error)

type Client struct{ invoke OwnerHostServiceInvoker }

var _ billingcap.Service = (*Client)(nil)

func NewClient(invoke OwnerHostServiceInvoker) (*Client, error) {
	if invoke == nil {
		return nil, gerror.New("billingcap bridge: owner host-service invoker is required")
	}
	return &Client{invoke: invoke}, nil
}

func NewGuestClient() (*Client, error) {
	return NewClient(pluginbridge.InvokeOwnerHostService)
}

func (c *Client) CurrentPaymentActor(ctx context.Context) (*billingcap.PaymentActor, error) {
	var result billingcap.PaymentActor
	if err := c.invokeJSON(ctx, billingcap.MethodPaymentActorCurrent, struct{}{}, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePaymentActor(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) PrepareChannelPayment(ctx context.Context, input billingcap.PrepareChannelPayment) (*billingcap.PreparedChannelPayment, error) {
	var result billingcap.PreparedChannelPayment
	if err := c.invokeJSON(ctx, billingcap.MethodChannelPaymentPrepare, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePreparedChannelPayment(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ConfirmChannelPayment(
	ctx context.Context,
	input billingcap.ConfirmedChannelPayment,
) (*billingcap.ChannelPaymentResult, error) {
	if c == nil || c.invoke == nil {
		return nil, gerror.New("billingcap bridge: client is unavailable")
	}
	if ctx == nil {
		return nil, gerror.New("billingcap bridge: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, gerror.Wrap(err, "billingcap bridge context is closed")
	}
	if err := billingcap.ValidateConfirmedChannelPayment(input); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, gerror.Wrap(err, "encode billingcap channel payment")
	}
	if len(payload) > billingcap.MaximumChannelPaymentPayloadBytes {
		return nil, gerror.New("billingcap bridge: channel payment payload is too large")
	}
	response, err := c.invoke(
		billingcap.OwnerPluginID, billingcap.ServiceName,
		string(billingcap.ContractVersion), string(billingcap.MethodChannelPaymentConfirm),
		"", "", payload,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "invoke billingcap channel payment")
	}
	if len(response) == 0 || len(response) > billingcap.MaximumChannelPaymentPayloadBytes {
		return nil, gerror.New("billingcap bridge: channel payment response is invalid")
	}
	var result billingcap.ChannelPaymentResult
	if err = json.Unmarshal(response, &result); err != nil {
		return nil, gerror.Wrap(err, "decode billingcap channel payment result")
	}
	if err = billingcap.ValidateChannelPaymentResult(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReleaseChannelPayment(ctx context.Context, input billingcap.ReleaseChannelPayment) (*billingcap.ReleasedChannelPayment, error) {
	var result billingcap.ReleasedChannelPayment
	if err := c.invokeJSON(ctx, billingcap.MethodChannelPaymentRelease, input, &result); err != nil {
		return nil, err
	}
	if result.PaymentAttemptNo != input.PaymentAttemptNo || result.BalanceHoldNo != input.BalanceHoldNo ||
		(result.Status != "released" && result.Status != "expired" && result.Status != "consumed") {
		return nil, gerror.New("billingcap bridge: channel payment release result is invalid")
	}
	return &result, nil
}

func (c *Client) AllocateExceptionFund(ctx context.Context, input billingcap.AllocateExceptionFund) (*billingcap.ExceptionFundAllocation, error) {
	if err := billingcap.ValidateAllocateExceptionFund(input); err != nil {
		return nil, err
	}
	var result billingcap.ExceptionFundAllocation
	if err := c.invokeJSON(ctx, billingcap.MethodExceptionFundAllocate, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateExceptionFundAllocation(result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *Client) ClaimPaymentClosure(ctx context.Context) (*billingcap.PaymentClosureWork, error) {
	var result billingcap.PaymentClosureWork
	if err := c.invokeJSON(ctx, billingcap.MethodPaymentClosureClaim, struct{}{}, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePaymentClosureWork(result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (c *Client) ReportPaymentClosure(ctx context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	if err := billingcap.ValidateReportPaymentClosure(input); err != nil {
		return nil, err
	}
	var result billingcap.PaymentClosureReport
	if err := c.invokeJSON(ctx, billingcap.MethodPaymentClosureReport, input, &result); err != nil {
		return nil, err
	}
	if result.CancellationNo != input.CancellationNo || result.OutcomeCode != input.OutcomeCode {
		return nil, gerror.New("billingcap bridge: payment closure report is invalid")
	}
	return &result, nil
}

func (c *Client) PrepareCreditRepayment(ctx context.Context, input billingcap.PrepareCreditRepayment) (*billingcap.PreparedCreditRepayment, error) {
	if err := billingcap.ValidatePrepareCreditRepayment(input); err != nil {
		return nil, err
	}
	var result billingcap.PreparedCreditRepayment
	if err := c.invokeJSON(ctx, billingcap.MethodCreditRepaymentPrepare, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePreparedCreditRepayment(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ConfirmCreditRepayment(ctx context.Context, input billingcap.ConfirmedCreditRepayment) (*billingcap.CreditRepaymentResult, error) {
	if err := billingcap.ValidateConfirmedCreditRepayment(input); err != nil {
		return nil, err
	}
	var result billingcap.CreditRepaymentResult
	if err := c.invokeJSON(ctx, billingcap.MethodCreditRepaymentConfirm, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateCreditRepaymentResult(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) PrepareBalanceRecharge(ctx context.Context, input billingcap.PrepareBalanceRecharge) (*billingcap.PreparedBalanceRecharge, error) {
	if err := billingcap.ValidatePrepareBalanceRecharge(input); err != nil {
		return nil, err
	}
	var result billingcap.PreparedBalanceRecharge
	if err := c.invokeJSON(ctx, billingcap.MethodBalanceRechargePrepare, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidatePreparedBalanceRecharge(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ConfirmBalanceRecharge(ctx context.Context, input billingcap.ConfirmedBalanceRecharge) (*billingcap.BalanceRechargeResult, error) {
	if err := billingcap.ValidateConfirmedBalanceRecharge(input); err != nil {
		return nil, err
	}
	var result billingcap.BalanceRechargeResult
	if err := c.invokeJSON(ctx, billingcap.MethodBalanceRechargeConfirm, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateBalanceRechargeResult(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ClaimRefundExecution(
	ctx context.Context,
	input billingcap.ClaimRefundExecution,
) (*billingcap.RefundExecutionWork, error) {
	if err := billingcap.ValidateClaimRefundExecution(input); err != nil {
		return nil, err
	}
	var result billingcap.RefundExecutionWork
	if err := c.invokeJSON(ctx, billingcap.MethodRefundExecutionClaim, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateRefundExecutionWork(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReportRefundExecution(
	ctx context.Context,
	input billingcap.ReportRefundExecution,
) (*billingcap.RefundExecutionReport, error) {
	if err := billingcap.ValidateReportRefundExecution(input); err != nil {
		return nil, err
	}
	var result billingcap.RefundExecutionReport
	if err := c.invokeJSON(ctx, billingcap.MethodRefundExecutionReport, input, &result); err != nil {
		return nil, err
	}
	if err := billingcap.ValidateRefundExecutionReport(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) invokeJSON(ctx context.Context, method billingcap.Method, input any, output any) error {
	if c == nil || c.invoke == nil || ctx == nil {
		return gerror.New("billingcap bridge: client or context is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return gerror.Wrap(err, "billingcap bridge context is closed")
	}
	payload, err := json.Marshal(input)
	if err != nil || len(payload) == 0 || len(payload) > billingcap.MaximumChannelPaymentPayloadBytes {
		return gerror.New("billingcap bridge: channel payment payload is invalid")
	}
	response, err := c.invoke(billingcap.OwnerPluginID, billingcap.ServiceName, string(billingcap.ContractVersion), string(method), "", "", payload)
	if err != nil {
		return gerror.Wrap(err, "invoke billingcap channel payment")
	}
	if len(response) == 0 || len(response) > billingcap.MaximumChannelPaymentPayloadBytes {
		return gerror.New("billingcap bridge: channel payment response is invalid")
	}
	if err = json.Unmarshal(response, output); err != nil {
		return gerror.Wrap(err, "decode billingcap channel payment result")
	}
	return nil
}
