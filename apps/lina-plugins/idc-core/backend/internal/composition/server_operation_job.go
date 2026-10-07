package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

const (
	serverOperationJobPattern     = "@every 10s"
	serverOperationJobName        = "idc-core-server-operation"
	serverOperationJobDisplayName = "IDC Server Operations"
	serverOperationJobDescription = "Converges core server actions with resource-owned Provider tasks."
	serverOperationBatchLimit     = 20
)

func RegisterServerOperationJob(ctx context.Context, registrar pluginhost.JobsRegistrar,
	worker serveroperation.Worker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("server operation job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, serverOperationJobPattern, serverOperationJobName,
		serverOperationJobDisplayName, serverOperationJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, serverOperationBatchLimit)
			return err
		})
}
