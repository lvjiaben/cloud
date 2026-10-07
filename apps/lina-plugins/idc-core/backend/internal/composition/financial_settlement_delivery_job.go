// This file registers financially settled order materialization before any
// optional Provider/renewal capability is resolved.
package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
)

const (
	financialSettlementDeliveryJobPattern     = "@every 10s"
	financialSettlementDeliveryJobName        = "idc-core-financial-settlement-delivery"
	financialSettlementDeliveryJobDisplayName = "IDC Financial Settlement Delivery"
	financialSettlementDeliveryJobDescription = "Materializes financially settled orders into local service and business-operation intents without invoking Providers."
	financialSettlementDeliveryBatchLimit     = 20
)

// RegisterFinancialSettlementDeliveryJob installs the core-owned worker on
// the primary node independently from optional resource capability plugins.
func RegisterFinancialSettlementDeliveryJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker fulfillment.Worker,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("financial settlement delivery job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, financialSettlementDeliveryJobPattern, financialSettlementDeliveryJobName,
		financialSettlementDeliveryJobDisplayName, financialSettlementDeliveryJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, financialSettlementDeliveryBatchLimit)
			return err
		},
	)
}
