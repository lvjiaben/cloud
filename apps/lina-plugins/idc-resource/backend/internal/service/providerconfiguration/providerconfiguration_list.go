// This file reads independently paged configuration directories with exact totals
// and one bounded plugin-owned usage projection, preserving recovery visibility.

package providerconfiguration

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

// configurationStatus names the selector's existing recoverable-deletion state.
type configurationStatus string

const deletedConfigurationStatus configurationStatus = "deleted"

// List returns two independently bounded directories under the same staff scope.
// Deleted configurations remain visible so existing restore actions still work.
func (s *service) List(ctx context.Context, input ListInput) (*Snapshot, error) {
	if _, err := s.authorize(ctx, PermissionList, "provider-configurations"); err != nil {
		return nil, err
	}
	if err := normalizeListInput(&input); err != nil {
		return nil, err
	}
	channelQuery := s.db.Model("plugin_idc_resource_channel c").Safe().Ctx(ctx).Unscoped().WherePrefix("c", do.Channel{TenantId: idccap.DefaultTenantID})
	if input.AvailableChannelsOnly {
		channelQuery = channelQuery.WhereNot("c.status_code", deletedConfigurationStatus)
	}
	if input.ChannelKeyword != "" {
		channelQuery = channelQuery.WhereLike("c.display_name", "%"+input.ChannelKeyword+"%")
	}
	channelTotal, err := channelQuery.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	channels := make([]channelListRow, 0, input.ChannelPageSize)
	err = channelQuery.Clone().
		LeftJoin("plugin_idc_resource_interface_channel_binding interface_binding",
			"interface_binding.tenant_id=c.tenant_id AND interface_binding.channel_no=c.channel_no").
		LeftJoin("plugin_idc_resource_channel_health_state health",
			"health.tenant_id=c.tenant_id AND health.channel_no=c.channel_no").
		Fields("c.channel_no,interface_binding.interface_instance_no,c.provider_id,c.provider_plugin_id,c.provider_runtime_type,c.display_name,c.endpoint,c.non_secret_settings::TEXT AS non_secret_settings,c.status_code,c.version",
			"interface_binding.resource_authorized_methods",
			"interface_binding.resource_contract_version,interface_binding.resource_plugin_release_version AS resource_release_version,interface_binding.resource_release_digest,interface_binding.resource_declaration_digest",
			"COALESCE(health.last_authentication_health_code,'unknown') AS verification_health_code,COALESCE(health.last_authentication_summary_code,'') AS verification_summary_code,health.last_authentication_observed_at AS verification_observed_at").
		OrderAsc("c.id").Page(input.ChannelPage, input.ChannelPageSize).Scan(&channels)
	if err != nil {
		return nil, bizerr.WrapCode(errOrLimit(err), CodeUnavailable)
	}
	type poolRow struct {
		PoolNo, ChannelNo, DisplayName, ResourceKind, StatusCode string
		ChannelName                                              string
		RoutingPriority, RoutingWeight                           int
		Version                                                  int64
	}
	poolQuery := s.db.Model("plugin_idc_resource_pool pool").Safe().Ctx(ctx).Unscoped().WherePrefix("pool", do.Pool{TenantId: idccap.DefaultTenantID})
	poolTotal, err := poolQuery.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	pools := make([]poolRow, 0, input.PoolPageSize)
	err = poolQuery.Clone().LeftJoin("plugin_idc_resource_channel channel", "channel.tenant_id=pool.tenant_id AND channel.channel_no=pool.channel_no").
		Fields("pool.pool_no,pool.channel_no,pool.display_name,pool.resource_kind,pool.status_code,pool.routing_priority,pool.routing_weight,pool.version,channel.display_name AS channel_name").
		OrderAsc("pool.id").Page(input.PoolPage, input.PoolPageSize).Scan(&pools)
	if err != nil {
		return nil, bizerr.WrapCode(errOrLimit(err), CodeUnavailable)
	}
	candidates, err := s.dynamicProviderCandidates(ctx)
	if err != nil {
		return nil, err
	}
	result := &Snapshot{
		Channels: make([]Channel, len(channels)), Pools: make([]Pool, len(pools)),
		ProviderCandidates: candidates,
		ChannelTotal:       channelTotal, PoolTotal: poolTotal,
	}
	configurations, secretErr := s.channelConfigurationStatuses(ctx, channels)
	if secretErr != nil {
		return nil, secretErr
	}
	for i, row := range channels {
		configuration := configurations[row.ChannelNo]
		publicSettings, projectionErr := resourcecap.ProjectPublicInterfaceSettings(json.RawMessage(row.NonSecretSettings), false)
		if projectionErr != nil {
			return nil, bizerr.WrapCode(projectionErr, CodeUnavailable)
		}
		result.Channels[i] = Channel{ChannelNo: row.ChannelNo, InterfaceInstanceNo: row.InterfaceInstanceNo, ProviderID: row.ProviderID, ProviderPluginID: row.ProviderPluginID, DisplayName: row.DisplayName, Endpoint: row.Endpoint, NonSecretSettings: publicSettings, Status: row.StatusCode, SecretFields: configuration.SecretFields, InterfaceConfiguration: configuration.InterfaceConfiguration, ProviderContractVersion: row.ResourceContractVersion, ProviderReleaseVersion: row.ResourceReleaseVersion, ProviderReleaseDigest: row.ResourceReleaseDigest, ProviderDeclarationDigest: row.ResourceDeclarationDigest, VerificationHealthCode: row.VerificationHealthCode, VerificationSummaryCode: row.VerificationSummaryCode, VerificationObservedAt: row.VerificationObservedAt, Version: row.Version}
		result.Channels[i].CatalogAvailable = slices.Contains(row.ResourceAuthorizedMethods, resourcecap.DynamicProviderMethodListCatalog) && slices.Contains(row.ResourceAuthorizedMethods, resourcecap.DynamicProviderMethodReadCatalogProduct)
	}
	for i, row := range pools {
		result.Pools[i] = Pool{PoolNo: row.PoolNo, ChannelNo: row.ChannelNo, ChannelName: row.ChannelName, DisplayName: row.DisplayName, ResourceKind: row.ResourceKind, Status: row.StatusCode, RoutingPriority: row.RoutingPriority, RoutingWeight: row.RoutingWeight, Version: row.Version}
	}
	channelNos := make([]string, len(result.Channels))
	for index, channel := range result.Channels {
		channelNos[index] = channel.ChannelNo
	}
	usage, err := s.usage.BatchGet(ctx, channelNos)
	if err != nil {
		return nil, err
	}
	byChannel := make(map[string]*resourceinterfaceprojectioncap.Usage, len(usage))
	for index := range usage {
		byChannel[usage[index].ChannelNo] = &usage[index]
	}
	for index := range result.Channels {
		result.Channels[index].Usage = byChannel[result.Channels[index].ChannelNo]
		if result.Channels[index].Usage == nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	return result, nil
}

// normalizeListInput keeps page offsets and selector search bounded.
func normalizeListInput(input *ListInput) error {
	input.ChannelKeyword = strings.TrimSpace(input.ChannelKeyword)
	for _, page := range []*int{&input.ChannelPage, &input.PoolPage} {
		if *page == 0 {
			*page = 1
		}
		if *page < 1 || *page > 1000000 {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	for _, size := range []*int{&input.ChannelPageSize, &input.PoolPageSize} {
		if *size == 0 {
			*size = 20
		}
		if *size != 10 && *size != 20 && *size != 50 && *size != 100 {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	if len([]rune(input.ChannelKeyword)) > 128 {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}
