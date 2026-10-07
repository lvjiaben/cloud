// This file adapts supplier configuration operations to HTTP DTOs. Directory
// totals and usage remain owner-projected; it does not perform per-row queries.

package admin

import (
	"context"
	"encoding/json"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/providerconfiguration"
	"lina-plugin-idc-resource/backend/internal/service/providerverification"
)

// ProviderConfigurationList preserves independently paged owner results.
func (c *ControllerV1) ProviderConfigurationList(ctx context.Context, req *v1.ProviderConfigurationListReq) (*v1.ProviderConfigurationListRes, error) {
	snapshot, err := c.configurations.List(ctx, providerconfiguration.ListInput{
		ChannelPage: req.ChannelPage, ChannelPageSize: req.ChannelPageSize,
		PoolPage: req.PoolPage, PoolPageSize: req.PoolPageSize,
		ChannelKeyword: req.ChannelKeyword, AvailableChannelsOnly: req.AvailableChannelsOnly,
	})
	if err != nil {
		return nil, err
	}
	result := &v1.ProviderConfigurationListRes{
		Channels:           make([]*v1.ProviderChannelItem, len(snapshot.Channels)),
		Pools:              make([]*v1.ResourcePoolItem, len(snapshot.Pools)),
		ProviderCandidates: make([]*v1.ProviderCandidateItem, len(snapshot.ProviderCandidates)),
		ChannelTotal:       snapshot.ChannelTotal, PoolTotal: snapshot.PoolTotal,
	}
	for i := range snapshot.Channels {
		result.Channels[i], err = providerChannelItem(snapshot.Channels[i])
		if err != nil {
			return nil, err
		}
	}
	for i := range snapshot.Pools {
		result.Pools[i] = resourcePoolItem(snapshot.Pools[i])
	}
	for i, candidate := range snapshot.ProviderCandidates {
		schema, marshalErr := json.Marshal(candidate.InterfaceConfiguration)
		if marshalErr != nil {
			return nil, marshalErr
		}
		result.ProviderCandidates[i] = &v1.ProviderCandidateItem{
			PluginID: candidate.PluginID, ProviderID: candidate.ProviderID,
			DisplayName: candidate.DisplayName, ReleaseVersion: candidate.ReleaseVersion,
			InterfaceConfiguration: schema,
		}
	}
	return result, nil
}

func (c *ControllerV1) ProviderChannelVerificationCreate(ctx context.Context, req *v1.ProviderChannelVerificationCreateReq) (*v1.ProviderChannelVerificationCreateRes, error) {
	result, err := c.verification.Verify(ctx, req.ChannelNo)
	if err != nil {
		return nil, err
	}
	return providerChannelVerificationResult(result), nil
}

func providerChannelVerificationResult(result providerverification.Result) *v1.ProviderChannelVerificationCreateRes {
	return &v1.ProviderChannelVerificationCreateRes{
		Verified: result.Verified, StatusCode: result.StatusCode, ObservedAt: result.ObservedAt.UnixMilli(),
	}
}

func (c *ControllerV1) ProviderChannelCreate(ctx context.Context, req *v1.ProviderChannelCreateReq) (*v1.ProviderChannelCreateRes, error) {
	settings, err := encodeNonSecretSettings(req.NonSecretSettings)
	if err != nil {
		return nil, err
	}
	item, err := c.configurations.CreateChannel(ctx, providerconfiguration.ChannelInput{ProviderPluginID: req.ProviderPluginID, DisplayName: req.DisplayName, Endpoint: req.Endpoint, NonSecretSettings: settings})
	if err != nil {
		return nil, err
	}
	response, err := providerChannelItem(*item)
	if err != nil {
		return nil, err
	}
	return &v1.ProviderChannelCreateRes{Item: response}, nil
}
func (c *ControllerV1) ProviderChannelUpdate(ctx context.Context, req *v1.ProviderChannelUpdateReq) (*v1.ProviderChannelUpdateRes, error) {
	settings, err := encodeNonSecretSettings(req.NonSecretSettings)
	if err != nil {
		return nil, err
	}
	item, err := c.configurations.UpdateChannel(ctx, providerconfiguration.ChannelInput{ChannelNo: req.ChannelNo, DisplayName: req.DisplayName, Endpoint: req.Endpoint, Status: req.Status, Version: req.Version, NonSecretSettings: settings})
	if err != nil {
		return nil, err
	}
	response, err := providerChannelItem(*item)
	if err != nil {
		return nil, err
	}
	return &v1.ProviderChannelUpdateRes{Item: response}, nil
}

