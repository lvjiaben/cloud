// This file maps credential updates to the governed supplier configuration service.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

// SupplierCredentialUpdate clears the request copy after passing it to the service.
func (c *ControllerV1) SupplierCredentialUpdate(
	ctx context.Context,
	req *v1.SupplierCredentialUpdateReq,
) (*v1.SupplierCredentialUpdateRes, error) {
	if c == nil || c.supplierConfiguration == nil || req == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	credential := []byte(req.CredentialEnvelope)
	req.CredentialEnvelope = ""
	defer clearControllerBytes(credential)
	configuration, err := c.supplierConfiguration.SaveCredential(
		ctx,
		supplierconfig.CredentialSaveInput{
			ProviderCode: req.ProviderCode, CredentialEnvelope: credential,
			ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	return &v1.SupplierCredentialUpdateRes{Item: toSupplierConfigurationItem(configuration)}, nil
}
