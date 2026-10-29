// This file receives POST callbacks and EPay's signed GET notifications. Query
// values are parsed exactly once; ambiguous values never reach verification.
package callback

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"lina-core/pkg/bizerr"

	"lina-plugin-idc-payment/backend/api/callback/v1"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
)

func (c *ControllerV1) PaymentReceive(ctx context.Context, req *v1.PaymentReceiveReq) (*v1.PaymentReceiveRes, error) {
	request := g.RequestFromCtx(ctx)
	values := map[string]string{}
	if request.Method == http.MethodGet {
		query, err := url.ParseQuery(request.URL.RawQuery)
		if err != nil {
			return nil, bizerr.NewCode(paymentflow.CodeEPayNotificationInvalid)
		}
		for key, entries := range query {
			if len(entries) != 1 {
				return nil, bizerr.NewCode(paymentflow.CodeEPayNotificationInvalid)
			}
			values[key] = entries[0]
		}
	} else {
		for key, value := range request.GetFormMap() {
			values[key] = gconv.String(value)
		}
	}
	headers := map[string]string{}
	for key, values := range request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	input := paymentcap.CallbackRequest{Values: values, Body: append([]byte(nil), request.GetBody()...), Headers: headers}
	callback := c.flow.Callback
	if request.Method == http.MethodGet {
		callback = c.flow.EPayCallback
	}
	_, provider, err := callback(ctx, req.ChannelNo, input)
	if err != nil {
		return nil, err
	}
	request.Response.Header().Set("Cache-Control", "no-store")
	if provider == paymentcap.ProviderWechatPay {
		request.Response.Header().Set("Content-Type", "application/json")
		request.Response.Write(`{"code":"SUCCESS","message":"OK"}`)
	} else {
		request.Response.Write(strings.ToLower("SUCCESS"))
	}
	return nil, nil
}
