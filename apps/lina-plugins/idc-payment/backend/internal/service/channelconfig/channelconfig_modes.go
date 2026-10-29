// This file updates channel payment modes with exact provider admission and CAS.
// The existing encrypted credential handle and verified connection are retained.
package channelconfig

import (
	"context"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/model/do"
)

// SaveModes updates only the supported entry points of one channel. It requires
// direct staff identity from the controller, current provider support, and the
// exact channel version. Stale, empty, duplicate or unauthorized modes fail
// without credential mutation or financial side effects.
func (s *Service) SaveModes(ctx context.Context, channelNo string, modes []paymentcap.Mode, expectedVersion int64, actorID int) (*Channel, error) {
	if strings.TrimSpace(channelNo) != channelNo || channelNo == "" || actorID <= 0 || expectedVersion <= 0 || len(modes) == 0 || len(modes) > 16 {
		return nil, bizerr.NewCode(CodeChannelModesInvalid)
	}
	channel, err := s.Get(ctx, channelNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeChannelModesUnavailable)
	}
	if channel.Version != expectedVersion {
		return nil, bizerr.NewCode(CodeChannelModesConflict)
	}
	if err = s.requireAvailableProvider(ctx, channel.Provider); err != nil {
		return nil, bizerr.WrapCode(err, CodeChannelModesUnavailable)
	}
	if s.extensions != nil {
		candidates, candidateErr := s.ListProviderCandidates(ctx)
		if candidateErr != nil {
			return nil, bizerr.WrapCode(candidateErr, CodeChannelModesUnavailable)
		}
		allowed := map[paymentcap.Mode]bool{}
		for _, candidate := range candidates {
			if candidate.Descriptor.ProviderCode == channel.Provider {
				for _, mode := range candidate.Descriptor.Modes {
					allowed[mode] = true
				}
			}
		}
		for _, mode := range modes {
			if !allowed[mode] {
				return nil, bizerr.NewCode(CodeChannelModesInvalid)
			}
		}
	}
	var (
		capabilities  paymentcap.Capabilities
		validationErr error
	)
	err = s.WithCredentials(ctx, channelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		raw := CredentialEnvelope{Extra: map[string]string{"jsapi_authorized": string(credentials.Extra["jsapi_authorized"])}}
		capabilities, validationErr = verifiedCapabilities(adapter.Capabilities(), channel.Provider, modes, raw)
		return validationErr
	})
	if validationErr != nil {
		return nil, bizerr.NewCode(CodeChannelModesInvalid)
	}
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeChannelModesUnavailable)
	}
	modeJSON, currencies, capJSON, err := capabilityProjection(capabilities)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeChannelModesUnavailable)
	}
	result, err := s.database.Model(do.Channel{}).Ctx(ctx).Where(do.Channel{TenantId: 0, ChannelNo: channelNo, Version: expectedVersion}).Data(do.Channel{SupportedModes: modeJSON, SupportedCurrencies: currencies, CapabilitySnapshot: capJSON, Version: expectedVersion + 1, UpdatedBy: actorID}).Update()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeChannelModesUnavailable)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeChannelModesUnavailable)
	}
	if affected != 1 {
		return nil, bizerr.NewCode(CodeChannelModesConflict)
	}
	updated, readErr := s.Get(ctx, channelNo)
	if readErr != nil {
		return nil, bizerr.WrapCode(readErr, CodeChannelModesUnavailable)
	}
	return updated, nil
}
