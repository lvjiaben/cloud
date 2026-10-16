// Staff verification delegates to the shared customer mutation owner.
package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
)

func (c *ControllerV1) CustomerVerificationUpdate(ctx context.Context, req *v1.CustomerVerificationUpdateReq) (*v1.CustomerVerificationUpdateRes, error) {
	service, ok := c.customerMutation.(customermanagement.VerificationService)
	if !ok {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	if err := service.UpdateVerification(ctx, customermanagement.VerificationInput{CustomerNo: req.CustomerNo, Action: customermanagement.VerificationAction(req.Action), ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey}); err != nil {
		return nil, err
	}
	return &v1.CustomerVerificationUpdateRes{}, nil
}
