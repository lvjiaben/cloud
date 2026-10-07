// This file submits a credit application derived from the current customer identity.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

// CreditApplicationSubmit submits an application for the current customer.
func (c *ControllerV1) CreditApplicationSubmit(ctx context.Context, req *v1.CreditApplicationSubmitReq) (res *v1.CreditApplicationSubmitRes, err error) {
	intent, err := creditApplicationIntent(req)
	if err != nil {
		return nil, err
	}
	application, err := c.creditCommands.SubmitCreditApplication(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.CreditApplicationSubmitRes{Application: toAPICreditApplication(application)}, nil
}
