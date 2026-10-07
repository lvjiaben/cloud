// This file maps the supplier configuration query service to its administration API.

package admin

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

// SupplierConfigurationGet returns one secret-safe installation-local projection.
func (c *ControllerV1) SupplierConfigurationGet(
	ctx context.Context,
	req *v1.SupplierConfigurationGetReq,
) (*v1.SupplierConfigurationGetRes, error) {
	if c == nil || c.supplierConfiguration == nil || req == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	configuration, err := c.supplierConfiguration.Get(ctx, req.ProviderCode)
	if err != nil {
		return nil, err
	}
	return &v1.SupplierConfigurationGetRes{Item: toSupplierConfigurationItem(configuration)}, nil
}

func toSupplierConfigurationItem(configuration *supplierconfig.Configuration) *v1.SupplierConfigurationItem {
	if configuration == nil {
		return nil
	}
	item := &v1.SupplierConfigurationItem{
		ProviderCode:           configuration.ProviderCode,
		CredentialStatus:       string(configuration.CredentialStatus),
		CredentialConfigured:   configuration.CredentialConfigured,
		CredentialMaskedTail:   configuration.CredentialMaskedTail,
		CredentialGeneration:   configuration.CredentialGeneration,
		CapabilityEvidenceCode: configuration.CapabilityEvidenceCode,
		AdapterEnabled:         configuration.AdapterEnabled, Version: configuration.Version,
		SubjectStatus:       string(configuration.SubjectStatus),
		SubjectBindingNo:    configuration.SubjectBindingNo,
		LastOperationKind:   string(configuration.LastOperationKind),
		LastOperationResult: string(configuration.LastOperationResult),
		LastOperationNo:     configuration.LastOperationNo,
	}
	item.LastOperationAt = unixMilliseconds(configuration.LastOperationAt)
	item.UpdatedAt = unixMilliseconds(configuration.UpdatedAt)
	return item
}

func unixMilliseconds(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	milliseconds := value.UTC().UnixMilli()
	return &milliseconds
}

func clearControllerBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
