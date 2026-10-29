package serveroperationsource

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
)

type capabilityMappingRow struct {
	ProviderPluginID string `orm:"provider_plugin_id"`
	ServiceNo        string `orm:"local_resource_no"`
	ChannelNo        string `orm:"channel_no"`
	PoolNo           string `orm:"pool_no"`
}

type latestCapabilityRow struct {
	SnapshotNo string    `orm:"snapshot_no"`
	ChannelNo  string    `orm:"channel_no"`
	ObservedAt time.Time `orm:"observed_at"`
}

type capabilityItemRow struct {
	SnapshotNo     string `orm:"snapshot_no"`
	CapabilityCode string `orm:"capability_code"`
	SupportCode    string `orm:"support_code"`
}

var _ serveroperationresourcecap.CapabilityDirectory = (*Service)(nil)
var _ serveroperationresourcecap.ReinstallDirectory = (*Service)(nil)
var _ serveroperationresourcecap.AccessoryDirectory = (*Service)(nil)
var _ serveroperationresourcecap.EntitlementRouteDirectory = (*Service)(nil)

func (s *Service) ResolveEntitlementRoute(
	ctx context.Context,
	request serveroperationresourcecap.EntitlementRouteRequest,
) (*serveroperationresourcecap.EntitlementRoute, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		!validDirectoryReference(request.ServiceNo, 40) ||
		!validDirectoryReference(request.CustomerNo, 40) {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	var rows []serveroperationresourcecap.EntitlementRoute
	if err := s.database.GetScan(ctx, &rows, `
SELECT ownership.local_resource_no AS service_no,mapping.channel_no,mapping.pool_no
FROM plugin_idc_resource_server_entitlement_projection ownership
JOIN plugin_idc_resource_mapping mapping
  ON mapping.tenant_id=ownership.tenant_id
 AND mapping.mapping_no=ownership.mapping_no
 AND mapping.local_resource_no=ownership.local_resource_no
JOIN plugin_idc_resource_channel channel
  ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no
JOIN plugin_idc_resource_pool pool
  ON pool.tenant_id=mapping.tenant_id AND pool.channel_no=mapping.channel_no
 AND pool.pool_no=mapping.pool_no
WHERE ownership.tenant_id=? AND ownership.local_resource_no=? AND ownership.customer_no=?
  AND mapping.status_code='active' AND mapping.external_resource_ref<>''
  AND channel.status_code='enabled' AND channel.deleted_at IS NULL
  AND pool.status_code='enabled' AND pool.deleted_at IS NULL
ORDER BY ownership.id LIMIT 2`, idccap.DefaultTenantID, request.ServiceNo,
		request.CustomerNo); err != nil {
		return nil, gerror.Wrap(err, "resolve entitlement resource route")
	}
	if len(rows) != 1 || rows[0].ServiceNo != request.ServiceNo ||
		rows[0].ChannelNo == "" || rows[0].PoolNo == "" {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return &rows[0], nil
}

func (s *Service) GetAccessories(ctx context.Context,
	route serveroperationresourcecap.CapabilityRoute) (*serveroperationresourcecap.AccessoryProjection, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		!validDirectoryReference(route.ServiceNo, 40) || !validDirectoryReference(route.ChannelNo, 64) ||
		!validDirectoryReference(route.PoolNo, 64) {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	var exact *struct {
		MappingNo        string `orm:"mapping_no"`
		ProviderPluginID string `orm:"provider_plugin_id"`
	}
	if err := s.database.GetScan(ctx, &exact, `SELECT mapping.mapping_no,channel.provider_plugin_id
FROM plugin_idc_resource_mapping mapping
JOIN plugin_idc_resource_channel channel ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no
WHERE mapping.tenant_id=? AND mapping.local_resource_no=? AND mapping.channel_no=? AND mapping.pool_no=?
 AND mapping.status_code='active' AND mapping.external_resource_ref<>''`, idccap.DefaultTenantID,
		route.ServiceNo, route.ChannelNo, route.PoolNo); err != nil {
		return nil, gerror.Wrap(err, "read accessory resource route")
	}
	result := &serveroperationresourcecap.AccessoryProjection{ServiceNo: route.ServiceNo,
		Actions: []serveroperationresourcecap.Action{}, Items: []serveroperationresourcecap.AccessoryItem{}}
	if exact == nil {
		return result, nil
	}
	capabilities, err := s.ListCapabilities(ctx, []serveroperationresourcecap.CapabilityRoute{route})
	if err != nil || len(capabilities) != 1 {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	directoryService, err := accessorydirectory.New(s.database)
	if err != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	directory, err := directoryService.Read(ctx, route.ServiceNo)
	if err != nil {
		if errors.Is(err, accessorydirectory.ErrDirectoryUnavailable) {
			return result, nil
		}
		return nil, err
	}
	result.SnapshotNo = directory.SnapshotNo
	result.ObservedAt = directory.ObservedAt
	result.ExpiresAt = directory.ExpiresAt
	result.Actions = append(result.Actions, capabilities[0].Actions...)
	for _, item := range directory.Items {
		result.Items = append(result.Items, serveroperationresourcecap.AccessoryItem{
			ObjectNo: item.ObjectNo, Kind: serveroperationresourcecap.AccessoryKind(item.Kind),
			Name: item.Name, State: serveroperationresourcecap.AccessoryState(providerAccessoryState(exact.ProviderPluginID, item.Kind, item.State)),
			Projection: append(json.RawMessage(nil), item.Projection...),
		})
	}
	return result, nil
}

type imageCapacityRow struct {
	ExternalRef string          `orm:"external_ref"`
	Projection  json.RawMessage `orm:"projection"`
}

type latestCapacityObservationRow struct {
	ObservationNo string    `orm:"observation_no"`
	ChannelNo     string    `orm:"channel_no"`
	ObservedAt    time.Time `orm:"observed_at"`
}

type imageProjectionWire struct {
	Name     string          `json:"name"`
	Group    string          `json:"group"`
	Status   json.RawMessage `json:"status"`
	OSType   string          `json:"os_type"`
	Arch     string          `json:"arch"`
	BootMode string          `json:"boot_mode"`
}

func (s *Service) GetReinstallImages(ctx context.Context,
	route serveroperationresourcecap.CapabilityRoute) (*serveroperationresourcecap.ReinstallImageProjection, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	if !validDirectoryReference(route.ServiceNo, 40) || !validDirectoryReference(route.ChannelNo, 64) ||
		!validDirectoryReference(route.PoolNo, 64) {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	return s.listReinstallImages(ctx, route)
}

func (s *Service) listReinstallImages(ctx context.Context,
	route serveroperationresourcecap.CapabilityRoute) (*serveroperationresourcecap.ReinstallImageProjection, error) {
	var observation *latestCapacityObservationRow
	if err := s.database.GetScan(ctx, &observation, `SELECT observation.observation_no,observation.channel_no,observation.observed_at
FROM plugin_idc_resource_mapping mapping
JOIN LATERAL (
 SELECT candidate.observation_no,candidate.channel_no,candidate.observed_at
 FROM plugin_idc_resource_provider_capacity_observation candidate
 WHERE candidate.tenant_id=mapping.tenant_id AND candidate.channel_no=mapping.channel_no
  AND candidate.capacity_kind='image' AND candidate.status_code='complete'
 ORDER BY candidate.observed_at DESC,candidate.id DESC LIMIT 1
) observation ON TRUE
WHERE mapping.tenant_id=? AND mapping.local_resource_no=? AND mapping.channel_no=? AND mapping.pool_no=?
 AND mapping.status_code='active' AND mapping.external_resource_ref<>''
LIMIT 1`, idccap.DefaultTenantID, route.ServiceNo, route.ChannelNo, route.PoolNo); err != nil {
		return nil, gerror.Wrap(err, "read latest reinstall image capacity observation")
	}
	projection := &serveroperationresourcecap.ReinstallImageProjection{ServiceNo: route.ServiceNo, Images: []serveroperationresourcecap.ReinstallImage{}}
	if observation == nil || observation.ObservedAt.IsZero() {
		return projection, nil
	}
	nowValue, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return nil, gerror.Wrap(err, "read reinstall image database time")
	}
	now := nowValue.Time().UTC()
	if observation.ObservedAt.After(now) || !observation.ObservedAt.Add(24*time.Hour).After(now) {
		return projection, nil
	}
	var rows []imageCapacityRow
	if err = s.database.Model("plugin_idc_resource_provider_capacity_observation_item").Ctx(ctx).
		Fields("external_ref,projection").Where("tenant_id", idccap.DefaultTenantID).
		Where("observation_no", observation.ObservationNo).Where("capacity_kind", "image").
		Order("external_ref,id").Limit(513).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "read reinstall image choices")
	}
	if len(rows) > 512 {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	projection.ObservedAt = observation.ObservedAt.UTC()
	for _, row := range rows {
		image, ok := projectReinstallImage(row)
		if !ok {
			return nil, serveroperationresourcecap.ErrSourceUnavailable
		}
		projection.Images = append(projection.Images, image)
	}
	sort.Slice(projection.Images, func(i, j int) bool {
		left, right := projection.Images[i], projection.Images[j]
		if left.Group != right.Group {
			return left.Group < right.Group
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.Reference < right.Reference
	})
	return projection, nil
}

func projectReinstallImage(row imageCapacityRow) (serveroperationresourcecap.ReinstallImage, bool) {
	if !positiveReference(row.ExternalRef) || len(row.Projection) == 0 || len(row.Projection) > 16*1024 {
		return serveroperationresourcecap.ReinstallImage{}, false
	}
	var wire imageProjectionWire
	if json.Unmarshal(row.Projection, &wire) != nil || !validImageText(wire.Name, 256) ||
		!validOptionalImageText(wire.Group, 256) || !validOptionalImageText(wire.OSType, 64) ||
		!validOptionalImageText(wire.Arch, 64) || !validOptionalImageText(wire.BootMode, 64) {
		return serveroperationresourcecap.ReinstallImage{}, false
	}
	return serveroperationresourcecap.ReinstallImage{Reference: row.ExternalRef, Name: wire.Name,
		Group: wire.Group, OSType: wire.OSType, Arch: wire.Arch, BootMode: wire.BootMode,
		Selectable: availableImageStatus(wire.Status)}, true
}

func positiveReference(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 63)
	return err == nil && parsed > 0
}

func validImageText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && value == strings.TrimSpace(value)
}

func validOptionalImageText(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value)
}

func availableImageStatus(raw json.RawMessage) bool {
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		return number.String() == "1"
	}
	var enabled bool
	if json.Unmarshal(raw, &enabled) == nil {
		return enabled
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "1", "active", "available", "enabled":
		return true
	default:
		return false
	}
}

