// This file maps subject-binding start requests to the external-confirmation gate.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

// SupplierSubjectBindingCreate enforces the currently closed external evidence gate.
func (c *ControllerV1) SupplierSubjectBindingCreate(
	ctx context.Context,
	req *v1.SupplierSubjectBindingCreateReq,
) (*v1.SupplierSubjectBindingCreateRes, error) {
	if c == nil || c.supplierConfiguration == nil || req == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	if err := c.supplierConfiguration.StartSubjectBinding(ctx, req.ProviderCode); err != nil {
		return nil, err
	}
	return &v1.SupplierSubjectBindingCreateRes{}, nil
}
