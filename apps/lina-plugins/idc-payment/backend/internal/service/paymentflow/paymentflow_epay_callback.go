// This file admits EPay's signed GET notification through the same durable
// callback receipt and asynchronous confirmation flow as POST notifications.
package paymentflow

import (
	"context"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
)

// EPayCallback rejects other providers before verification or receipt creation.
// All merchant, amount, signature and replay checks remain owner-controlled.
func (s *Service) EPayCallback(ctx context.Context, channelNo string, input paymentcap.CallbackRequest) (*paymentattempt.CallbackOutcome, paymentcap.ProviderCode, error) {
	return s.callback(ctx, channelNo, input, paymentcap.ProviderEPay)
}
