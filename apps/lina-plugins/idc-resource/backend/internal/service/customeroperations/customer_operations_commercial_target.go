// This file builds commercial context from owner facts. No browser-supplied
// entitlement, route, price, runtime state or supplier identity is trusted.
package customeroperations

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// CommercialTarget is the source-only context for quote, package and checkout
// adapters. Core still rechecks its entitlement, ownership and financial fence.
func (s *Service) CommercialTarget(ctx context.Context, serverNo string) (servercommercialcap.TargetSnapshot, error) {
	var result servercommercialcap.TargetSnapshot
	owned, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil {
		return result, err
	}
	entitlement, err := s.entitlements.ReadCurrentEntitlement(ctx, owned.EntitlementNo)
	if err != nil || entitlement == nil || !entitlement.CanOperate || entitlement.CustomerNo != owned.CustomerNo || entitlement.EntitlementNo != owned.EntitlementNo {
		return result, ErrUnavailable
	}
	if busy, busyErr := s.busy(ctx, owned.LocalResourceNo); busyErr != nil || busy {
		return result, ErrConflict
	}
	observationCtx, cancelObservation := context.WithTimeout(ctx, customerObservationBudget)
	observation, err := s.observe(observationCtx, owned)
	cancelObservation()
	if err != nil || observation.State == resourcecap.ResourceStateUnknown {
		return result, ErrUnavailable
	}
	directoryCtx, cancelDirectory := context.WithTimeout(ctx, customerCapabilityBudget)
	directory, err := s.operations.GetAccessories(directoryCtx, serveroperationresourcecap.CapabilityRoute{ServiceNo: owned.LocalResourceNo, ChannelNo: owned.ChannelNo, PoolNo: owned.PoolNo})
	cancelDirectory()
	if err != nil || directory == nil || directory.ServiceNo != owned.LocalResourceNo || directory.SnapshotNo == "" || directory.ObservedAt.IsZero() || !directory.ExpiresAt.After(time.Now()) || len(directory.Items) > 500 {
		return result, ErrUnavailable
	}
	current, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil || !sameRoute(current, owned) {
		return result, ErrUnavailable
	}
	state := string(observation.State)
	if observation.State == resourcecap.ResourceStateActive {
		state = "running"
	}
	result = servercommercialcap.TargetSnapshot{EntitlementNo: owned.EntitlementNo, CustomerNo: owned.CustomerNo, LocalResourceNo: owned.LocalResourceNo, ChannelNo: owned.ChannelNo, PoolNo: owned.PoolNo, EntitlementVersion: entitlement.Version, RuntimeStatus: state, SnapshotNo: directory.SnapshotNo, ObservedAt: directory.ObservedAt, ExpiresAt: directory.ExpiresAt, Actions: directory.Actions, Items: directory.Items}
	// A suspended VM is never assumed to be suspended for traffic. That reason
	// requires a dedicated verified provider fact before traffic checkout is enabled.
	return result, nil
}