func (s *Service) ListCapabilities(ctx context.Context,
	routes []serveroperationresourcecap.CapabilityRoute) ([]serveroperationresourcecap.CapabilityProjection, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || len(routes) == 0 || len(routes) > 64 {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	byService := make(map[string]serveroperationresourcecap.CapabilityRoute, len(routes))
	serviceNos := make([]string, 0, len(routes))
	for _, route := range routes {
		if !validDirectoryReference(route.ServiceNo, 40) || !validDirectoryReference(route.ChannelNo, 64) ||
			!validDirectoryReference(route.PoolNo, 64) {
			return nil, serveroperationresourcecap.ErrSourceUnavailable
		}
		if _, exists := byService[route.ServiceNo]; exists {
			return nil, serveroperationresourcecap.ErrSourceUnavailable
		}
		byService[route.ServiceNo] = route
		serviceNos = append(serviceNos, route.ServiceNo)
	}
	var mappings []capabilityMappingRow
	if err := s.database.Model("plugin_idc_resource_mapping").Ctx(ctx).As("mapping").
		InnerJoin("plugin_idc_resource_channel", "channel", "channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no").
		Fields("mapping.local_resource_no,mapping.channel_no,mapping.pool_no,channel.provider_plugin_id").
		Where("mapping.tenant_id", idccap.DefaultTenantID).Where("mapping.status_code", "active").
		WhereIn("mapping.local_resource_no", serviceNos).Scan(&mappings); err != nil {
		return nil, gerror.Wrap(err, "read server capability mappings")
	}
	channels := make([]string, 0, len(mappings))
	validMappings := make(map[string]capabilityMappingRow, len(mappings))
	channelSet := map[string]struct{}{}
	for _, mapping := range mappings {
		route, exists := byService[mapping.ServiceNo]
		if !exists || mapping.ChannelNo != route.ChannelNo || mapping.PoolNo != route.PoolNo {
			continue
		}
		validMappings[mapping.ServiceNo] = mapping
		if _, exists = channelSet[mapping.ChannelNo]; !exists {
			channelSet[mapping.ChannelNo] = struct{}{}
			channels = append(channels, mapping.ChannelNo)
		}
	}
	var latest []latestCapabilityRow
	if len(channels) > 0 {
		if err := s.database.Model("plugin_idc_resource_capability_snapshot").Ctx(ctx).
			Fields("DISTINCT ON (channel_no) snapshot_no,channel_no,observed_at").
			Where("tenant_id", idccap.DefaultTenantID).Where("status_code", "complete").
			WhereIn("channel_no", channels).Order("channel_no,observed_at DESC,id DESC").Scan(&latest); err != nil {
			return nil, gerror.Wrap(err, "read latest server capability snapshots")
		}
	}
	snapshotNos := make([]string, 0, len(latest))
	latestByChannel := make(map[string]latestCapabilityRow, len(latest))
	for _, row := range latest {
		latestByChannel[row.ChannelNo] = row
		snapshotNos = append(snapshotNos, row.SnapshotNo)
	}
	var items []capabilityItemRow
	if len(snapshotNos) > 0 {
		if err := s.database.Model("plugin_idc_resource_capability_snapshot_item").Ctx(ctx).
			Fields("snapshot_no,capability_code,support_code").
			Where("tenant_id", idccap.DefaultTenantID).WhereIn("snapshot_no", snapshotNos).
			WhereIn("support_code", []string{"supported", "conditional"}).Scan(&items); err != nil {
			return nil, gerror.Wrap(err, "read server capability snapshot items")
		}
	}
	actionsBySnapshot := make(map[string][]serveroperationresourcecap.Action, len(latest))
	for _, item := range items {
		if actions := actionsForCapability(resourcecap.CapabilityCode(item.CapabilityCode)); len(actions) > 0 {
			actionsBySnapshot[item.SnapshotNo] = append(actionsBySnapshot[item.SnapshotNo], actions...)
		}
	}
	nowValue, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return nil, gerror.Wrap(err, "read server capability database time")
	}
	now := nowValue.Time().UTC()
	projections := make([]serveroperationresourcecap.CapabilityProjection, 0, len(routes))
	for _, route := range routes {
		projection := serveroperationresourcecap.CapabilityProjection{ServiceNo: route.ServiceNo}
		if mapping, exists := validMappings[route.ServiceNo]; exists {
			if snapshot, ok := latestByChannel[mapping.ChannelNo]; ok && !snapshot.ObservedAt.After(now) &&
				snapshot.ObservedAt.Add(24*time.Hour).After(now) {
				projection.ObservedAt = snapshot.ObservedAt.UTC()
				projection.Actions = make([]serveroperationresourcecap.Action, 0, len(actionsBySnapshot[snapshot.SnapshotNo]))
				for _, action := range actionsBySnapshot[snapshot.SnapshotNo] {
					if !withdrawnProviderAction(mapping.ProviderPluginID, action) {
						projection.Actions = append(projection.Actions, action)
					}
				}
				sort.Slice(projection.Actions, func(i, j int) bool { return projection.Actions[i] < projection.Actions[j] })
			}
		}
		projections = append(projections, projection)
	}
	return projections, nil
}

