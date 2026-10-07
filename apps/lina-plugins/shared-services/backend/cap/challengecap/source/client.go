// Package source constructs the caller-bound source-plugin challenge client.
package source

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-shared-services/backend/cap/challengecap"
	"lina-plugin-shared-services/backend/internal/service/challenge"
	"lina-plugin-shared-services/backend/internal/service/sharedsource"
)

func New(ctx context.Context, services capability.Services, callerPluginID string) (challengecap.Service, error) {
	caller, err := sharedsource.Authorize(ctx, services, callerPluginID)
	if err != nil {
		return nil, err
	}
	config, err := challenge.LoadConfig()
	if err != nil {
		return nil, err
	}
	service, err := challenge.New(g.DB(), config)
	if err != nil {
		return nil, err
	}
	return challenge.Bind(service, caller)
}
