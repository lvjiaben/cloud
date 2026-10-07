// This file maps subject-reopening start requests to the external-confirmation gate.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

// SupplierSubjectReopeningCreate enforces the currently closed external evidence gate.
func (c *ControllerV1) SupplierSubjectReopeningCreate(
	ctx context.Context,
	req *v1.SupplierSubjectReopeningCreateReq,
) (*v1.SupplierSubjectReopeningCreateRes, error) {
	if c == nil || c.supplierConfiguration == nil || req == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	if err := c.supplierConfiguration.StartSubjectReopening(ctx, req.ProviderCode); err != nil {
		return nil, err
	}
	return &v1.SupplierSubjectReopeningCreateRes{}, nil
}
