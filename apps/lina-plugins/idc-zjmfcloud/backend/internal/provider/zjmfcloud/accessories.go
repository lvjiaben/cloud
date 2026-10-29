// This file projects a bounded, field-whitelisted per-server accessory directory.
package zjmfcloud

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const accessorySnapshotTTL = 5 * time.Minute

var _ resourcecap.AccessoryProvider = (*Provider)(nil)

type accessoryDetail struct {
	items                  []resourcecap.AccessoryItem
	userReference          string
	resourceType           string
	securityGroupReference string
	nodeReference          string
	isoReference           string
}

func (p *Provider) QueryAccessories(ctx context.Context, query resourcecap.AccessoryQuery) (*resourcecap.AccessorySnapshot, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if !validNumericReference(query.ResourceRef) {
		return nil, errInvalidConfig
	}
	detail, err := p.readAccessoryDetail(ctx, query.ResourceRef)
	if err != nil {
		return nil, err
	}
	items := detail.items
	additional, err := p.readSnapshotAccessories(ctx, query.ResourceRef)
	if err != nil {
		return nil, err
	}
	items = append(items, additional...)
	additional, err = p.readNATAccessories(ctx, query.ResourceRef)
	if err != nil {
		return nil, err
	}
	items = append(items, additional...)
	if detail.userReference != "" && detail.resourceType != "" {
		additional, err = p.readSecurityGroupAccessories(ctx, detail)
		if err != nil {
			return nil, err
		}
		items = append(items, additional...)
	}
	if detail.nodeReference != "" {
		additional, err = p.readVirtualMediaAccessories(ctx, detail)
		if err != nil {
			return nil, err
		}
		items = append(items, additional...)
	}
	if len(items) > resourcecap.MaximumAccessoryItemCount {
		return nil, errInvalidResponse
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Kind != items[right].Kind {
			return items[left].Kind < items[right].Kind
		}
		return items[left].ExternalRef < items[right].ExternalRef
	})
	observedAt := p.now().UTC()
	snapshot := &resourcecap.AccessorySnapshot{Items: items, ObservedAt: observedAt, ExpiresAt: observedAt.Add(accessorySnapshotTTL)}
	if err := snapshot.ValidateAt(observedAt); err != nil {
		return nil, errInvalidResponse
	}
	return snapshot, nil
}

