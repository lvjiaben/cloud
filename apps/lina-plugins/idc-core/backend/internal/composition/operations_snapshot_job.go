package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/operations"
)

const (
	operationsSnapshotJobPattern     = "@every 30s"
	operationsSnapshotJobName        = "idc-core-operations-snapshot"
	operationsSnapshotJobDisplayName = "IDC Operations Snapshot"
	operationsSnapshotJobDescription = "Refreshes the bounded operations read model from PostgreSQL authority."
)

// RegisterOperationsSnapshotJob keeps refresh scheduling in the plugin's
// composition layer. The worker remains a small service boundary so a future
// persisted snapshot implementation can replace it without changing routing.
func RegisterOperationsSnapshotJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker operations.Service) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil {
		return gerror.New("operations snapshot job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if worker == nil {
		return gerror.New("operations snapshot worker is unavailable")
	}
	return registrar.AddWithMetadata(ctx, operationsSnapshotJobPattern, operationsSnapshotJobName, operationsSnapshotJobDisplayName, operationsSnapshotJobDescription, func(jobCtx context.Context) error {
		_, err := worker.RefreshInternal(jobCtx)
		return err
	})
}