func actionsForCapability(capability resourcecap.CapabilityCode) []serveroperationresourcecap.Action {
	switch capability {
	case resourcecap.CapabilityStart:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionStart}
	case resourcecap.CapabilityStop:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionStop}
	case resourcecap.CapabilityReboot:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionReboot}
	case resourcecap.CapabilityResetPassword:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionResetPassword}
	case resourcecap.CapabilityConsole:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionConsole}
	case resourcecap.CapabilityEnterRescue:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionEnterRescue}
	case resourcecap.CapabilityExitRescue:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionExitRescue}
	case resourcecap.CapabilityReinstall:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionReinstall}
	case resourcecap.CapabilityResize:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionResize}
	case resourcecap.CapabilityChangeIP:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionChangeIP}
	case resourcecap.CapabilityManageSnapshot:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionSnapshotCreate,
			serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionSnapshotRestore}
	case resourcecap.CapabilityBackup:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionBackupCreate,
			serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore}
	case resourcecap.CapabilitySecurityGroup:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionSecurityGroupAttach,
			serveroperationresourcecap.ActionSecurityGroupDetach}
	case resourcecap.CapabilityNAT:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionNATCreate,
			serveroperationresourcecap.ActionNATDelete}
	case resourcecap.CapabilityVirtualMedia:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionVirtualMediaMount,
			serveroperationresourcecap.ActionVirtualMediaUnmount}
	case resourcecap.CapabilityTrafficPackage:
		return []serveroperationresourcecap.Action{serveroperationresourcecap.ActionTrafficAdd}
	default:
		return nil
	}
}

func validDirectoryReference(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && value == strings.TrimSpace(value)
}
