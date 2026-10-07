// This file binds automatic Provider capacity pages to the host-owned job lifecycle.
package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
	"lina-plugin-idc-resource/backend/internal/service/principalprovision"
	"lina-plugin-idc-resource/backend/internal/service/providercapacity"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

const (
	providerCapacitySyncJobPattern     = "@every 30s"
	providerCapacitySyncJobName        = "idc-resource-provider-capacity-refresh"
	providerCapacitySyncJobDisplayName = "IDC Resource Provider Capacity Refresh"
	providerCapacitySyncJobDescription = "Advances bounded Provider capacity observations one page per claimed operation."
	providerCapacitySyncBatchLimit     = 8
	interfaceProjectionJobPattern      = "@every 5s"
	interfaceProjectionJobName         = "idc-resource-interface-projection"
	interfaceProjectionJobDisplayName  = "IDC Resource Interface Projection"
	interfaceProjectionJobDescription  = "Delivers bounded secret-free resource interface projections."
	interfaceProjectionBatchLimit      = 16
)

func RegisterResourceJobs(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker providercapacity.Worker,
	accessoryWorker accessorydirectory.Worker,
	taskExecutor *resourcetask.Executor,
	principalWorker *principalprovision.Worker,
	projectionWorker *interfaceprojection.Worker,
) error {
	if err := RegisterInterfaceProjectionJob(ctx, registrar, projectionWorker); err != nil {
		return err
	}
	if err := RegisterProviderCapacitySyncJob(ctx, registrar, worker); err != nil {
		return err
	}
	if err := RegisterAccessoryRefreshJob(ctx, registrar, accessoryWorker); err != nil {
		return err
	}
	if err := RegisterPrincipalProvisionJob(ctx, registrar, principalWorker); err != nil {
		return err
	}
	return RegisterResourceTaskJob(ctx, registrar, taskExecutor)
}

func RegisterInterfaceProjectionJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker *interfaceprojection.Worker,
) error {
	if ctx == nil || ctx.Err() != nil || isNilInterface(registrar) {
		return gerror.New("resource interface projection job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if worker == nil {
		return gerror.New("resource interface projection worker is unavailable")
	}
	return registrar.AddWithMetadata(
		ctx, interfaceProjectionJobPattern, interfaceProjectionJobName,
		interfaceProjectionJobDisplayName, interfaceProjectionJobDescription,
		func(jobCtx context.Context) error {
			for index := 0; index < interfaceProjectionBatchLimit; index++ {
				worked, runErr := worker.RunOne(jobCtx)
				if runErr != nil || !worked {
					return runErr
				}
			}
			return nil
		},
	)
}

func RegisterProviderCapacitySyncJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker providercapacity.Worker,
) error {
	if ctx == nil || ctx.Err() != nil || isNilInterface(registrar) {
		return gerror.New("resource provider capacity refresh job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if isNilInterface(worker) {
		return gerror.New("resource provider capacity refresh worker is unavailable")
	}
	return registrar.AddWithMetadata(
		ctx, providerCapacitySyncJobPattern, providerCapacitySyncJobName,
		providerCapacitySyncJobDisplayName, providerCapacitySyncJobDescription,
		func(jobCtx context.Context) error {
			for index := 0; index < providerCapacitySyncBatchLimit; index++ {
				worked, runErr := worker.RunOne(jobCtx)
				if runErr != nil || !worked {
					return runErr
				}
			}
			return nil
		},
	)
}
