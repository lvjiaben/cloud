// This file maps owner-checked native server facts to the customer API.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
	"time"
)

func (c *ControllerV1) NativeServerDetail(ctx context.Context, req *v1.NativeServerDetailReq) (*v1.NativeServerDetailRes, error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	item, err := c.service.Get(ctx, req.ServerNo)
	if err != nil {
		return nil, publicError(err)
	}
	result := &v1.NativeServerDetailRes{EntitlementNo: item.Entitlement.EntitlementNo, ServerNo: item.ServerNo, Addresses: item.Addresses, AddressesConfirmed: item.AddressesConfirmed, ProductNameZhCN: item.Entitlement.ProductNameZhCN, ProductNameEnUS: item.Entitlement.ProductNameEnUS, EntitlementStatus: item.Entitlement.Status, RuntimeState: string(item.RuntimeState), OrderNo: item.Entitlement.OrderNo, PeriodUnit: item.Entitlement.PeriodUnit, PeriodCount: item.Entitlement.PeriodCount, StartedAt: item.Entitlement.StartedAt.UnixMilli(), AvailableActions: item.AvailableActions, Operations: make([]v1.NativeServerOperation, 0, len(item.Operations)), CredentialAvailable: item.CredentialAvailable}
	result.Information = v1.NativeServerInformation{Hostname: item.Information.Hostname, OSName: item.Information.OSName, LoginUsername: item.Information.LoginUsername, Port: item.Information.Port, CPUCores: item.Information.CPUCores, MemoryMiB: item.Information.MemoryMiB, NetworkType: item.Information.NetworkType, BootOrder: item.Information.BootOrder, Rescue: item.Information.Rescue}
	result.Capabilities = make([]v1.NativeServerCapability, 0, len(item.Capabilities))
	for _, capability := range item.Capabilities {
		result.Capabilities = append(result.Capabilities, v1.NativeServerCapability{Action: capability.Action, Supported: capability.Supported, Available: capability.Available, ReasonCode: capability.ReasonCode})
	}
	result.CommercialCapabilities = make([]v1.NativeServerCommercialCapability, 0, len(item.CommercialCapabilities))
	for _, capability := range item.CommercialCapabilities {
		result.CommercialCapabilities = append(result.CommercialCapabilities, v1.NativeServerCommercialCapability{Action: capability.Action, Supported: capability.Supported, CanQuote: capability.CanQuote, ReasonCode: capability.ReasonCode})
	}
	if item.Entitlement.ExpiresAt != nil {
		value := item.Entitlement.ExpiresAt.UnixMilli()
		result.ExpiresAt = &value
	}
	if !item.ObservedAt.IsZero() {
		value := item.ObservedAt.UnixMilli()
		result.ObservedAt = &value
	}
	for _, operation := range item.Operations {
		result.Operations = append(result.Operations, projectOperation(operation))
	}
	return result, nil
}

func projectOperation(item customeroperations.Operation) v1.NativeServerOperation {
	result := v1.NativeServerOperation{OperationNo: item.OperationNo, Action: item.Action, Status: item.Status, CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), FailureCode: item.FailureCode, AvailableOutputs: []v1.NativeServerOutput{}}
	if item.WaitingUntil != nil {
		stamp := item.WaitingUntil.UnixMilli()
		result.WaitingUntil = &stamp
	}
	for _, output := range item.AvailableOutputs {
		projected := v1.NativeServerOutput{Name: output.Name, Policy: output.Policy}
		if output.ExpiresAt != nil {
			stamp := output.ExpiresAt.UnixMilli()
			projected.ExpiresAt = &stamp
		}
		result.AvailableOutputs = append(result.AvailableOutputs, projected)
	}
	return result
}

// Missing business times remain null instead of becoming the Unix epoch.
func nativeNullableTime(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	stamp := value.UnixMilli()
	return &stamp
}
