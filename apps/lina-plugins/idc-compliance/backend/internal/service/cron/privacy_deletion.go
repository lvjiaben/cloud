// Privacy deletion cron wiring invokes bounded task generation on the local installation. It schedules candidates only and never confirms destructive deletion.

package cron

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
)

const (
	privacyDeletionPattern = "@every 1h"
	privacyDeletionName    = "idc-compliance-privacy-deletion-scan"
)

type PrivacyDeletionScanner interface {
	ScanDueDeletionTasks(context.Context, int) (int, error)
}

func RegisterPrivacyDeletion(ctx context.Context, registrar pluginhost.JobsRegistrar, worker PrivacyDeletionScanner) error {
	if ctx == nil || ctx.Err() != nil || nilDependency(registrar) || nilDependency(worker) {
		return gerror.New("privacy deletion job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, privacyDeletionPattern, privacyDeletionName,
		"IDC Privacy Deletion Scan", "Creates bounded local deletion tasks for expired sensitive originals without deleting them automatically.",
		func(jobCtx context.Context) error { _, err := worker.ScanDueDeletionTasks(jobCtx, 100); return err },
	)
}
