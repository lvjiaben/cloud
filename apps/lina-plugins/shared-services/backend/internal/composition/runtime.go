// Package composition constructs shared-services runtime dependencies.
package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-shared-services/backend/internal/service/challenge"
	"lina-plugin-shared-services/backend/internal/service/inbox"
)

type Runtime struct {
	Challenge *challenge.ServiceRuntime
	Inbox     *inbox.ServiceRuntime
}

func Build() (*Runtime, error) {
	config, err := challenge.LoadConfig()
	if err != nil {
		return nil, err
	}
	challengeService, err := challenge.New(g.DB(), config)
	if err != nil {
		return nil, err
	}
	inboxService, err := inbox.New(g.DB())
	if err != nil {
		return nil, err
	}
	return &Runtime{Challenge: challengeService, Inbox: inboxService}, nil
}

func RegisterCleanupJob(ctx context.Context, registrar pluginhost.JobsRegistrar, service interface {
	CleanupExpired(context.Context, int) error
}) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || service == nil {
		return gerror.New("shared challenge cleanup job is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 5m", "shared-services-challenge-cleanup",
		"Shared Challenge Cleanup", "Removes expired human-verification challenge material after its retention window.",
		func(jobCtx context.Context) error { return service.CleanupExpired(jobCtx, 500) })
}
