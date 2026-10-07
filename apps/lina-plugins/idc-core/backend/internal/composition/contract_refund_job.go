// This file registers the bounded required-contract refund bridge.

package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const (
	contractRefundJobPattern     = "@every 10s"
	contractRefundJobName        = "idc-core-contract-refunds"
	contractRefundJobDisplayName = "IDC Contract Refund Bridge"
	contractRefundJobDescription = "Moves required-contract and paid resize failures into the unified durable refund workflow."
	contractRefundJobBatchLimit  = 20
)

// RegisterContractRefundJob installs the bridge only on the primary node.
func RegisterContractRefundJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker billing.ContractRefundWorker,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("contract refund job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, contractRefundJobPattern, contractRefundJobName,
		contractRefundJobDisplayName, contractRefundJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, contractRefundJobBatchLimit)
			return err
		},
	)
}
