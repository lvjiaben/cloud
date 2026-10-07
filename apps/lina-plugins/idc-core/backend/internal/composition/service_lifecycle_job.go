// This file registers primary-node lifecycle scheduling and processing jobs.

package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/lifecycle"
)

const (
	serviceLifecycleJobPattern           = "@every 30s"
	serviceLifecycleJobName              = "idc-core-service-lifecycle"
	serviceLifecycleJobDisplayName       = "IDC Service Lifecycle Scheduling"
	serviceLifecycleJobDescription       = "Materializes one bounded task-only service lifecycle batch."
	serviceLifecycleBatchLimit           = 32
	serviceLifecycleProcessorJobName     = "idc-core-service-lifecycle-processor"
	serviceLifecycleProcessorDisplayName = "IDC Service Lifecycle Processing"
	serviceLifecycleProcessorDescription = "Processes one bounded service lifecycle task batch."
)

// RegisterServiceLifecycleJob installs the bounded scheduler only on the
// primary node. The worker materializes facts; it never dispatches Providers.
func RegisterServiceLifecycleJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker lifecycle.Scheduler,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("service lifecycle job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, serviceLifecycleJobPattern, serviceLifecycleJobName,
		serviceLifecycleJobDisplayName, serviceLifecycleJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, serviceLifecycleBatchLimit)
			return err
		},
	)
}

// RegisterServiceLifecycleProcessorJob installs task convergence after the
// independent scheduling pass on the primary node.
func RegisterServiceLifecycleProcessorJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker lifecycle.Processor,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("service lifecycle processor job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, serviceLifecycleJobPattern, serviceLifecycleProcessorJobName,
		serviceLifecycleProcessorDisplayName, serviceLifecycleProcessorDescription,
		func(jobCtx context.Context) error {
			_, err := worker.RunTasks(jobCtx, serviceLifecycleBatchLimit)
			return err
		},
	)
}
