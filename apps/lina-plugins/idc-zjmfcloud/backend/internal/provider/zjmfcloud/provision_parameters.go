// This file maps the server owner's secret-free provision plan to ZJMFCloud v1 parameters.

package zjmfcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// directProvisionMode identifies the owner-created route using frozen upstream IDs.
const directProvisionMode = "direct"

var selectorValuePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type provisionSelector struct {
	Mode       string            `json:"mode,omitempty"`
	Provider   string            `json:"provider"`
	Areas      map[string]string `json:"areas"`
	Nodes      map[string]string `json:"nodes"`
	NodeGroups map[string]string `json:"node_groups"`
	Images     map[string]string `json:"images"`
	Defaults   provisionDefaults `json:"defaults"`
}

type provisionDefaults struct {
	NetworkType  string `json:"network_type"`
	IPGroup      string `json:"ip_group"`
	TrafficType  int64  `json:"traffic_type"`
	NodePriority int64  `json:"node_priority"`
}

type provisionValues struct {
	region, line, datacenter, image string
	cpu, memory                     int64
	systemDisk                      int64
	dataDisk                        int64
	ip                              int64
	bandwidth                       int64
	traffic                         int64
	unlimitedTraffic                bool
	quotas                          map[resourcecap.ServerProvisionComponentType]resourcecap.ServerProvisionComponent
}

// BuildProvisionParameters translates one reviewed server plan without
// receiving credentials, secret handles, an HTTP client, or owner persistence.
func BuildProvisionParameters(
	ctx context.Context,
	plan resourcecap.ServerProvisionPlan,
) (json.RawMessage, error) {
	if ctx == nil || ctx.Err() != nil || plan.ExternalPrincipalRef == "" {
		return nil, errInvalidConfig
	}
	selector, err := decodeProvisionSelector(plan.Selector)
	if err != nil || selector.Provider != ProviderID || !strictPositiveDecimal(plan.ExternalPrincipalRef) {
		return nil, errInvalidConfig
	}
	values, err := collectProvisionValues(plan.Components)
	if err != nil || values.cpu == 0 || values.memory == 0 || values.image == "" {
		return nil, errInvalidConfig
	}
	if selector.Mode == directProvisionMode {
		selector.Areas = map[string]string{values.region: values.region}
		selector.Nodes = map[string]string{values.line: values.line}
		selector.Images = map[string]string{values.image: values.image}
		selector.NodeGroups = map[string]string{values.datacenter: values.datacenter}
	}
	parameters := map[string]any{
		"num": 1, "cpu": values.cpu, "memory": values.memory,
		"os":     lookupNumeric(selector.Images, values.image),
		"client": plan.ExternalPrincipalRef, "hostname": plan.Hostname,
	}
	if parameters["os"] == "" {
		return nil, errInvalidConfig
	}
	if err = putMappedParameter(parameters, "area", selector.Areas, values.region); err != nil {
		return nil, err
	}
	if err = putMappedParameter(parameters, "node", selector.Nodes, values.line); err != nil {
		return nil, err
	}
	if err = putMappedParameter(parameters, "node_group", selector.NodeGroups, values.datacenter); err != nil {
		return nil, err
	}
	if err = appendProvisionDefaults(parameters, selector.Defaults); err != nil {
		return nil, err
	}
	if values.systemDisk > 0 {
		parameters["system_disk_size"] = values.systemDisk
	}
	// A zero capacity means no data disk; keep the upstream default in that
	// case instead of translating it into an unlimited resource.
	if values.dataDisk > 0 {
		parameters["data_disk_size"] = values.dataDisk
	}
	if values.ip > 0 {
		parameters["ip_num"] = values.ip
	}
	appendProvisionQuotas(parameters, values.quotas)
	if values.bandwidth > 0 {
		parameters["in_bw"] = values.bandwidth
		parameters["out_bw"] = values.bandwidth
	}
	if values.unlimitedTraffic || values.traffic > 0 {
		trafficType := selector.Defaults.TrafficType
		if trafficType == 0 {
			trafficType = 3
		}
		if trafficType < 1 || trafficType > 3 {
			return nil, errInvalidConfig
		}
		parameters["traffic_type"] = trafficType
		parameters["traffic_quota"] = values.traffic
	}
	if err = appendFrozenNetworkParameters(parameters, plan); err != nil {
		return nil, err
	}
	if err = appendFrozenAdvancedParameters(parameters, plan); err != nil {
		return nil, err
	}
	// Match the documented finance connector: choose the reset day when
	// preparing the create request. The resource owner freezes the resulting
	// native parameters before outbound submission and retry handling.
	if err = appendTrafficResetDay(parameters, time.Now().Day()); err != nil {
		return nil, err
	}
	custom, err := productfulfillmentcap.CustomParameters(plan.ProductConfiguration)
	if err != nil {
		return nil, errInvalidConfig
	}
	if len(custom) > 0 {
		parameters[productfulfillmentcap.CustomParametersKey] = custom
	}
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return nil, gerror.Wrap(err, "encode ZJMFCloud provisioning parameters")
	}
	return encoded, nil
}

