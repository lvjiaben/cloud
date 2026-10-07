// This file maps subject revocations to the installation-local gateway workflow.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

// SupplierSubjectRevocationCreate revokes one opaque installation-local binding.
func (c *ControllerV1) SupplierSubjectRevocationCreate(
	ctx context.Context,
	req *v1.SupplierSubjectRevocationCreateReq,
) (*v1.SupplierSubjectRevocationCreateRes, error) {
	if c == nil || c.supplierConfiguration == nil || req == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	configuration, err := c.supplierConfiguration.RevokeSubject(
		ctx,
		supplierconfig.SubjectRevokeInput{
			ProviderCode: req.ProviderCode, BindingNo: req.BindingNo,
			ReasonCode: req.ReasonCode, ExpectedVersion: req.ExpectedVersion,
			IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	return &v1.SupplierSubjectRevocationCreateRes{
		Item: toSupplierConfigurationItem(configuration),
	}, nil
}
