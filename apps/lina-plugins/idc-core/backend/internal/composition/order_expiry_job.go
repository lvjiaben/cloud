// This file binds bounded unpaid-order expiry to the host-owned job lifecycle.
package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

const (
	orderExpiryJobPattern     = "@every 30s"
	orderExpiryJobName        = "idc-core-order-expiry"
	orderExpiryJobDisplayName = "IDC Unpaid Order Expiry"
	orderExpiryJobDescription = "Cancels a bounded batch of locally expired unpaid orders without contacting payment channels or Providers."
	orderExpiryBatchLimit     = 32
)

// RegisterOrderExpiryJob registers one primary-node worker. The commerce
// service owns row claiming and one transaction per order.
func RegisterOrderExpiryJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	lifecycle commerce.OrderLifecycleService,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil {
		return gerror.New("order expiry job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if lifecycle == nil {
		return gerror.New("order expiry lifecycle service is unavailable")
	}
	return registrar.AddWithMetadata(
		ctx,
		orderExpiryJobPattern,
		orderExpiryJobName,
		orderExpiryJobDisplayName,
		orderExpiryJobDescription,
		func(jobCtx context.Context) error {
			_, err := lifecycle.ExpireDueOrders(jobCtx, orderExpiryBatchLimit)
			return err
		},
	)
}
