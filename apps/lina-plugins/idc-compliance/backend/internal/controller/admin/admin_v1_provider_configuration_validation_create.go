// This file delegates one read-only provider connectivity validation.
package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
)

// ProviderConfigurationValidationCreate returns only the persisted bounded outcome.
func (c *ControllerV1) ProviderConfigurationValidationCreate(
	ctx context.Context,
	_ *v1.ProviderConfigurationValidationCreateReq,
) (*v1.ProviderConfigurationValidationCreateRes, error) {
	configuration, err := c.providerConfiguration.ValidateConnectivity(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ProviderConfigurationValidationCreateRes{
		Item: toProviderConfigurationItem(configuration),
	}, nil
}
