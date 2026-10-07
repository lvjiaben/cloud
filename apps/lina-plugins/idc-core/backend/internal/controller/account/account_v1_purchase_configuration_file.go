// This file maps authenticated purchase file reservations, uploads, and safe
// status reads without exposing private object or scanner details.

package account

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

func (c *ControllerV1) PurchaseConfigurationFileUploadCreate(ctx context.Context, req *v1.PurchaseConfigurationFileUploadCreateReq) (*v1.PurchaseConfigurationFileUploadCreateRes, error) {
	item, err := c.purchaseFiles.Reserve(ctx, purchaseconfigurationfile.ReserveInput{PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, FieldToken: req.FieldToken, FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.PurchaseConfigurationFileUploadCreateRes{UploadToken: item.UploadToken, ExpiresAt: item.ExpiresAt.UnixMilli()}, nil
}

func (c *ControllerV1) PurchaseConfigurationFileUploadContentUpdate(ctx context.Context, req *v1.PurchaseConfigurationFileUploadContentUpdateReq) (*v1.PurchaseConfigurationFileUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, bizerr.NewCode(purchaseconfigurationfile.CodeUnavailable)
	}
	item, err := c.purchaseFiles.Upload(ctx, req.UploadToken, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return purchaseFileResponse(item), nil
}

func (c *ControllerV1) PurchaseConfigurationFileUploadDetail(ctx context.Context, req *v1.PurchaseConfigurationFileUploadDetailReq) (*v1.PurchaseConfigurationFileUploadDetailRes, error) {
	item, err := c.purchaseFiles.Get(ctx, req.UploadToken)
	if err != nil {
		return nil, err
	}
	return purchaseFileResponse(item), nil
}

func (c *ControllerV1) PurchaseConfigurationFileUploadRevoke(ctx context.Context, req *v1.PurchaseConfigurationFileUploadRevokeReq) (*v1.PurchaseConfigurationFileUploadRevokeRes, error) {
	item, err := c.purchaseFiles.Revoke(ctx, req.UploadToken)
	if err != nil {
		return nil, err
	}
	return purchaseFileResponse(item), nil
}

func purchaseFileResponse(item *purchaseconfigurationfile.Item) *v1.PurchaseConfigurationFileUploadContentUpdateRes {
	return &v1.PurchaseConfigurationFileUploadContentUpdateRes{FileToken: item.FileToken, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Status: item.Status, ExpiresAt: item.ExpiresAt.UnixMilli()}
}
