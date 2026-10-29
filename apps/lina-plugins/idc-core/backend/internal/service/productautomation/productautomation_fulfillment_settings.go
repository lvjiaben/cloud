// This file coordinates the merchant's interface and fulfillment choices in the
// existing saved-settings transaction; failed admission never leaves half an edit.

package productautomation

import (
	"context"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// FulfillmentSettingsInput distinguishes retaining a binding (nil), explicitly
// removing it (empty), and selecting an existing admitted interface.
type FulfillmentSettingsInput struct {
	VersionNo           string
	StrategyCode        string
	InterfaceInstanceNo *string
}

// SavedFulfillmentSettings contains only the authoritative saved policy and its
// server-resolved contract; no execution fields are accepted from the merchant.
type SavedFulfillmentSettings struct {
	Policy   *catalog.FulfillmentPolicyEvidence
	Contract *catalog.FrozenFulfillmentContract
}

// SaveFulfillmentSettings uses the shared publication transaction and existing
// action-specific permissions/admission. An omitted interface preserves it.
// Any binding, policy, audit or application failure rolls back the entire edit.
func SaveFulfillmentSettings(ctx context.Context, automation Service, publication catalog.PublicationService, input FulfillmentSettingsInput) (*SavedFulfillmentSettings, error) {
	var result *SavedFulfillmentSettings
	err := publication.WithSavedSettings(ctx, input.VersionNo, func(ctx context.Context) error {
		if input.InterfaceInstanceNo != nil {
			if *input.InterfaceInstanceNo == "" {
				if err := automation.Unbind(ctx, input.VersionNo); err != nil {
					return err
				}
			} else {
				if _, err := automation.Bind(ctx, BindInput{VersionNo: input.VersionNo, InterfaceInstanceNo: *input.InterfaceInstanceNo}); err != nil {
					return err
				}
			}
		}
		authorization, err := automation.AuthorizeFulfillmentPolicyWrite(ctx, input.VersionNo)
		if err != nil {
			return err
		}
		policy, err := catalog.SaveFulfillmentPolicy(ctx, catalog.FulfillmentPolicyUpdateInput{VersionNo: input.VersionNo, StrategyCode: input.StrategyCode}, authorization)
		if err != nil {
			return err
		}
		result = &SavedFulfillmentSettings{Policy: policy, Contract: authorization.Contract}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
