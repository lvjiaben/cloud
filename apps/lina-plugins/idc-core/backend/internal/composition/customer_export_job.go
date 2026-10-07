package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/customerexport"
)

func RegisterCustomerExportJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker customerexport.Worker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("customer export job requires registrar and worker")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 15s", "idc-core-customer-export",
		"IDC Customer Data Export", "Generates private customer-owned exports and deletes expired artifacts.",
		func(jobCtx context.Context) error {
			if _, err := worker.Run(jobCtx, 10); err != nil {
				return err
			}
			_, err := worker.Cleanup(jobCtx, 20)
			return err
		})
}
