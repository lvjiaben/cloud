// This generated controller records offline commission evidence only.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
	"time"
)

func (c *ControllerV1) SalesManagementCommissionPayoutCreate(ctx context.Context, req *v1.SalesManagementCommissionPayoutCreateReq) (*v1.SalesManagementCommissionPayoutCreateRes, error) {
	owner, ok := c.salesManagement.(salesmanagement.CommissionService)
	if !ok {
		return nil, gerror.New("commission service unavailable")
	}
	item, err := owner.RecordCommissionPayout(ctx, salesmanagement.CommissionPayoutInput{AccountNo: req.AccountNo, AmountMinor: req.AmountMinor, EvidenceReference: req.EvidenceReference, IdempotencyKey: req.IdempotencyKey, PaidAt: time.UnixMilli(req.PaidAt), ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementCommissionPayoutCreateRes{Item: salesManagementCommissionPayout(item)}, nil
}
