package callback

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"lina-plugin-idc-payment/backend/api/callback/v1"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

func (c *ControllerV1) PaymentReceive(ctx context.Context, req *v1.PaymentReceiveReq) (*v1.PaymentReceiveRes, error) {
	request := g.RequestFromCtx(ctx)
	values := map[string]string{}
	for key, value := range request.GetFormMap() {
		values[key] = gconv.String(value)
	}
	headers := map[string]string{}
	for key, values := range request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	_, provider, err := c.flow.Callback(ctx, req.ChannelNo, paymentcap.CallbackRequest{Values: values, Body: append([]byte(nil), request.GetBody()...), Headers: headers})
	if err != nil {
		return nil, err
	}
	if provider == paymentcap.ProviderWechatPay {
		request.Response.Header().Set("Content-Type", "application/json")
		request.Response.Write(`{"code":"SUCCESS","message":"OK"}`)
	} else {
		request.Response.Write(strings.ToLower("SUCCESS"))
	}
	return nil, nil
}
