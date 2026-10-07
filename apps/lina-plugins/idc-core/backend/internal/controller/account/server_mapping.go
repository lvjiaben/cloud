package account

import (
	"time"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func toAPICustomerServer(value *serveroperation.Server) *v1.CustomerServer {
	if value == nil {
		return nil
	}
	result := &v1.CustomerServer{
		ServiceNo: value.ServiceNo, CustomerAlias: value.CustomerAlias,
		CustomerRemark: value.CustomerRemark, AutoRenewEnabled: value.AutoRenewEnabled,
		RenewalCycles: value.RenewalCycles, ActiveRefundNo: value.ActiveRefundNo,
		ProductNameZhCN: value.ProductNameZhCN,
		ProductNameEnUS: value.ProductNameEnUS, ProductType: value.ProductType,
		EntitlementStatus: value.EntitlementStatus, RuntimeStatus: value.RuntimeStatus,
		ProviderTaskStatus: value.ProviderTaskStatus, PeriodUnit: value.PeriodUnit,
		PeriodCount: value.PeriodCount, EntitlementStartedAt: unixMilliPointer(value.EntitlementStartedAt),
		EntitlementExpiresAt: unixMilliPointer(value.EntitlementExpiresAt), UpdatedAt: value.UpdatedAt.UnixMilli(),
		Version: value.Version, CapabilityObservedAt: unixMilliPointer(value.CapabilityObservedAt),
		AvailableActions: make([]string, 0, len(value.AvailableActions)),
	}
	for _, action := range value.AvailableActions {
		result.AvailableActions = append(result.AvailableActions, string(action))
	}
	if value.ActiveOperation != nil {
		result.ActiveOperation = &v1.ServerOperationSummary{
			OperationNo: value.ActiveOperation.OperationNo, Action: string(value.ActiveOperation.Action),
			Status: value.ActiveOperation.Status, CreatedAt: value.ActiveOperation.CreatedAt.UnixMilli(),
		}
	}
	return result
}

func toAPICustomerServerDetail(value *serveroperation.ServerDetail) *v1.CustomerServerDetail {
	if value == nil {
		return nil
	}
	result := &v1.CustomerServerDetail{Server: toAPICustomerServer(&value.Server),
		Configuration:   make([]*v1.CustomerServerConfigurationItem, 0, len(value.Configuration)),
		Operations:      make([]*v1.CustomerServerOperationHistory, 0, len(value.Operations)),
		ReinstallImages: make([]*v1.CustomerServerReinstallImage, 0, len(value.ReinstallImages)),
		Accessories:     make([]*v1.CustomerServerAccessory, 0, len(value.Accessories))}
	for _, item := range value.Configuration {
		result.Configuration = append(result.Configuration, &v1.CustomerServerConfigurationItem{
			ComponentType: item.ComponentType, GroupNameZhCN: item.GroupNameZhCN,
			GroupNameEnUS: item.GroupNameEnUS, OptionNameZhCN: item.OptionNameZhCN,
			OptionNameEnUS: item.OptionNameEnUS, NormalizedValue: item.NormalizedValue,
			Quantity: item.Quantity,
		})
	}
	for _, operation := range value.Operations {
		result.Operations = append(result.Operations, &v1.CustomerServerOperationHistory{
			OperationNo: operation.OperationNo, Action: string(operation.Action), Status: operation.Status,
			FailureCode: operation.FailureCode, CreatedAt: operation.CreatedAt.UnixMilli(),
			UpdatedAt: operation.UpdatedAt.UnixMilli(), CompletedAt: unixMilliPointer(operation.CompletedAt),
			ConfidentialOutputAvailable: operation.ConfidentialOutputAvailable,
		})
	}
	for _, image := range value.ReinstallImages {
		result.ReinstallImages = append(result.ReinstallImages, &v1.CustomerServerReinstallImage{
			Reference: image.Reference, Name: image.Name, Group: image.Group, OSType: image.OSType,
			Arch: image.Arch, BootMode: image.BootMode, Selectable: image.Selectable,
		})
	}
	result.ReinstallImagesObservedAt = unixMilliPointer(value.ReinstallImagesObservedAt)
	for _, item := range value.Accessories {
		result.Accessories = append(result.Accessories, &v1.CustomerServerAccessory{
			ObjectNo: item.ObjectNo, Kind: string(item.Kind), Name: item.Name,
			State: string(item.State), Projection: item.Projection,
		})
	}
	result.AccessoriesObservedAt = unixMilliPointer(value.AccessoriesObservedAt)
	result.AccessoriesExpiresAt = unixMilliPointer(value.AccessoriesExpiresAt)
	return result
}

func unixMilliPointer(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	result := value.UTC().UnixMilli()
	return &result
}
