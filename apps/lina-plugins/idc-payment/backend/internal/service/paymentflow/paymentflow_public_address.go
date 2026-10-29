// This file reads shared current routing before creating any new supplier write.
package paymentflow

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

func (s *Service) publicAddresses(ctx context.Context) (*channelconfig.PublicAddress, error) {
	current, err := s.channels.ReadPublicAddress(ctx)
	if err != nil {
		return nil, err
	}
	if current.Version == 0 {
		if !s.ready {
			return nil, bizerr.NewCode(channelconfig.CodePublicAddressUnconfigured)
		}
		return &channelconfig.PublicAddress{NotificationOrigin: s.origin, CustomerReturnOrigin: s.origin}, nil
	}
	return channelconfig.NormalizePublicAddress(*current)
}
