// Package sharedsource validates the source-plugin identity bound by host services.
package sharedsource

import (
	"context"
	"regexp"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-shared-services/backend/internal/service/sharedcode"
)

const OwnerPluginID = "shared-services"

var pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,127}$`)

func Authorize(ctx context.Context, services capability.Services, claimedCaller string) (string, error) {
	if !dependenciesAvailable(ctx, services) || !validClaim(claimedCaller) {
		return "", bizerr.NewCode(sharedcode.CodeSourceNotAuthorized)
	}
	if !currentCaller(ctx, services, claimedCaller) || !ownerEnabled(ctx, services) {
		return "", bizerr.NewCode(sharedcode.CodeSourceNotAuthorized)
	}
	return claimedCaller, nil
}

func dependenciesAvailable(ctx context.Context, services capability.Services) bool {
	return ctx != nil && ctx.Err() == nil && services != nil && services.Plugins() != nil &&
		services.Plugins().Registry() != nil && services.Plugins().State() != nil
}

func validClaim(claimedCaller string) bool {
	return claimedCaller == strings.TrimSpace(claimedCaller) && pluginIDPattern.MatchString(claimedCaller) &&
		claimedCaller != OwnerPluginID
}

func currentCaller(ctx context.Context, services capability.Services, claimedCaller string) bool {
	current, err := services.Plugins().Registry().Current(ctx)
	return err == nil && current != nil && string(current.ID) == claimedCaller && current.Installed && current.Enabled
}

func ownerEnabled(ctx context.Context, services capability.Services) bool {
	enabled, err := services.Plugins().State().IsEnabledAuthoritative(ctx, plugincap.PluginID(OwnerPluginID))
	return err == nil && enabled
}
