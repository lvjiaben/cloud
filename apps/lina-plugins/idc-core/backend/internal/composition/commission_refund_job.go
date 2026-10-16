package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/commission"
)

const (
	commissionRefundJobPattern     = "@every 10s"
	commissionRefundJobName        = "idc-core-commission-refund-reversal"
	commissionRefundJobDisplayName = "IDC Commission Processing"
	commissionRefundJobDescription = "Accrues paid orders and renewals, reverses refunds, and matures due commissions."
	commissionRefundJobBatchLimit  = 20
)

func RegisterCommissionRefundJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker commission.Worker,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("commission refund job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, commissionRefundJobPattern, commissionRefundJobName,
		commissionRefundJobDisplayName, commissionRefundJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, commissionRefundJobBatchLimit)
			return err
		},
	)
}
