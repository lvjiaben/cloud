package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/customerassistance"
)

func RegisterCustomerAssistanceExpiryJob(ctx context.Context, registrar pluginhost.JobsRegistrar, service customerassistance.ExpiryWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || service == nil {
		return gerror.New("customer assistance expiry job requires registrar and service")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 1m", "idc-core-customer-assistance-expiry",
		"IDC Customer Assistance Expiry", "Closes expired read-only customer assistance sessions and records immutable expiry events.",
		func(jobCtx context.Context) error {
			_, err := service.Expire(jobCtx, 100)
			return err
		})
}