func encodeNonSecretSettings(values map[string]any) (json.RawMessage, error) {
	if values == nil {
		return nil, nil
	}
	return json.Marshal(values)
}
func (c *ControllerV1) ResourcePoolCreate(ctx context.Context, req *v1.ResourcePoolCreateReq) (*v1.ResourcePoolCreateRes, error) {
	item, err := c.configurations.CreatePool(ctx, providerconfiguration.PoolInput{ChannelNo: req.ChannelNo, DisplayName: req.DisplayName, ResourceKind: req.ResourceKind, RoutingPriority: req.RoutingPriority, RoutingWeight: req.RoutingWeight})
	if err != nil {
		return nil, err
	}
	return &v1.ResourcePoolCreateRes{Item: resourcePoolItem(*item)}, nil
}
func (c *ControllerV1) ResourcePoolUpdate(ctx context.Context, req *v1.ResourcePoolUpdateReq) (*v1.ResourcePoolUpdateRes, error) {
	item, err := c.configurations.UpdatePool(ctx, providerconfiguration.PoolInput{PoolNo: req.PoolNo, DisplayName: req.DisplayName, ResourceKind: req.ResourceKind, Status: req.Status, RoutingPriority: req.RoutingPriority, RoutingWeight: req.RoutingWeight, Version: req.Version})
	if err != nil {
		return nil, err
	}
	return &v1.ResourcePoolUpdateRes{Item: resourcePoolItem(*item)}, nil
}

func providerChannelItem(item providerconfiguration.Channel) (*v1.ProviderChannelItem, error) {
	schema, err := json.Marshal(item.InterfaceConfiguration)
	if err != nil {
		return nil, err
	}
	secretFields := make([]*v1.ProviderSecretFieldStatus, len(item.SecretFields))
	for index, field := range item.SecretFields {
		status := &v1.ProviderSecretFieldStatus{Key: field.Key, Required: field.Required, Ready: field.Ready}
		if !field.UpdatedAt.IsZero() {
			updatedAt := field.UpdatedAt
			status.UpdatedAt = &updatedAt
		}
		secretFields[index] = status
	}
	var verifiedAt *int64
	var usage *v1.ProviderChannelUsageItem
	if item.Usage != nil {
		usage = &v1.ProviderChannelUsageItem{ActiveInstances: item.Usage.ActiveInstances, TotalInstances: item.Usage.TotalInstances, ConfiguredProducts: item.Usage.ConfiguredProducts}
	}
	if item.VerificationObservedAt != nil {
		value := item.VerificationObservedAt.UnixMilli()
		verifiedAt = &value
	}
	return &v1.ProviderChannelItem{CatalogAvailable: item.CatalogAvailable, Usage: usage, ChannelNo: item.ChannelNo, InterfaceInstanceNo: item.InterfaceInstanceNo, ProviderID: item.ProviderID, ProviderPluginID: item.ProviderPluginID, DisplayName: item.DisplayName, Endpoint: item.Endpoint, NonSecretSettings: item.NonSecretSettings, Status: item.Status, SecretFields: secretFields, InterfaceConfiguration: schema, ProviderContractVersion: item.ProviderContractVersion, ProviderReleaseVersion: item.ProviderReleaseVersion, ProviderReleaseDigest: item.ProviderReleaseDigest, ProviderDeclarationDigest: item.ProviderDeclarationDigest, ConnectionStatus: item.VerificationHealthCode, ConnectionStatusCode: item.VerificationSummaryCode, ConnectionVerifiedAt: verifiedAt, Version: item.Version}, nil
}
func resourcePoolItem(item providerconfiguration.Pool) *v1.ResourcePoolItem {
	return &v1.ResourcePoolItem{ChannelName: item.ChannelName, PoolNo: item.PoolNo, ChannelNo: item.ChannelNo, DisplayName: item.DisplayName, ResourceKind: item.ResourceKind, Status: item.Status, RoutingPriority: item.RoutingPriority, RoutingWeight: item.RoutingWeight, Version: item.Version}
}
