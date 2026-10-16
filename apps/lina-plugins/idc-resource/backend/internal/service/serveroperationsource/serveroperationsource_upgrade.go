// This file compiles catalog-owned frozen configurations into a restricted
// resize operation. Ownership and capability checks precede financial quoting.

package serveroperationsource

import (
	"context"
	"encoding/json"
	"reflect"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverconfiguration"
)

var _ serveroperationresourcecap.UpgradeConfigurationCompiler = (*Service)(nil)

// CompileUpgrade resolves an owned entitlement route and accepts only sizing
// increases that the selected Provider explicitly supports. Other components
// must remain identical; this never treats a reinstall as a resize.
func (s *Service) CompileUpgrade(ctx context.Context, request serveroperationresourcecap.UpgradeConfigurationRequest) (*serveroperationresourcecap.UpgradeConfiguration, error) {
	route, err := s.ResolveEntitlementRoute(ctx, serveroperationresourcecap.EntitlementRouteRequest{ServiceNo: request.ServiceNo, CustomerNo: request.CustomerNo})
	if err != nil {
		return nil, err
	}
	current, err := serverconfiguration.Compile(request.ProductConfiguration, request.CurrentConfiguration)
	if err != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	target, err := serverconfiguration.Compile(request.ProductConfiguration, request.TargetConfiguration)
	if err != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	parameters, err := compileUpgradeParameters(current, target)
	if err != nil {
		return nil, err
	}
	if err = s.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{ServiceNo: route.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo}, serveroperationresourcecap.ActionResize, parameters); err != nil {
		return nil, err
	}
	return &serveroperationresourcecap.UpgradeConfiguration{Route: *route, Parameters: parameters}, nil
}

// resizeSizingParameters intentionally omits zero fields. An unchanged value
// is not sent back to the Provider, and arbitrary configuration keys cannot
// enter this command through generic JSON passthrough.
type resizeSizingParameters struct {
	CPU        int64 `json:"cpu,omitempty"`
	Memory     int64 `json:"memory,omitempty"`
	SystemDisk int64 `json:"system_disk_size,omitempty"`
	InBW       int64 `json:"in_bw,omitempty"`
	OutBW      int64 `json:"out_bw,omitempty"`
}

func compileUpgradeParameters(current, target []resourcecap.ServerProvisionComponent) (json.RawMessage, error) {
	if len(current) != len(target) {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var parameters resizeSizingParameters
	for index, before := range current {
		after := target[index]
		if before.Type != after.Type || before.SelectionQuantity != after.SelectionQuantity {
			return nil, serveroperationresourcecap.ErrCapabilityUnavailable
		}
		if reflect.DeepEqual(before, after) {
			continue
		}
		switch before.Type {
		case resourcecap.ServerProvisionComponentCPU:
			if after.Cores <= before.Cores {
				return nil, serveroperationresourcecap.ErrCapabilityUnavailable
			}
			parameters.CPU = after.Cores
		case resourcecap.ServerProvisionComponentMemory:
			if after.Megabytes <= before.Megabytes {
				return nil, serveroperationresourcecap.ErrCapabilityUnavailable
			}
			parameters.Memory = after.Megabytes
		case resourcecap.ServerProvisionComponentSystemDisk:
			if after.Gigabytes <= before.Gigabytes {
				return nil, serveroperationresourcecap.ErrCapabilityUnavailable
			}
			parameters.SystemDisk = after.Gigabytes
		case resourcecap.ServerProvisionComponentBandwidth:
			if after.MegabitsPerSecond <= before.MegabitsPerSecond {
				return nil, serveroperationresourcecap.ErrCapabilityUnavailable
			}
			parameters.InBW, parameters.OutBW = after.MegabitsPerSecond, after.MegabitsPerSecond
		default:
			// The existing resize contract has no per-disk identity or quota
			// mutation operation. Refuse before charging rather than guess.
			return nil, serveroperationresourcecap.ErrCapabilityUnavailable
		}
	}
	if parameters == (resizeSizingParameters{}) {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	return encoded, nil
}
