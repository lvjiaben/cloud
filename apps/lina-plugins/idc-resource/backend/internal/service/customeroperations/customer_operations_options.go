// This file reads resource-owner directories after proving native ownership.
// It never queries a supplier to render a list of management choices.
package customeroperations

import (
	"context"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// Options keeps explicit observation timestamps separate from candidate values.
type Options struct {
	Images      *serveroperationresourcecap.ReinstallImageProjection
	Accessories *serveroperationresourcecap.AccessoryProjection
}

// Options returns complete local directories only for the customer's active mapping.
func (s *Service) Options(ctx context.Context, serverNo string) (*Options, error) {
	owned, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil {
		return nil, err
	}
	if _, err = s.entitlements.ReadCurrentEntitlement(ctx, owned.EntitlementNo); err != nil {
		return nil, ErrUnavailable
	}
	return s.readOptions(ctx, owned), nil
}

func (s *Service) readOptions(ctx context.Context, owned *route) *Options {
	var err error
	route := serveroperationresourcecap.CapabilityRoute{ServiceNo: owned.LocalResourceNo, ChannelNo: owned.ChannelNo, PoolNo: owned.PoolNo}
	result := &Options{}
	result.Images, err = s.operations.GetReinstallImages(ctx, route)
	if err != nil || (result.Images != nil && result.Images.ObservedAt.IsZero()) {
		result.Images = nil
	}
	result.Accessories, err = s.operations.GetAccessories(ctx, route)
	if err != nil || (result.Accessories != nil && (result.Accessories.ObservedAt.IsZero() || result.Accessories.ExpiresAt.IsZero())) {
		result.Accessories = nil
	}
	return result
}

// choicesAvailable never advertises a directory-dependent operation with stale or missing choices.
func choicesAvailable(options *Options, action serveroperationresourcecap.Action) bool {
	switch action {
	case serveroperationresourcecap.ActionReinstall:
		if options.Images == nil {
			return false
		}
		for _, image := range options.Images.Images {
			if image.Selectable {
				return true
			}
		}
		return false
	case serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete, serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach, serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount, serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore:
		return options.Accessories != nil
	default:
		return true
	}
}
