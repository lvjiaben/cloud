// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package plugin

import (
	"context"

	"lina-core/api/plugin/v1"
)

type IPluginV1 interface {
	StopNewBusiness(ctx context.Context, req *v1.StopNewBusinessReq) (res *v1.StopNewBusinessRes, err error)
	ResumeNewBusiness(ctx context.Context, req *v1.ResumeNewBusinessReq) (res *v1.ResumeNewBusinessRes, err error)
	BusinessImpact(ctx context.Context, req *v1.BusinessImpactReq) (res *v1.BusinessImpactRes, err error)
	DependencyCheck(ctx context.Context, req *v1.DependencyCheckReq) (res *v1.DependencyCheckRes, err error)
	Detail(ctx context.Context, req *v1.DetailReq) (res *v1.DetailRes, err error)
	Disable(ctx context.Context, req *v1.DisableReq) (res *v1.DisableRes, err error)
	DynamicList(ctx context.Context, req *v1.DynamicListReq) (res *v1.DynamicListRes, err error)
	UploadDynamicPackage(ctx context.Context, req *v1.UploadDynamicPackageReq) (res *v1.UploadDynamicPackageRes, err error)
	Enable(ctx context.Context, req *v1.EnableReq) (res *v1.EnableRes, err error)
	Install(ctx context.Context, req *v1.InstallReq) (res *v1.InstallRes, err error)
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	PageBridgeGrant(ctx context.Context, req *v1.PageBridgeGrantReq) (res *v1.PageBridgeGrantRes, err error)
	ResourceList(ctx context.Context, req *v1.ResourceListReq) (res *v1.ResourceListRes, err error)
	Sync(ctx context.Context, req *v1.SyncReq) (res *v1.SyncRes, err error)
	Uninstall(ctx context.Context, req *v1.UninstallReq) (res *v1.UninstallRes, err error)
	UpdateTenantProvisioningPolicy(ctx context.Context, req *v1.UpdateTenantProvisioningPolicyReq) (res *v1.UpdateTenantProvisioningPolicyRes, err error)
	Upgrade(ctx context.Context, req *v1.UpgradeReq) (res *v1.UpgradeRes, err error)
	UpgradePreview(ctx context.Context, req *v1.UpgradePreviewReq) (res *v1.UpgradePreviewRes, err error)
}