func (p *Provider) readAccessoryDetail(ctx context.Context, resourceReference string) (accessoryDetail, error) {
	response, err := p.authenticatedRequest(ctx, http.MethodGet, "/clouds/"+resourceReference, nil, nil)
	if err != nil {
		return accessoryDetail{}, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return accessoryDetail{}, readStatusError(response.statusCode)
	}
	object, err := decodeObject(response.body)
	if err != nil {
		return accessoryDetail{}, errInvalidResponse
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	result := accessoryDetail{}
	if raw, exists := object["ip"]; exists {
		entries, decodeErr := decodeRawObjectArray(raw)
		if decodeErr != nil || len(entries) > maximumOwnershipCandidateCount {
			clearRawObjects(entries)
			return accessoryDetail{}, errInvalidResponse
		}
		for _, entry := range entries {
			item, itemErr := projectIPAccessory(entry)
			if itemErr != nil {
				clearRawObjects(entries)
				return accessoryDetail{}, itemErr
			}
			result.items = append(result.items, item)
		}
		clearRawObjects(entries)
	}
	if raw, exists := object["disk"]; exists {
		entries, decodeErr := decodeRawObjectArray(raw)
		if decodeErr != nil || len(entries) > maximumOwnershipCandidateCount {
			clearRawObjects(entries)
			return accessoryDetail{}, errInvalidResponse
		}
		for index, entry := range entries {
			item, itemErr := projectDiskAccessory(entry, index+1)
			if itemErr != nil {
				clearRawObjects(entries)
				return accessoryDetail{}, itemErr
			}
			result.items = append(result.items, item)
		}
		clearRawObjects(entries)
	}
	quota, quotaOK := optionalNonNegativeUint64(object, "traffic_quota")
	temporary, temporaryOK := optionalNonNegativeUint64(object, "tmp_traffic")
	if quotaOK || temporaryOK {
		if !quotaOK || !temporaryOK || quota > ^uint64(0)-temporary {
			return accessoryDetail{}, errInvalidResponse
		}
		projection, marshalErr := json.Marshal(map[string]any{"quota_gb": quota, "temporary_gb": temporary, "total_gb": quota + temporary})
		if marshalErr != nil {
			return accessoryDetail{}, errInvalidResponse
		}
		result.items = append(result.items, resourcecap.AccessoryItem{Kind: resourcecap.AccessoryKindTraffic,
			ExternalRef: resourceReference, DisplayName: "Traffic", State: resourcecap.AccessoryStateActive, Data: projection})
	}
	result.userReference, _ = firstReference(object, "user_id", "userid", "uid")
	result.resourceType, _ = stringScalar(object["type"])
	if result.resourceType != "" && !validUpstreamType(result.resourceType) {
		return accessoryDetail{}, errInvalidResponse
	}
	result.securityGroupReference, _ = nonNegativeReference(object["security"])
	result.nodeReference, _ = firstReference(object, "node_id", "node")
	result.isoReference, _ = nonNegativeReference(object["iso"])
	return result, nil
}

func projectIPAccessory(object rawObject) (resourcecap.AccessoryItem, error) {
	reference, ok := firstReference(object, "id")
	if !ok {
		return resourcecap.AccessoryItem{}, errInvalidResponse
	}
	address := ""
	for _, key := range []string{"ipaddress", "ip", "address"} {
		if value, exists := stringScalar(object[key]); exists {
			address = value
			break
		}
	}
	parsed := net.ParseIP(address)
	if parsed == nil {
		return resourcecap.AccessoryItem{}, errInvalidResponse
	}
	version := 6
	if parsed.To4() != nil {
		version = 4
		address = parsed.To4().String()
	} else {
		address = parsed.String()
	}
	projection, err := json.Marshal(map[string]any{"address": address, "version": version})
	if err != nil {
		return resourcecap.AccessoryItem{}, errInvalidResponse
	}
	return resourcecap.AccessoryItem{Kind: resourcecap.AccessoryKindIP, ExternalRef: reference,
		DisplayName: address, State: resourcecap.AccessoryStateAttached, Data: projection}, nil
}

func projectDiskAccessory(object rawObject, position int) (resourcecap.AccessoryItem, error) {
	reference, ok := firstReference(object, "id")
	if !ok {
		return resourcecap.AccessoryItem{}, errInvalidResponse
	}
	projection, err := projectScalars(object, []string{"size", "disk_size", "type", "status"})
	if err != nil {
		return resourcecap.AccessoryItem{}, errInvalidResponse
	}
	return resourcecap.AccessoryItem{Kind: resourcecap.AccessoryKindDisk, ExternalRef: reference,
		DisplayName: "Disk " + strconv.Itoa(position), State: resourcecap.AccessoryStateAttached, Data: projection}, nil
}

func optionalNonNegativeUint64(object rawObject, key string) (uint64, bool) {
	raw, exists := object[key]
	if !exists {
		return 0, false
	}
	return nonNegativeUint64Scalar(raw)
}

func (p *Provider) readSnapshotAccessories(ctx context.Context, resourceReference string) ([]resourcecap.AccessoryItem, error) {
	response, err := p.authenticatedRequest(ctx, http.MethodGet, "/clouds/"+resourceReference+"/snapshots",
		url.Values{"page": {"1"}, "per_page": {"9999"}}, nil)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return nil, readStatusError(response.statusCode)
	}
	entries, meta, err := decodeObjectPage(response.body)
	if err != nil {
		return nil, errInvalidResponse
	}
	defer clearRawObjects(entries)
	defer clearRawObject(meta)
	if len(entries) >= 9999 {
		return nil, errInvalidResponse
	}
	items := make([]resourcecap.AccessoryItem, 0, len(entries))
	for _, entry := range entries {
		reference, ok := firstReference(entry, "id")
		kindValue, kindOK := stringScalar(entry["type"])
		if !ok || !kindOK || (kindValue != "snap" && kindValue != "backup") {
			return nil, errInvalidResponse
		}
		name, _ := stringScalar(entry["name"])
		if name != "" && !validSnapshotName(name) {
			return nil, errInvalidResponse
		}
		kind := resourcecap.AccessoryKindSnapshot
		if kindValue == "backup" {
			kind = resourcecap.AccessoryKindBackup
		}
		projection, projectErr := projectScalars(entry, []string{"status", "size", "created_at"})
		if projectErr != nil {
			return nil, errInvalidResponse
		}
		items = append(items, resourcecap.AccessoryItem{Kind: kind, ExternalRef: reference,
			DisplayName: name, State: resourcecap.AccessoryStateUnknown, Data: projection})
	}
	return items, nil
}

