// This file adapts staff server accessory API requests to the governed service boundary.

package admin

import (
	"context"
	"strconv"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerAccessoryManagementDetail(ctx context.Context,
	req *v1.ServerAccessoryManagementDetailReq) (*v1.ServerAccessoryManagementDetailRes, error) {
	value, err := c.serverAccessoryManagement.Get(ctx, req.ServiceNo)
	if err != nil {
		return nil, err
	}
	detail := value.Detail
	projection := &v1.ManagementServerAccessoryDetail{
		ServiceNo: detail.ServiceNo, CustomerNo: value.CustomerNo,
		ProductNameZhCN: detail.ProductNameZhCN, ProductNameEnUS: detail.ProductNameEnUS,
		EntitlementStatus: detail.EntitlementStatus, RuntimeStatus: detail.RuntimeStatus,
		ProviderTaskStatus: detail.ProviderTaskStatus,
		AvailableActions:   make([]string, 0, len(detail.AvailableActions)),
		Accessories:        make([]*v1.ManagementServerAccessory, 0, len(detail.Accessories)),
	}
	if detail.ActiveOperation != nil {
		projection.ActiveOperationNo = detail.ActiveOperation.OperationNo
	}
	for _, action := range detail.AvailableActions {
		projection.AvailableActions = append(projection.AvailableActions, string(action))
	}
	for _, item := range detail.Accessories {
		projection.Accessories = append(projection.Accessories, &v1.ManagementServerAccessory{
			ObjectNo: item.ObjectNo, Kind: string(item.Kind), Name: item.Name,
			State: string(item.State), Projection: append([]byte(nil), item.Projection...),
		})
	}
	if detail.AccessoriesObservedAt != nil {
		value := detail.AccessoriesObservedAt.UnixMilli()
		projection.AccessoriesObservedAt = &value
	}
	if detail.AccessoriesExpiresAt != nil {
		value := detail.AccessoriesExpiresAt.UnixMilli()
		projection.AccessoriesExpiresAt = &value
	}
	return &v1.ServerAccessoryManagementDetailRes{Detail: projection}, nil
}

func (c *ControllerV1) createServerAccessoryOperation(ctx context.Context,
	req *v1.ServerAccessoryManagementOperationInput,
	action serveroperationresourcecap.Action) (*v1.ServerAccessoryManagementOperationRes, error) {
	operation, err := c.serverAccessoryManagement.Create(ctx, serveroperation.ManagementCommand{
		ServiceNo: req.ServiceNo, Action: action, PackageNo: req.PackageNo, PackageVersion: req.PackageVersion,
		Parameters: req.Parameters, IdempotencyKey: req.RequestNo, Confirmed: req.Confirmed,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerAccessoryManagementOperationRes{
		OperationNo: operation.OperationNo, Action: string(operation.Action),
		OrderNo: operation.OrderNo, InvoiceNo: operation.InvoiceNo, Status: operation.Status,
		AmountMinor: strconv.FormatInt(operation.AmountMinor, 10), CurrencyCode: operation.CurrencyCode,
		PaymentRequired: operation.PaymentRequired,
	}, nil
}

func (c *ControllerV1) ServerAccessorySnapshotCreate(ctx context.Context,
	req *v1.ServerAccessorySnapshotCreateReq) (*v1.ServerAccessorySnapshotCreateRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionSnapshotCreate)
}

func (c *ControllerV1) ServerAccessorySnapshotDelete(ctx context.Context,
	req *v1.ServerAccessorySnapshotDeleteReq) (*v1.ServerAccessorySnapshotDeleteRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionSnapshotDelete)
}

func (c *ControllerV1) ServerAccessorySnapshotRestore(ctx context.Context,
	req *v1.ServerAccessorySnapshotRestoreReq) (*v1.ServerAccessorySnapshotRestoreRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionSnapshotRestore)
}

func (c *ControllerV1) ServerAccessoryBackupCreate(ctx context.Context,
	req *v1.ServerAccessoryBackupCreateReq) (*v1.ServerAccessoryBackupCreateRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionBackupCreate)
}

func (c *ControllerV1) ServerAccessoryBackupDelete(ctx context.Context,
	req *v1.ServerAccessoryBackupDeleteReq) (*v1.ServerAccessoryBackupDeleteRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionBackupDelete)
}

func (c *ControllerV1) ServerAccessoryBackupRestore(ctx context.Context,
	req *v1.ServerAccessoryBackupRestoreReq) (*v1.ServerAccessoryBackupRestoreRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionBackupRestore)
}

func (c *ControllerV1) ServerAccessorySecurityGroupAttach(ctx context.Context,
	req *v1.ServerAccessorySecurityGroupAttachReq) (*v1.ServerAccessorySecurityGroupAttachRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionSecurityGroupAttach)
}

func (c *ControllerV1) ServerAccessorySecurityGroupDetach(ctx context.Context,
	req *v1.ServerAccessorySecurityGroupDetachReq) (*v1.ServerAccessorySecurityGroupDetachRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionSecurityGroupDetach)
}

func (c *ControllerV1) ServerAccessoryNATCreate(ctx context.Context,
	req *v1.ServerAccessoryNATCreateReq) (*v1.ServerAccessoryNATCreateRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionNATCreate)
}

func (c *ControllerV1) ServerAccessoryNATDelete(ctx context.Context,
	req *v1.ServerAccessoryNATDeleteReq) (*v1.ServerAccessoryNATDeleteRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionNATDelete)
}

func (c *ControllerV1) ServerAccessoryVirtualMediaMount(ctx context.Context,
	req *v1.ServerAccessoryVirtualMediaMountReq) (*v1.ServerAccessoryVirtualMediaMountRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionVirtualMediaMount)
}

func (c *ControllerV1) ServerAccessoryVirtualMediaUnmount(ctx context.Context,
	req *v1.ServerAccessoryVirtualMediaUnmountReq) (*v1.ServerAccessoryVirtualMediaUnmountRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionVirtualMediaUnmount)
}

func (c *ControllerV1) ServerAccessoryTrafficAdd(ctx context.Context,
	req *v1.ServerAccessoryTrafficAddReq) (*v1.ServerAccessoryTrafficAddRes, error) {
	return c.createServerAccessoryOperation(ctx, &req.ServerAccessoryManagementOperationInput,
		serveroperationresourcecap.ActionTrafficAdd)
}
