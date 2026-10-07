package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

const (
	renewalProviderJobPattern     = "@every 5s"
	renewalProviderJobName        = "idc-core-renewal-provider"
	renewalProviderJobDisplayName = "IDC Renewal Provider Convergence"
	renewalProviderJobDescription = "Creates and reconciles bounded resource-owner tasks for settled renewals."
	renewalProviderBatchLimit     = 16
)

func RegisterRenewalProviderJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	coordinator renewal.ProviderService,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || coordinator == nil {
		return gerror.New("renewal Provider job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, renewalProviderJobPattern, renewalProviderJobName,
		renewalProviderJobDisplayName, renewalProviderJobDescription,
		func(jobCtx context.Context) error {
			_, err := coordinator.RunProviderOperations(jobCtx, renewalProviderBatchLimit)
			return err
		},
	)
}
