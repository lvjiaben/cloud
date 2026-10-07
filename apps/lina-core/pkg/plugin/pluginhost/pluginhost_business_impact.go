// This file defines the host-governed business-impact contract used by stable
// source-plugin owners to protect managed plugin disable and uninstall actions.

package pluginhost

import "context"

// BusinessImpactCategory identifies one bounded class of references that can
// keep a managed plugin necessary for existing business.
type BusinessImpactCategory string

// Supported plugin business-impact categories.
const (
	BusinessImpactSellingProductVersions   BusinessImpactCategory = "selling_product_versions"
	BusinessImpactInFlightTasks            BusinessImpactCategory = "in_flight_tasks"
	BusinessImpactActiveServices           BusinessImpactCategory = "active_services"
	BusinessImpactPendingHistoricalActions BusinessImpactCategory = "pending_historical_actions"
)

// PluginBusinessImpactInput identifies the managed plugin being inspected.
type PluginBusinessImpactInput struct {
	TargetPluginID string
}

// PluginBusinessImpactItem is one owner-provided aggregate. TotalCount keeps
// historical visibility while BlockingCount includes only objects that have
// not completed migration or explicit manual takeover.
type PluginBusinessImpactItem struct {
	Category      BusinessImpactCategory
	TotalCount    int64
	BlockingCount int64
}

// SourcePluginBusinessImpactHandler returns bounded aggregate impact data from
// one stable business owner. It must not return object details or secrets.
type SourcePluginBusinessImpactHandler func(
	ctx context.Context,
	input PluginBusinessImpactInput,
) ([]PluginBusinessImpactItem, error)

// IsSupportedBusinessImpactCategory reports whether a business owner returned
// one of the host-published lifecycle categories.
func IsSupportedBusinessImpactCategory(category BusinessImpactCategory) bool {
	switch category {
	case BusinessImpactSellingProductVersions,
		BusinessImpactInFlightTasks,
		BusinessImpactActiveServices,
		BusinessImpactPendingHistoricalActions:
		return true
	default:
		return false
	}
}

// ListSourcePluginBusinessImpactProviders returns only source plugins that
// explicitly registered the bounded impact provider contract.
func ListSourcePluginBusinessImpactProviders() []SourcePluginDefinition {
	plugins := ListSourcePlugins()
	items := make([]SourcePluginDefinition, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin == nil || plugin.ID() == "" || plugin.GetBusinessImpactHandler() == nil {
			continue
		}
		items = append(items, plugin)
	}
	return items
}
