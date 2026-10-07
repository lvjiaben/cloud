package composition

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

const (
	resourceTaskJobPattern     = "@every 5s"
	resourceTaskJobName        = "idc-resource-provider-tasks"
	resourceTaskJobDisplayName = "IDC Resource Provider Tasks"
	resourceTaskJobDescription = "Submits and observes bounded idempotent Provider operation batches."
	resourceTaskBatchLimit     = 8
	resourceTaskLeaseDuration  = 2 * time.Minute
	resourceTaskWorkerID       = "idc-resource-primary"
)

func RegisterResourceTaskJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	executor *resourcetask.Executor,
) error {
	if ctx == nil || ctx.Err() != nil || isNilInterface(registrar) || executor == nil {
		return gerror.New("resource Provider task job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, resourceTaskJobPattern, resourceTaskJobName,
		resourceTaskJobDisplayName, resourceTaskJobDescription,
		func(jobCtx context.Context) error {
			if _, err := executor.RunWrites(jobCtx, resourcetask.ClaimInput{
				WorkerID: resourceTaskWorkerID, Limit: resourceTaskBatchLimit,
				LeaseDuration: resourceTaskLeaseDuration,
			}); err != nil {
				return err
			}
			if _, err := executor.RunObservations(jobCtx, resourcetask.ObservationClaimInput{
				WorkerID: resourceTaskWorkerID, Limit: resourceTaskBatchLimit,
				LeaseDuration: resourceTaskLeaseDuration,
			}); err != nil {
				return err
			}
			_, err := executor.RunResourceObservations(
				jobCtx,
				resourcetask.ObservationClaimInput{
					WorkerID: resourceTaskWorkerID, Limit: resourceTaskBatchLimit,
					LeaseDuration: resourceTaskLeaseDuration,
				},
			)
			return err
		},
	)
}
