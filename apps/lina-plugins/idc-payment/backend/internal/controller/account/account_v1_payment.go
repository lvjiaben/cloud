package account

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-payment/backend/api/account/v1"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
)

func (c *ControllerV1) PaymentChannelList(ctx context.Context, req *v1.PaymentChannelListReq) (*v1.PaymentChannelListRes, error) {
	currency := strings.ToUpper(strings.TrimSpace(req.CurrencyCode))
	channels, err := c.flow.AvailableChannels(ctx, currency)
	if err != nil {
		return nil, err
	}
	result := &v1.PaymentChannelListRes{Channels: make([]*v1.PaymentChannelItem, 0, len(channels))}
	for _, channel := range channels {
		modes := make([]string, len(channel.Modes))
		for index := range channel.Modes {
			modes[index] = string(channel.Modes[index])
		}
		result.Channels = append(result.Channels, &v1.PaymentChannelItem{ChannelNo: channel.ChannelNo, DisplayName: channel.DisplayName, Provider: string(channel.Provider), Modes: modes, Currencies: append([]string(nil), channel.Currencies...)})
	}
	return result, nil
}

func (c *ControllerV1) PaymentCreate(ctx context.Context, req *v1.PaymentCreateReq) (*v1.PaymentCreateRes, error) {
	balanceMinor, err := strconv.ParseInt(req.BalanceAmountMinor, 10, 64)
	if err != nil || balanceMinor < 0 {
		return nil, errors.New("invalid balance amount")
	}
	created, err := c.flow.Create(ctx, paymentflow.CreateInput{PaymentContextNo: req.PaymentContextNo, ChannelNo: req.ChannelNo, Mode: paymentcap.Mode(req.ModeCode), BalanceAmountMinor: balanceMinor, ClientIP: g.RequestFromCtx(ctx).GetClientIp(), OpenID: req.OpenID})
	if err != nil {
		return nil, err
	}
	return &v1.PaymentCreateRes{AttemptNo: created.AttemptNo, StatusCode: created.StatusCode, RedirectMethod: created.Redirect.Method, RedirectURL: created.Redirect.URL, RedirectFields: created.Redirect.Fields, ExpiresAt: created.ExpiresAt.UnixMilli()}, nil
}

func (c *ControllerV1) PaymentStatus(ctx context.Context, req *v1.PaymentStatusReq) (*v1.PaymentStatusRes, error) {
	status, err := c.flow.Status(ctx, req.AttemptNo)
	if err != nil {
		return nil, err
	}
	var confirmedAt *int64
	if !status.ConfirmedAt.IsZero() {
		value := status.ConfirmedAt.UnixMilli()
		confirmedAt = &value
	}
	return &v1.PaymentStatusRes{InvoiceNo: status.InvoiceNo, OrderNo: status.OrderNo, AttemptNo: status.AttemptNo, StatusCode: status.StatusCode, ChannelNo: status.ChannelNo, ModeCode: status.ModeCode, CurrencyCode: status.CurrencyCode, PurposeCode: status.PurposeCode, PurposeReference: status.PurposeReference, TotalAmountMinor: strconv.FormatInt(status.TotalAmountMinor, 10), ChannelAmountMinor: strconv.FormatInt(status.ChannelAmountMinor, 10), BalanceAmountMinor: strconv.FormatInt(status.BalanceAmountMinor, 10), ExpiresAt: status.ExpiresAt.UnixMilli(), ConfirmedAt: confirmedAt, QueryQueued: status.QueryQueued}, nil
}

func (c *ControllerV1) CreditRepaymentCreate(ctx context.Context, req *v1.CreditRepaymentCreateReq) (*v1.CreditRepaymentCreateRes, error) {
	amountMinor, err := strconv.ParseInt(req.AmountMinor, 10, 64)
	if err != nil || amountMinor <= 0 {
		return nil, errors.New("invalid credit repayment amount")
	}
	created, err := c.flow.CreateCreditRepayment(ctx, paymentflow.CreditCreateInput{StatementNo: req.StatementNo, ChannelNo: req.ChannelNo, AmountMinor: amountMinor, Mode: paymentcap.Mode(req.ModeCode), ClientIP: g.RequestFromCtx(ctx).GetClientIp(), OpenID: req.OpenID})
	if err != nil {
		return nil, err
	}
	return &v1.CreditRepaymentCreateRes{AttemptNo: created.AttemptNo, StatusCode: created.StatusCode, RedirectMethod: created.Redirect.Method, RedirectURL: created.Redirect.URL, RedirectFields: created.Redirect.Fields, ExpiresAt: created.ExpiresAt.UnixMilli()}, nil
}

func (c *ControllerV1) BalanceRechargeCreate(ctx context.Context, req *v1.BalanceRechargeCreateReq) (*v1.BalanceRechargeCreateRes, error) {
	amountMinor, err := strconv.ParseInt(req.AmountMinor, 10, 64)
	if err != nil || amountMinor <= 0 {
		return nil, errors.New("invalid balance recharge amount")
	}
	created, err := c.flow.CreateBalanceRecharge(ctx, paymentflow.RechargeCreateInput{ChannelNo: req.ChannelNo, CurrencyCode: req.CurrencyCode, AmountMinor: amountMinor, Mode: paymentcap.Mode(req.ModeCode), ClientIP: g.RequestFromCtx(ctx).GetClientIp(), OpenID: req.OpenID})
	if err != nil {
		return nil, err
	}
	return &v1.BalanceRechargeCreateRes{AttemptNo: created.AttemptNo, StatusCode: created.StatusCode, RedirectMethod: created.Redirect.Method, RedirectURL: created.Redirect.URL, RedirectFields: created.Redirect.Fields, ExpiresAt: created.ExpiresAt.UnixMilli()}, nil
}
