// This file consumes one credential request and delegates the fail-closed replacement service.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

// ProviderConfigurationUpdate clears request-held credential references before delegating.
func (c *ControllerV1) ProviderConfigurationUpdate(
	ctx context.Context,
	req *v1.ProviderConfigurationUpdateReq,
) (*v1.ProviderConfigurationUpdateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(providerconfig.CodeInvalid)
	}
	accessKeyID := []byte(req.AccessKeyID)
	accessKeySecret := []byte(req.AccessKeySecret)
	req.AccessKeyID = ""
	req.AccessKeySecret = ""
	defer clearCredentialBytes(accessKeyID)
	defer clearCredentialBytes(accessKeySecret)
	configuration, err := c.providerConfiguration.Save(ctx, providerconfig.SaveInput{
		SceneID: req.SceneID, AccessKeyID: accessKeyID, AccessKeySecret: accessKeySecret,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ProviderConfigurationUpdateRes{Item: toProviderConfigurationItem(configuration)}, nil
}

// clearCredentialBytes removes the controller-owned mutable credential copy.
func clearCredentialBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
