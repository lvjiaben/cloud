// This file consumes fresh recovery credentials and delegates safe mutation replay.
package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

// ProviderConfigurationRecoveryCreate clears request-held credential references
// before resuming the single server-tracked updating mutation.
func (c *ControllerV1) ProviderConfigurationRecoveryCreate(
	ctx context.Context,
	req *v1.ProviderConfigurationRecoveryCreateReq,
) (*v1.ProviderConfigurationRecoveryCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(providerconfig.CodeInvalid)
	}
	accessKeyID := []byte(req.AccessKeyID)
	accessKeySecret := []byte(req.AccessKeySecret)
	req.AccessKeyID = ""
	req.AccessKeySecret = ""
	defer clearCredentialBytes(accessKeyID)
	defer clearCredentialBytes(accessKeySecret)
	recovery, err := c.providerConfiguration.Recover(ctx, providerconfig.RecoveryInput{
		AccessKeyID: accessKeyID, AccessKeySecret: accessKeySecret,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ProviderConfigurationRecoveryCreateRes{
		Item:                   toProviderConfigurationItem(recovery.Configuration),
		ReusedExistingMutation: recovery.ReusedExistingMutation,
	}, nil
}
