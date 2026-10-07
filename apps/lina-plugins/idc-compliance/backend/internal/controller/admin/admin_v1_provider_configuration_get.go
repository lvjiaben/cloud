// This file maps the authorized service projection to the public secret-safe API response.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

// ProviderConfigurationGet returns the one authorized local configuration projection.
func (c *ControllerV1) ProviderConfigurationGet(
	ctx context.Context,
	_ *v1.ProviderConfigurationGetReq,
) (*v1.ProviderConfigurationGetRes, error) {
	configuration, err := c.providerConfiguration.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ProviderConfigurationGetRes{Item: toProviderConfigurationItem(configuration)}, nil
}

// toProviderConfigurationItem converts internal time and status values at the HTTP boundary.
func toProviderConfigurationItem(configuration *providerconfig.Configuration) *v1.ProviderConfigurationItem {
	if configuration == nil {
		return nil
	}
	item := &v1.ProviderConfigurationItem{
		ProviderCode: configuration.ProviderCode, Status: string(configuration.Status),
		SceneID: configuration.SceneID, CredentialConfigured: configuration.CredentialConfigured,
		CredentialMaskedTail: configuration.CredentialMaskedTail,
		CredentialGeneration: configuration.CredentialGeneration,
		ValidationStatus:     string(configuration.ValidationStatus), Version: configuration.Version,
	}
	if configuration.UpdatedAt != nil {
		updatedAt := configuration.UpdatedAt.UTC().UnixMilli()
		item.UpdatedAt = &updatedAt
	}
	if configuration.ValidatedAt != nil {
		validatedAt := configuration.ValidatedAt.UTC().UnixMilli()
		item.ValidatedAt = &validatedAt
	}
	return item
}