func decodeProvisionSelector(payload json.RawMessage) (provisionSelector, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var envelope struct {
		Provision json.RawMessage `json:"provision"`
	}
	if err := decoder.Decode(&envelope); err != nil || len(envelope.Provision) == 0 {
		return provisionSelector{}, errInvalidConfig
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return provisionSelector{}, errInvalidConfig
	}
	strict := json.NewDecoder(bytes.NewReader(envelope.Provision))
	strict.DisallowUnknownFields()
	var selector provisionSelector
	if err := strict.Decode(&selector); err != nil {
		return provisionSelector{}, errInvalidConfig
	}
	if err := strict.Decode(&struct{}{}); err != io.EOF {
		return provisionSelector{}, errInvalidConfig
	}
	return normalizeProvisionSelectorMode(selector)
}

// normalizeProvisionSelectorMode keeps direct routing exclusive of merchant mappings.
func normalizeProvisionSelectorMode(selector provisionSelector) (provisionSelector, error) {
	switch selector.Mode {
	case "":
		return selector, nil
	case directProvisionMode:
		if selector.Provider != "" || len(selector.Areas) > 0 || len(selector.Nodes) > 0 || len(selector.NodeGroups) > 0 || len(selector.Images) > 0 || selector.Defaults != (provisionDefaults{}) {
			return provisionSelector{}, errInvalidConfig
		}
		selector.Provider = ProviderID
		return selector, nil
	default:
		return provisionSelector{}, errInvalidConfig
	}
}

func collectProvisionValues(components []resourcecap.ServerProvisionComponent) (provisionValues, error) {
	values := provisionValues{quotas: make(map[resourcecap.ServerProvisionComponentType]resourcecap.ServerProvisionComponent, 4)}
	seen := make(map[resourcecap.ServerProvisionComponentType]struct{}, len(components))
	for _, component := range components {
		if _, exists := seen[component.Type]; exists {
			return provisionValues{}, errInvalidConfig
		}
		seen[component.Type] = struct{}{}
		quantity := component.SelectionQuantity
		var err error
		switch component.Type {
		case resourcecap.ServerProvisionComponentRegion:
			if quantity != 1 {
				return provisionValues{}, errInvalidConfig
			}
			values.region = component.Code
		case resourcecap.ServerProvisionComponentLine:
			if quantity != 1 {
				return provisionValues{}, errInvalidConfig
			}
			values.line = component.Code
		case resourcecap.ServerProvisionComponentDatacenter:
			if quantity != 1 {
				return provisionValues{}, errInvalidConfig
			}
			values.datacenter = component.Code
		case resourcecap.ServerProvisionComponentImage:
			if quantity != 1 {
				return provisionValues{}, errInvalidConfig
			}
			values.image = component.Code
		case resourcecap.ServerProvisionComponentCPU:
			values.cpu, err = safeMultiply(component.Cores, quantity)
		case resourcecap.ServerProvisionComponentMemory:
			values.memory, err = safeMultiply(component.Megabytes, quantity)
		case resourcecap.ServerProvisionComponentSystemDisk:
			if quantity != 1 {
				return provisionValues{}, errInvalidConfig
			}
			values.systemDisk = component.Gigabytes
		case resourcecap.ServerProvisionComponentIP:
			if component.Family != "ipv4" {
				return provisionValues{}, errInvalidConfig
			}
			values.ip, err = safeMultiply(component.Count, quantity)
		case resourcecap.ServerProvisionComponentBackup,
			resourcecap.ServerProvisionComponentSnapshot,
			resourcecap.ServerProvisionComponentNATForward,
			resourcecap.ServerProvisionComponentNATWebsite:
			if err = validateProvisionQuota(component); err == nil {
				values.quotas[component.Type] = component
			}
		case resourcecap.ServerProvisionComponentBandwidth:
			values.bandwidth, err = safeMultiply(component.MegabitsPerSecond, quantity)
		case resourcecap.ServerProvisionComponentTraffic:
			values.unlimitedTraffic = component.Unlimited
			if !component.Unlimited {
				values.traffic, err = safeMultiply(component.Gigabytes, quantity)
			}
		case resourcecap.ServerProvisionComponentDataDisk:
			// This release supports one capacity-only disk. Storage/media
			// selectors require separately declared upstream evidence.
			if quantity != 1 || component.Gigabytes < 0 || component.Gigabytes > 1048576 ||
				component.Media != "" || component.Code != "" || component.ModelCode != "" {
				return provisionValues{}, errInvalidConfig
			}
			values.dataDisk = component.Gigabytes
		case resourcecap.ServerProvisionComponentOther:
			return provisionValues{}, errInvalidConfig
		default:
			return provisionValues{}, errInvalidConfig
		}
		if err != nil {
			return provisionValues{}, errInvalidConfig
		}
	}
	return values, nil
}

