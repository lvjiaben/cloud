// This file explicitly binds an authenticated API customer's uploader identity
// to file verification. It creates neither browser sessions nor host identities.
package backend

import (
	"context"
	"io"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type channelFilePrincipalKey struct{}
type channelFilePrincipal struct{}

func (channelFilePrincipal) Current(ctx context.Context) (*principal.Audience, error) {
	value, ok := ctx.Value(channelFilePrincipalKey{}).(principal.Audience)
	if !ok || value.UserID < 1 || value.CustomerNo == "" || value.Code != principal.AudienceCustomer || value.Status != principal.AudienceStatusActive {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return &value, nil
}

func (runtime *channelCommerceRuntime) fileCustomerContext(ctx context.Context, customerNo string) (context.Context, error) {
	row, err := runtime.requirePayer(ctx, customerNo)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, channelFilePrincipalKey{}, principal.Audience{UserID: row.ActorUserID, CustomerNo: row.CustomerNo, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}), nil
}

func (runtime *channelCommerceRuntime) ReservePurchaseFile(ctx context.Context, customerNo string, input channelcommercecap.PurchaseFileInput) (*channelcommercecap.PurchaseFileReservation, error) {
	ctx, err := runtime.fileCustomerContext(ctx, customerNo)
	if err != nil {
		return nil, err
	}
	result, err := runtime.purchaseFiles.Reserve(ctx, purchaseconfigurationfile.ReserveInput{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, FieldToken: input.FieldToken, FileName: input.FileName, SizeBytes: input.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &channelcommercecap.PurchaseFileReservation{UploadToken: result.UploadToken, ExpiresAt: result.ExpiresAt.UnixMilli()}, nil
}

func (runtime *channelCommerceRuntime) UploadPurchaseFile(ctx context.Context, customerNo, token, mediaType string, body io.Reader) (*channelcommercecap.PurchaseFile, error) {
	ctx, err := runtime.fileCustomerContext(ctx, customerNo)
	if err != nil {
		return nil, err
	}
	result, err := runtime.purchaseFiles.Upload(ctx, token, mediaType, body)
	if err != nil {
		return nil, err
	}
	return &channelcommercecap.PurchaseFile{FileToken: result.FileToken, FileName: result.FileName, MediaType: result.MediaType, Status: result.Status, SizeBytes: result.SizeBytes, ExpiresAt: result.ExpiresAt.UnixMilli()}, nil
}
