// This file supplies the lifecycle impact of the descriptor-only payment-method
// contract. Transaction execution and retained evidence belong to the builtin owner.

package paymentcap

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
)

// BusinessImpact reports no transactional references to a v1 payment-method
// guest, whose only allowed method is capabilities.describe. Existing channels,
// payments, callbacks, refunds and reconciliation continue through builtin
// adapters and retained owner records; they do not invoke the managed guest.
// The host still checks every other declared owner, authorization, dependencies
// and stopped admission. Invalid context or target fails closed at that boundary.
func BusinessImpact(ctx context.Context, input pluginhost.PluginBusinessImpactInput) ([]pluginhost.PluginBusinessImpactItem, error) {
	if ctx == nil || strings.TrimSpace(input.TargetPluginID) == "" {
		return nil, gerror.New("payment method business impact requires context and target")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []pluginhost.PluginBusinessImpactItem{}, nil
}