func safeMultiply(left, right int64) (int64, error) {
	if left < 0 || right < 1 || (left > 0 && right > (1<<63-1)/left) {
		return 0, errInvalidConfig
	}
	return left * right, nil
}

func putMappedParameter(parameters map[string]any, name string, mapping map[string]string, code string) error {
	if code == "" {
		return nil
	}
	value := lookupNumeric(mapping, code)
	if value == "" {
		return errInvalidConfig
	}
	parameters[name] = value
	return nil
}

func lookupNumeric(mapping map[string]string, code string) string {
	value := mapping[code]
	if !strictPositiveDecimal(value) {
		return ""
	}
	return value
}

func strictPositiveDecimal(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 63)
	return err == nil && parsed > 0 && value == strconv.FormatUint(parsed, 10)
}

// validateProvisionQuota rejects multiplied or contradictory quota facts.
func validateProvisionQuota(component resourcecap.ServerProvisionComponent) error {
	if component.SelectionQuantity != 1 || component.Count < 0 || (component.Unlimited && component.Count != 0) {
		return errInvalidConfig
	}
	return nil
}

// appendProvisionQuotas translates at most four owner quota kinds. Omitted
// kinds retain the legacy supplier behavior; explicit disabled is never zero.
func appendProvisionQuotas(parameters map[string]any, quotas map[resourcecap.ServerProvisionComponentType]resourcecap.ServerProvisionComponent) {
	for kind, component := range quotas {
		key := ""
		switch kind {
		case resourcecap.ServerProvisionComponentBackup:
			key = "backup_num"
		case resourcecap.ServerProvisionComponentSnapshot:
			key = "snap_num"
		case resourcecap.ServerProvisionComponentNATForward:
			key = "nat_acl_limit"
		case resourcecap.ServerProvisionComponentNATWebsite:
			key = "nat_web_limit"
		}
		switch {
		case component.Unlimited:
			parameters[key] = int64(0)
		case component.Count == 0:
			parameters[key] = int64(-1)
		default:
			parameters[key] = component.Count
		}
	}
}

// appendFrozenNetworkParameters applies optional connector-owned network fields
// from the frozen package, then the frozen purchase. Explicit zero IPv4 is
// preserved; it must never fall back to the supplier's default address count.
func appendFrozenNetworkParameters(parameters map[string]any, plan resourcecap.ServerProvisionPlan) error {
	for _, payload := range []json.RawMessage{plan.ProductConfiguration, plan.CustomerConfiguration} {
		if len(bytes.TrimSpace(payload)) == 0 {
			continue
		}
		var network struct {
			IPCount  json.Number `json:"ip_count"`
			Inbound  json.Number `json:"inbound_bandwidth"`
			Outbound json.Number `json:"outbound_bandwidth"`
		}
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.UseNumber()
		if decoder.Decode(&network) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return errInvalidConfig
		}
		for _, field := range []struct {
			key              string
			value            json.Number
			minimum, maximum int64
		}{
			{"ip_num", network.IPCount, 0, 1024},
			{"in_bw", network.Inbound, 1, 1000000},
			{"out_bw", network.Outbound, 1, 1000000},
		} {
			if field.value == "" {
				continue
			}
			value, parseErr := field.value.Int64()
			if parseErr != nil || field.value.String() != strconv.FormatInt(value, 10) || value < field.minimum || value > field.maximum {
				return errInvalidConfig
			}
			parameters[field.key] = value
		}
	}
	return nil
}

// appendProvisionDefaults checks the configured selector's optional native
// defaults before adding them; customer choices are applied separately later.
func appendProvisionDefaults(parameters map[string]any, defaults provisionDefaults) error {
	if defaults.NetworkType != "" {
		if !selectorValuePattern.MatchString(defaults.NetworkType) {
			return errInvalidConfig
		}
		parameters["network_type"] = defaults.NetworkType
	}
	if defaults.IPGroup != "" {
		if !strictPositiveDecimal(defaults.IPGroup) {
			return errInvalidConfig
		}
		parameters["ip_group"] = defaults.IPGroup
	}
	if defaults.NodePriority != 0 {
		if defaults.NodePriority < 1 || defaults.NodePriority > 4 {
			return errInvalidConfig
		}
		parameters["node_priority"] = defaults.NodePriority
	}
	return nil
}
