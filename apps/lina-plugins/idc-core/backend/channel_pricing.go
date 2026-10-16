// This file assembles stateless customer pricing owners at plugin startup.
// HTTP and channel runtimes share the authoritative database and price policy;
// no request-scoped service graph or process-global service locator is used.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/managementruntime"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func newChannelQuoteAdjustments(ctx context.Context, services capability.Services) (commerce.QuoteAdjustmentEvaluator, error) {
	if ctx == nil || ctx.Err() != nil || services == nil {
		return nil, gerror.New("customer pricing startup context is unavailable")
	}
	pricing, err := salesmanagement.NewPricing(g.DB())
	if err != nil {
		return nil, err
	}
	marketingOwner, err := marketing.New(g.DB(), marketing.Config{})
	if err != nil {
		return nil, err
	}
	marketingEvaluator, err := commerce.NewMarketingQuoteAdjustmentEvaluator(marketingOwner)
	if err != nil {
		return nil, err
	}
	return commerce.NewAgentQuoteAdjustmentEvaluator(pricing, marketingEvaluator)
}

// newManagementRuntime assembles pricing once when a consumer binds its runtime.
func newManagementRuntime(ctx context.Context, services capability.Services) (managementcap.Runtime, error) {
	adjustments, err := newChannelQuoteAdjustments(ctx, services)
	if err != nil {
		return managementcap.Runtime{}, err
	}
	return managementruntime.New(ctx, services, adjustments)
}
