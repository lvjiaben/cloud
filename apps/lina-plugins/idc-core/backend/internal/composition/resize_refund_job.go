// This file registers the bounded required-resize refund bridge.

package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const (
	resizeRefundJobPattern     = "@every 10s"
	resizeRefundJobName        = "idc-core-resize-refunds"
	resizeRefundJobDisplayName = "IDC Resize Refund Bridge"
	resizeRefundJobDescription = "Moves paid resize failures into the unified durable refund workflow."
	resizeRefundJobBatchLimit  = 20
)

// RegisterResizeRefundJob installs the bridge only on the primary node.
func RegisterResizeRefundJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker billing.ResizeRefundWorker,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("resize refund job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, resizeRefundJobPattern, resizeRefundJobName,
		resizeRefundJobDisplayName, resizeRefundJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, resizeRefundJobBatchLimit)
			return err
		},
	)
}
