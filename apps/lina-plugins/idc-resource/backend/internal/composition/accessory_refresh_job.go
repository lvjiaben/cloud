package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
)

const (
	accessoryRefreshJobPattern     = "@every 30s"
	accessoryRefreshJobName        = "idc-resource-accessory-refresh"
	accessoryRefreshJobDisplayName = "IDC Resource Accessory Refresh"
	accessoryRefreshJobDescription = "Refreshes bounded field-whitelisted server accessory directories."
	accessoryRefreshBatchLimit     = 8
)

func RegisterAccessoryRefreshJob(ctx context.Context, registrar pluginhost.JobsRegistrar,
	worker accessorydirectory.Worker) error {
	if ctx == nil || ctx.Err() != nil || isNilInterface(registrar) {
		return gerror.New("resource accessory refresh job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if isNilInterface(worker) {
		return gerror.New("resource accessory refresh worker is unavailable")
	}
	return registrar.AddWithMetadata(ctx, accessoryRefreshJobPattern, accessoryRefreshJobName,
		accessoryRefreshJobDisplayName, accessoryRefreshJobDescription, func(jobCtx context.Context) error {
			for index := 0; index < accessoryRefreshBatchLimit; index++ {
				worked, err := worker.RunOne(jobCtx)
				if err != nil || !worked {
					return err
				}
			}
			return nil
		})
}
