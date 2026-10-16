// This file declares the read-only website provider's lifecycle impact.
package websitecap

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/pluginhost"
)

// BusinessImpact reports no transactional references: website settings are retained
// independently and Public gates their visibility on authoritative availability.
// Templates never own sales, provisioned services, tasks or historical actions.
func BusinessImpact(ctx context.Context, input pluginhost.PluginBusinessImpactInput) ([]pluginhost.PluginBusinessImpactItem, error) {
	if ctx == nil || strings.TrimSpace(input.TargetPluginID) == "" {
		return nil, gerror.New("website business impact requires context and target")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []pluginhost.PluginBusinessImpactItem{}, nil
}
