package serveroperation

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type regressionCapabilityDirectory map[string][]serveroperationresourcecap.Action

func (d regressionCapabilityDirectory) ListCapabilities(_ context.Context,
	routes []serveroperationresourcecap.CapabilityRoute,
) ([]serveroperationresourcecap.CapabilityProjection, error) {
	result := make([]serveroperationresourcecap.CapabilityProjection, len(routes))
	for index := range routes {
		result[index] = serveroperationresourcecap.CapabilityProjection{ServiceNo: routes[index].ServiceNo,
			Actions:    append([]serveroperationresourcecap.Action(nil), d[routes[index].ServiceNo]...),
			ObservedAt: time.Now().UTC()}
	}
	return result, nil
}

func TestThreeDimensionalStateAndBareMetalCapabilityProjectionFailClosed(t *testing.T) {
	capabilities := regressionCapabilityDirectory{
		"safe":            {serveroperationresourcecap.ActionStop},
		"bad-entitlement": {serveroperationresourcecap.ActionStop},
		"bad-runtime":     {serveroperationresourcecap.ActionStop},
		"provider-busy":   {serveroperationresourcecap.ActionStop},
		"operation-busy":  {serveroperationresourcecap.ActionStop},
		"bare-metal":      {serveroperationresourcecap.ActionStop},
	}
	directory := &directoryImpl{capabilities: capabilities}
	now := time.Now().UTC()
	rows := []serverDirectoryRow{
		{ServiceNo: "safe", ChannelNo: "channel-safe", PoolNo: "pool-safe", EntitlementStatus: "active", RuntimeStatus: "running", ProviderTaskStatus: "none", UpdatedAt: now},
		{ServiceNo: "bad-entitlement", ChannelNo: "channel-safe", PoolNo: "pool-safe", EntitlementStatus: "terminated", RuntimeStatus: "running", ProviderTaskStatus: "none", UpdatedAt: now},
		{ServiceNo: "bad-runtime", ChannelNo: "channel-safe", PoolNo: "pool-safe", EntitlementStatus: "active", RuntimeStatus: "unknown", ProviderTaskStatus: "none", UpdatedAt: now},
		{ServiceNo: "provider-busy", ChannelNo: "channel-safe", PoolNo: "pool-safe", EntitlementStatus: "active", RuntimeStatus: "running", ProviderTaskStatus: "running", UpdatedAt: now},
		{ServiceNo: "operation-busy", EntitlementStatus: "active", RuntimeStatus: "running", ProviderTaskStatus: "none",
			ChannelNo: "channel-safe", PoolNo: "pool-safe", OperationNo: "operation-1", OperationAction: "reinstall", OperationStatus: "running", OperationCreatedAt: &now, UpdatedAt: now},
		{ServiceNo: "bare-metal", ChannelNo: "channel-safe", PoolNo: "pool-safe", ProductType: "bare_metal", EntitlementStatus: "active", RuntimeStatus: "running", ProviderTaskStatus: "none", UpdatedAt: now},
	}
	items, err := directory.projectServers(context.Background(), rows)
	if err != nil || len(items) != len(rows) {
		t.Fatalf("project regression state matrix: items=%+v err=%v", items, err)
	}
	if len(items[0].AvailableActions) != 1 || items[0].AvailableActions[0] != serveroperationresourcecap.ActionStop {
		t.Fatalf("safe service actions=%v", items[0].AvailableActions)
	}
	for index := 1; index <= 4; index++ {
		if len(items[index].AvailableActions) != 0 {
			t.Fatalf("unsafe three-dimensional state %s exposed actions=%v", items[index].ServiceNo, items[index].AvailableActions)
		}
	}
	if len(items[5].AvailableActions) != 1 || items[5].AvailableActions[0] != serveroperationresourcecap.ActionStop {
		t.Fatalf("bare-metal projection invented or removed capabilities=%v", items[5].AvailableActions)
	}
}

func TestServerProjectionKeepsHistoricalUnboundRowsReadOnly(t *testing.T) {
	directory := &directoryImpl{capabilities: regressionCapabilityDirectory{
		"bound": {serveroperationresourcecap.ActionStop},
	}}
	now := time.Now().UTC()
	items, err := directory.projectServers(context.Background(), []serverDirectoryRow{
		{ServiceNo: "unbound", EntitlementStatus: "active", RuntimeStatus: "running", ProviderTaskStatus: "none", UpdatedAt: now},
		{ServiceNo: "bound", ChannelNo: "channel-safe", PoolNo: "pool-safe", EntitlementStatus: "active", RuntimeStatus: "running", ProviderTaskStatus: "none", UpdatedAt: now},
	})
	if err != nil || len(items) != 2 {
		t.Fatalf("project mixed route rows: items=%+v err=%v", items, err)
	}
	if len(items[0].AvailableActions) != 0 {
		t.Fatalf("historical unbound row exposed actions=%v", items[0].AvailableActions)
	}
	if len(items[1].AvailableActions) != 1 || items[1].AvailableActions[0] != serveroperationresourcecap.ActionStop {
		t.Fatalf("bound row actions=%v", items[1].AvailableActions)
	}
}
