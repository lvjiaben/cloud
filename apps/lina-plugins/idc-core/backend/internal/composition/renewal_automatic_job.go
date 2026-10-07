package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

const (
	renewalAutomaticJobPattern     = "@every 30s"
	renewalAutomaticJobName        = "idc-core-renewal-automatic"
	renewalAutomaticJobDisplayName = "IDC Automatic Renewals"
	renewalAutomaticJobDescription = "Processes one bounded same-currency automatic renewal batch."
	renewalAutomaticBatchLimit     = 32
)

func RegisterAutomaticRenewalJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker renewal.AutomaticService,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("automatic renewal job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, renewalAutomaticJobPattern, renewalAutomaticJobName,
		renewalAutomaticJobDisplayName, renewalAutomaticJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.RunAutomaticRenewals(jobCtx, renewalAutomaticBatchLimit)
			return err
		},
	)
}
