// Package source constructs a caller-bound source-plugin security-code client.
package source

import (
	"context"
	"regexp"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-notify/backend/cap/securitycodecap"
	"lina-plugin-idc-notify/backend/internal/composition"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

var pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,127}$`)

func New(ctx context.Context, services capability.Services, callerPluginID string) (securitycodecap.Service, error) {
	callerPluginID = strings.TrimSpace(callerPluginID)
	if !validSourceContext(ctx, services, callerPluginID) {
		return nil, bizerr.NewCode(notification.CodeNotAccessible)
	}
	current, err := services.Plugins().Registry().Current(ctx)
	if err != nil || current == nil || string(current.ID) != callerPluginID || !current.Installed || !current.Enabled {
		return nil, bizerr.NewCode(notification.CodeNotAccessible)
	}
	enabled, err := services.Plugins().State().IsEnabledAuthoritative(ctx, plugincap.PluginID(securitycodecap.OwnerPluginID))
	if err != nil || !enabled {
		return nil, bizerr.NewCode(notification.CodeNotAccessible)
	}
	return composition.BuildSecurityCodeService(ctx, services, callerPluginID)
}

func validSourceContext(ctx context.Context, services capability.Services, callerPluginID string) bool {
	return ctx != nil && ctx.Err() == nil && services != nil && services.Plugins() != nil &&
		services.Plugins().Registry() != nil && services.Plugins().State() != nil &&
		pluginIDPattern.MatchString(callerPluginID) && callerPluginID != securitycodecap.OwnerPluginID
}
