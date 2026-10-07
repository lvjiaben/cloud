package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/contenthub"
)

const (
	contentScanJobPattern     = "@every 15s"
	contentScanJobName        = "idc-core-content-scan"
	contentScanJobDisplayName = "IDC Content Scan"
	contentScanJobDescription = "Streams one queued download candidate to the installation-local ClamAV clamd and cleans expired candidates."
	contentScanJobBatchLimit  = 1
)

func RegisterContentScanJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker contenthub.ScanWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil {
		return gerror.New("content scan job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if worker == nil {
		return gerror.New("content scan worker is unavailable")
	}
	return registrar.AddWithMetadata(
		ctx,
		contentScanJobPattern,
		contentScanJobName,
		contentScanJobDisplayName,
		contentScanJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, contentScanJobBatchLimit)
			return err
		},
	)
}
