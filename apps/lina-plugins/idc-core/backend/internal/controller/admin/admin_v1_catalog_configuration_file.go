package admin

import (
	"context"
	"io"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func managementFileResponse(item *productautomation.ManagementConfigurationFileItem) *v1.CatalogConfigurationFileRes {
	return &v1.CatalogConfigurationFileRes{FileToken: item.FileToken, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Status: item.Status, ExpiresAt: item.ExpiresAt.UnixMilli()}
}
func managementFileBody(ctx context.Context) (string, io.Reader, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return "", nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	return request.GetHeader("Content-Type"), request.Request.Body, nil
}

func (c *ControllerV1) CatalogAutomationInterfaceConfigurationFileReserve(ctx context.Context, req *v1.CatalogAutomationInterfaceConfigurationFileReserveReq) (*v1.CatalogAutomationInterfaceConfigurationFileReserveRes, error) {
	item, err := c.productAutomation.ReserveManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceInterface, TargetNo: req.InterfaceInstanceNo, TargetRevision: req.TargetRevision, FieldKey: req.FieldKey, FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}
func (c *ControllerV1) CatalogAutomationInterfaceConfigurationFileUpload(ctx context.Context, req *v1.CatalogAutomationInterfaceConfigurationFileUploadReq) (*v1.CatalogAutomationInterfaceConfigurationFileUploadRes, error) {
	kind, body, err := managementFileBody(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.productAutomation.UploadManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceInterface, TargetNo: req.InterfaceInstanceNo, TargetRevision: req.TargetRevision}, req.UploadToken, kind, body)
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}
func (c *ControllerV1) CatalogAutomationInterfaceConfigurationFileDetail(ctx context.Context, req *v1.CatalogAutomationInterfaceConfigurationFileDetailReq) (*v1.CatalogAutomationInterfaceConfigurationFileDetailRes, error) {
	item, err := c.productAutomation.GetManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceInterface, TargetNo: req.InterfaceInstanceNo, TargetRevision: req.TargetRevision}, req.UploadToken)
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}
func (c *ControllerV1) CatalogAutomationInterfaceConfigurationFileRevoke(ctx context.Context, req *v1.CatalogAutomationInterfaceConfigurationFileRevokeReq) (*v1.CatalogAutomationInterfaceConfigurationFileRevokeRes, error) {
	item, err := c.productAutomation.RevokeManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceInterface, TargetNo: req.InterfaceInstanceNo, TargetRevision: req.TargetRevision}, req.UploadToken)
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}

func (c *ControllerV1) CatalogProductVersionConfigurationFileReserve(ctx context.Context, req *v1.CatalogProductVersionConfigurationFileReserveReq) (*v1.CatalogProductVersionConfigurationFileReserveRes, error) {
	item, err := c.productAutomation.ReserveManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceProduct, TargetNo: req.VersionNo, TargetRevision: req.TargetRevision, FieldKey: req.FieldKey, FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}
func (c *ControllerV1) CatalogProductVersionConfigurationFileUpload(ctx context.Context, req *v1.CatalogProductVersionConfigurationFileUploadReq) (*v1.CatalogProductVersionConfigurationFileUploadRes, error) {
	kind, body, err := managementFileBody(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.productAutomation.UploadManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceProduct, TargetNo: req.VersionNo, TargetRevision: req.TargetRevision}, req.UploadToken, kind, body)
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}
func (c *ControllerV1) CatalogProductVersionConfigurationFileDetail(ctx context.Context, req *v1.CatalogProductVersionConfigurationFileDetailReq) (*v1.CatalogProductVersionConfigurationFileDetailRes, error) {
	item, err := c.productAutomation.GetManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceProduct, TargetNo: req.VersionNo, TargetRevision: req.TargetRevision}, req.UploadToken)
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}
func (c *ControllerV1) CatalogProductVersionConfigurationFileRevoke(ctx context.Context, req *v1.CatalogProductVersionConfigurationFileRevokeReq) (*v1.CatalogProductVersionConfigurationFileRevokeRes, error) {
	item, err := c.productAutomation.RevokeManagementConfigurationFile(ctx, productautomation.ManagementConfigurationFileInput{Surface: productautomation.ConfigurationSurfaceProduct, TargetNo: req.VersionNo, TargetRevision: req.TargetRevision}, req.UploadToken)
	if err != nil {
		return nil, err
	}
	return managementFileResponse(item), nil
}
