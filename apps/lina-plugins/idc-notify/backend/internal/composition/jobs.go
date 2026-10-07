package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

const notificationWorkerBatch = 40

func RegisterJobs(ctx context.Context, registrar pluginhost.JobsRegistrar, service notification.Service) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || service == nil {
		return gerror.New("notification job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if err := registrar.AddWithMetadata(ctx, "@every 5s", "idc-notify-events",
		"IDC Notification Event Materializer", "Materializes bounded core-owned notification events into durable delivery tasks.",
		func(jobCtx context.Context) error { return service.ProcessEvents(jobCtx, notificationWorkerBatch) }); err != nil {
		return err
	}
	if err := registrar.AddWithMetadata(ctx, "@every 5s", "idc-notify-delivery",
		"IDC Notification Delivery", "Claims bounded notification attempts with persistent retry fences.",
		func(jobCtx context.Context) error { return service.ProcessDue(jobCtx, notificationWorkerBatch) }); err != nil {
		return err
	}
	if err := registrar.AddWithMetadata(ctx, "@every 30s", "idc-notify-channel-recovery",
		"IDC Notification Channel Recovery", "Recovers interrupted channel credential changes without retaining plaintext.",
		func(jobCtx context.Context) error {
			return service.ProcessChannelIntents(jobCtx, notificationWorkerBatch)
		}); err != nil {
		return err
	}
	if err := registrar.AddWithMetadata(ctx, "@every 30s", "idc-notify-wecom-binding-recovery",
		"IDC WeCom Employee Binding Recovery", "Recovers interrupted employee identity secret creation, revocation, and bounded queue cancellation.",
		func(jobCtx context.Context) error {
			return service.ProcessWeComBindings(jobCtx, notificationWorkerBatch)
		}); err != nil {
		return err
	}
	return registrar.AddWithMetadata(ctx, "@every 1m", "idc-notify-query",
		"IDC Notification Result Query", "Converges externally accepted notifications using provider-authoritative query limits.",
		func(jobCtx context.Context) error { return service.ProcessQueries(jobCtx, notificationWorkerBatch) })
}
