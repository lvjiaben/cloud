// Package source constructs the caller-bound source-plugin inbox client.
package source

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-shared-services/backend/cap/inboxcap"
	"lina-plugin-shared-services/backend/internal/service/inbox"
	"lina-plugin-shared-services/backend/internal/service/sharedsource"
)

func New(ctx context.Context, services capability.Services, callerPluginID string) (inboxcap.Service, error) {
	caller, err := sharedsource.Authorize(ctx, services, callerPluginID)
	if err != nil {
		return nil, err
	}
	service, err := inbox.New(g.DB())
	if err != nil {
		return nil, err
	}
	return inbox.Bind(service, caller)
}
