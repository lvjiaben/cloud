// This file registers the bounded core-owned refund posting worker.

package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const (
	refundLocalJobPattern     = "@every 10s"
	refundLocalJobName        = "idc-core-local-refunds"
	refundLocalJobDisplayName = "IDC Local Refund Posting"
	refundLocalJobDescription = "Posts bounded same-currency balance and credit refund facts without external channel calls."
	refundLocalJobBatchLimit  = 20
)

// RegisterLocalRefundJob installs the core-owned refund worker on the primary node.
func RegisterLocalRefundJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker billing.LocalRefundWorker,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("local refund job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, refundLocalJobPattern, refundLocalJobName,
		refundLocalJobDisplayName, refundLocalJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, refundLocalJobBatchLimit)
			return err
		},
	)
}
