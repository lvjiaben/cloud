package sourceauth

import (
	"context"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-staff-access/backend/cap/staffaccesscap"
)

var pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,127}$`)

func Authorize(ctx context.Context, services capability.Services, claimedCaller string) (string, error) {
	if err := validateRequest(ctx, services, claimedCaller); err != nil {
		return "", err
	}
	current, err := services.Plugins().Registry().Current(ctx)
	if err != nil || current == nil || string(current.ID) != claimedCaller || !current.Installed || !current.Enabled {
		return "", gerror.New("staff access source caller is not authorized")
	}
	enabled, err := services.Plugins().State().IsEnabledAuthoritative(ctx, plugincap.PluginID(staffaccesscap.OwnerPluginID))
	if err != nil || !enabled {
		return "", gerror.New("staff access source plugin is unavailable")
	}
	return claimedCaller, nil
}

func validateRequest(ctx context.Context, services capability.Services, claimedCaller string) error {
	if ctx == nil || ctx.Err() != nil || services == nil || services.Plugins() == nil {
		return gerror.New("staff access source caller is not authorized")
	}
	plugins := services.Plugins()
	if plugins.Registry() == nil || plugins.State() == nil {
		return gerror.New("staff access source caller is not authorized")
	}
	if claimedCaller != strings.TrimSpace(claimedCaller) ||
		!pluginIDPattern.MatchString(claimedCaller) || claimedCaller == staffaccesscap.OwnerPluginID {
		return gerror.New("staff access source caller is not authorized")
	}
	return nil
}
