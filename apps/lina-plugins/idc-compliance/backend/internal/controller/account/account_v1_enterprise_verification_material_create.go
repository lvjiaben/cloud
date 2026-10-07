// This file maps the customer JSON reservation command to the enterprise service.

package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationMaterialCreate creates or replays a safe upload reservation.
func (c *ControllerV1) EnterpriseVerificationMaterialCreate(ctx context.Context, req *v1.EnterpriseVerificationMaterialCreateReq) (res *v1.EnterpriseVerificationMaterialCreateRes, err error) {
	if req == nil {
		return nil, bizerr.NewCode(enterpriseverification.CodeInvalid)
	}
	item, err := c.enterprise.ReserveMaterial(ctx, enterpriseverification.ReserveMaterialInput{
		CaseNo: req.CaseNo, MaterialType: enterpriseverification.MaterialType(req.MaterialTypeCode),
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationMaterialCreateRes{Item: &v1.EnterpriseVerificationMaterialReservationItem{
		MaterialNo: item.MaterialNo, MaterialTypeCode: string(item.MaterialType),
		StatusCode: item.StatusCode, Reused: item.Reused,
	}}, nil
}
