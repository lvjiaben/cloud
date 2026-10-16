// This file resolves the authenticated application binding before any private
// file operation. HTTP callers cannot nominate another customer's identity.
package channel

import (
	"context"
	"io"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func (s *service) ReservePurchaseFile(ctx context.Context, appID string, input channelcommercecap.PurchaseFileInput) (*channelcommercecap.PurchaseFileReservation, error) {
	account, err := s.requireChannel(ctx, appID, true)
	if err != nil {
		return nil, err
	}
	runtime, ok := s.core.(channelcommercecap.PurchaseFileRuntime)
	if !ok {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return runtime.ReservePurchaseFile(ctx, account.CustomerNo, input)
}

func (s *service) UploadPurchaseFile(ctx context.Context, appID, token, mediaType string, body io.Reader) (*channelcommercecap.PurchaseFile, error) {
	account, err := s.requireChannel(ctx, appID, true)
	if err != nil {
		return nil, err
	}
	runtime, ok := s.core.(channelcommercecap.PurchaseFileRuntime)
	if !ok {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return runtime.UploadPurchaseFile(ctx, account.CustomerNo, token, mediaType, body)
}