func (p *Provider) readNATAccessories(ctx context.Context, resourceReference string) ([]resourcecap.AccessoryItem, error) {
	entries, err := p.readNATForwardings(ctx, resourceReference)
	if err != nil {
		return nil, err
	}
	items := make([]resourcecap.AccessoryItem, 0, len(entries))
	for _, entry := range entries {
		protocol := map[string]string{"1": "tcp", "2": "udp", "3": "tcp_udp"}[entry.protocol]
		projection, marshalErr := json.Marshal(map[string]string{"external_port": entry.externalPort,
			"internal_port": entry.internalPort, "protocol": protocol})
		if marshalErr != nil {
			return nil, errInvalidResponse
		}
		items = append(items, resourcecap.AccessoryItem{Kind: resourcecap.AccessoryKindNAT,
			ExternalRef: entry.reference, DisplayName: entry.name, State: resourcecap.AccessoryStateActive, Data: projection})
	}
	return items, nil
}

func (p *Provider) readSecurityGroupAccessories(ctx context.Context, detail accessoryDetail) ([]resourcecap.AccessoryItem, error) {
	response, err := p.authenticatedRequest(ctx, http.MethodGet, "/security_groups", url.Values{
		"list_type": {"all"}, "type": {detail.resourceType}, "user": {detail.userReference},
	}, nil)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return nil, readStatusError(response.statusCode)
	}
	entries, meta, err := decodeObjectPage(response.body)
	if err != nil {
		return nil, errInvalidResponse
	}
	defer clearRawObjects(entries)
	defer clearRawObject(meta)
	if len(entries) > maximumOwnershipCandidateCount {
		return nil, errInvalidResponse
	}
	items := make([]resourcecap.AccessoryItem, 0, len(entries))
	attached := 0
	for _, entry := range entries {
		reference, ok := firstReference(entry, "id")
		itemType, typeOK := stringScalar(entry["type"])
		name, nameOK := stringScalar(entry["name"])
		if !ok || !typeOK || itemType != detail.resourceType || !nameOK || !validSnapshotName(name) {
			return nil, errInvalidResponse
		}
		state := resourcecap.AccessoryStateAvailable
		if reference == detail.securityGroupReference {
			state = resourcecap.AccessoryStateAttached
			attached++
		}
		items = append(items, resourcecap.AccessoryItem{Kind: resourcecap.AccessoryKindSecurityGroup,
			ExternalRef: reference, DisplayName: name, State: state, Data: json.RawMessage(`{}`)})
	}
	if detail.securityGroupReference != "" && detail.securityGroupReference != "0" && attached != 1 {
		return nil, errInvalidResponse
	}
	return items, nil
}

func (p *Provider) readVirtualMediaAccessories(ctx context.Context, detail accessoryDetail) ([]resourcecap.AccessoryItem, error) {
	response, err := p.authenticatedRequest(ctx, http.MethodGet, "/node_isos",
		url.Values{"id": {detail.nodeReference}, "type": {"node"}}, nil)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return nil, readStatusError(response.statusCode)
	}
	root, err := decodeObject(response.body)
	if err != nil {
		return nil, errInvalidResponse
	}
	defer clearRawObject(root)
	groups, err := decodeRawObjectArray(root["data"])
	if err != nil || len(groups) > maximumOwnershipCandidateCount {
		clearRawObjects(groups)
		return nil, errInvalidResponse
	}
	defer clearRawObjects(groups)
	items := make([]resourcecap.AccessoryItem, 0)
	mounted := 0
	for _, group := range groups {
		entries, decodeErr := decodeRawObjectArray(group["info"])
		if decodeErr != nil {
			return nil, errInvalidResponse
		}
		for _, entry := range entries {
			if len(items) >= maximumOwnershipCandidateCount {
				clearRawObjects(entries)
				return nil, errInvalidResponse
			}
			reference, ok := firstReference(entry, "id")
			name, nameOK := stringScalar(entry["name"])
			if !ok || !nameOK || !validSnapshotName(name) {
				clearRawObjects(entries)
				return nil, errInvalidResponse
			}
			state := resourcecap.AccessoryStateAvailable
			if reference == detail.isoReference {
				state = resourcecap.AccessoryStateMounted
				mounted++
			}
			items = append(items, resourcecap.AccessoryItem{Kind: resourcecap.AccessoryKindVirtualMedia,
				ExternalRef: reference, DisplayName: name, State: state, Data: json.RawMessage(`{}`)})
		}
		clearRawObjects(entries)
	}
	if detail.isoReference != "" && detail.isoReference != "0" && mounted != 1 {
		return nil, errInvalidResponse
	}
	return items, nil
}
