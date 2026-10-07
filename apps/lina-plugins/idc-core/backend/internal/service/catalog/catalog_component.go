// This file normalizes first-release server component values into canonical JSON.

package catalog

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

// ComponentType identifies a typed server selector or internal resource.
type ComponentType string

// Supported first-release component types.
const (
	ComponentRegion     ComponentType = "region"
	ComponentLine       ComponentType = "line"
	ComponentDatacenter ComponentType = "datacenter"
	ComponentCPU        ComponentType = "cpu"
	ComponentMemory     ComponentType = "memory"
	ComponentSystemDisk ComponentType = "system_disk"
	ComponentDataDisk   ComponentType = "data_disk"
	ComponentIP         ComponentType = "ip"
	ComponentBackup     ComponentType = "backup"
	ComponentImage      ComponentType = "image"
	ComponentTraffic    ComponentType = "traffic"
	ComponentBandwidth  ComponentType = "bandwidth"
	ComponentOther      ComponentType = "other"
)

// ComponentValueInput is a typed union. Fields outside the selected component
// type are rejected instead of being silently persisted as arbitrary JSON.
type ComponentValueInput struct {
	Code              string
	Cores             *int
	Megabytes         *int
	Gigabytes         *int
	Media             string
	MegabitsPerSecond *int
	Unlimited         *bool
	Count             *int
	Family            string
	Quantity          *int
}

type componentValueWire struct {
	Code              string `json:"code"`
	ModelCode         string `json:"model_code"`
	Cores             *int   `json:"cores"`
	Megabytes         *int   `json:"megabytes"`
	Gigabytes         *int   `json:"gigabytes"`
	Media             string `json:"media"`
	MegabitsPerSecond *int   `json:"megabits_per_second"`
	Unlimited         *bool  `json:"unlimited"`
	Count             *int   `json:"count"`
	Family            string `json:"family"`
	Quantity          *int   `json:"quantity"`
}

var componentCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// NormalizeComponentValue returns deterministic JSON containing only the
// selected type's authoritative fields.
func NormalizeComponentValue(componentType ComponentType, input ComponentValueInput) (string, error) {
	value, valid := normalizedComponentValue(componentType, input)
	if !valid {
		return "", bizerr.NewCode(CodeComponentInvalid)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode normalized catalog component")
	}
	return string(encoded), nil
}

// ValidateNormalizedComponentValue rejects unknown, ambiguous, or noncanonical
// stored JSON and proves it matches the same typed union accepted on writes.
func ValidateNormalizedComponentValue(componentType ComponentType, value string) error {
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	var wire componentValueWire
	if err := decoder.Decode(&wire); err != nil {
		return bizerr.NewCode(CodeComponentInvalid)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return bizerr.NewCode(CodeComponentInvalid)
	}
	code := wire.Code
	if componentType == ComponentCPU {
		if wire.Code != "" {
			return bizerr.NewCode(CodeComponentInvalid)
		}
		code = wire.ModelCode
	} else if wire.ModelCode != "" {
		return bizerr.NewCode(CodeComponentInvalid)
	}
	normalized, err := NormalizeComponentValue(componentType, ComponentValueInput{
		Code: code, Cores: wire.Cores, Megabytes: wire.Megabytes,
		Gigabytes: wire.Gigabytes, Media: wire.Media,
		MegabitsPerSecond: wire.MegabitsPerSecond, Unlimited: wire.Unlimited,
		Count: wire.Count, Family: wire.Family, Quantity: wire.Quantity,
	})
	if err != nil {
		return err
	}
	var decoded any
	if err = json.Unmarshal([]byte(value), &decoded); err != nil {
		return bizerr.NewCode(CodeComponentInvalid)
	}
	canonical, err := json.Marshal(decoded)
	if err != nil || string(canonical) != normalized {
		return bizerr.NewCode(CodeComponentInvalid)
	}
	return nil
}

func normalizedComponentValue(componentType ComponentType, input ComponentValueInput) (map[string]any, bool) {
	switch componentType {
	case ComponentRegion, ComponentLine, ComponentDatacenter, ComponentImage:
		return normalizedCodeValue(input)
	case ComponentCPU:
		return normalizedCPUValue(input)
	case ComponentMemory:
		return normalizedMemoryValue(input)
	case ComponentSystemDisk, ComponentDataDisk:
		return normalizedDiskValue(input)
	case ComponentBandwidth:
		return normalizedBandwidthValue(input)
	case ComponentTraffic:
		return normalizedTrafficValue(input)
	case ComponentIP:
		return normalizedIPValue(input)
	case ComponentBackup:
		return normalizedBackupValue(input)
	case ComponentOther:
		return normalizedOtherValue(input)
	default:
		return nil, false
	}
}

func normalizedCodeValue(input ComponentValueInput) (map[string]any, bool) {
	if !validCode(input.Code) || !onlyCode(input) {
		return nil, false
	}
	return map[string]any{"code": input.Code}, true
}

func normalizedCPUValue(input ComponentValueInput) (map[string]any, bool) {
	if input.Cores == nil || *input.Cores < 1 || *input.Cores > 1024 ||
		(input.Code != "" && !validCode(input.Code)) || !onlyCPU(input) {
		return nil, false
	}
	value := map[string]any{"cores": *input.Cores}
	if input.Code != "" {
		value["model_code"] = input.Code
	}
	return value, true
}

func normalizedMemoryValue(input ComponentValueInput) (map[string]any, bool) {
	if input.Megabytes == nil || *input.Megabytes < 128 || *input.Megabytes > 16777216 ||
		!onlyMemory(input) {
		return nil, false
	}
	return map[string]any{"megabytes": *input.Megabytes}, true
}

func normalizedDiskValue(input ComponentValueInput) (map[string]any, bool) {
	if input.Gigabytes == nil || *input.Gigabytes < 1 || *input.Gigabytes > 1000000 ||
		(input.Media != "hdd" && input.Media != "ssd" && input.Media != "nvme") ||
		!onlyDisk(input) {
		return nil, false
	}
	return map[string]any{"gigabytes": *input.Gigabytes, "media": input.Media}, true
}

func normalizedBandwidthValue(input ComponentValueInput) (map[string]any, bool) {
	if input.MegabitsPerSecond == nil || *input.MegabitsPerSecond < 1 ||
		*input.MegabitsPerSecond > 1000000 || !onlyBandwidth(input) {
		return nil, false
	}
	return map[string]any{"megabits_per_second": *input.MegabitsPerSecond}, true
}

func normalizedTrafficValue(input ComponentValueInput) (map[string]any, bool) {
	if !onlyTraffic(input) {
		return nil, false
	}
	if input.Unlimited != nil && *input.Unlimited && input.Gigabytes == nil {
		return map[string]any{"unlimited": true}, true
	}
	if input.Unlimited == nil && input.Gigabytes != nil &&
		*input.Gigabytes >= 1 && *input.Gigabytes <= 1000000000 {
		return map[string]any{"gigabytes": *input.Gigabytes}, true
	}
	return nil, false
}

func normalizedIPValue(input ComponentValueInput) (map[string]any, bool) {
	if input.Count == nil || *input.Count < 1 || *input.Count > 65535 ||
		(input.Family != "ipv4" && input.Family != "ipv6" && input.Family != "dual") ||
		!onlyIP(input) {
		return nil, false
	}
	return map[string]any{"count": *input.Count, "family": input.Family}, true
}

func normalizedBackupValue(input ComponentValueInput) (map[string]any, bool) {
	if input.Count == nil || *input.Count < 0 || *input.Count > 1000 || !onlyBackup(input) {
		return nil, false
	}
	return map[string]any{"count": *input.Count}, true
}

func normalizedOtherValue(input ComponentValueInput) (map[string]any, bool) {
	if !validCode(input.Code) || input.Quantity == nil || *input.Quantity < 1 ||
		*input.Quantity > 100000 || !onlyOther(input) {
		return nil, false
	}
	return map[string]any{"code": input.Code, "quantity": *input.Quantity}, true
}

func validCode(value string) bool {
	return componentCodePattern.MatchString(value)
}

func onlyCode(value ComponentValueInput) bool {
	return value.Cores == nil && value.Megabytes == nil && value.Gigabytes == nil &&
		value.Media == "" && value.MegabitsPerSecond == nil && value.Unlimited == nil &&
		value.Count == nil && value.Family == "" && value.Quantity == nil
}

func onlyCPU(value ComponentValueInput) bool {
	return value.Megabytes == nil && value.Gigabytes == nil && value.Media == "" &&
		value.MegabitsPerSecond == nil && value.Unlimited == nil && value.Count == nil &&
		value.Family == "" && value.Quantity == nil
}

func onlyMemory(value ComponentValueInput) bool {
	return value.Code == "" && value.Cores == nil && value.Gigabytes == nil && value.Media == "" &&
		value.MegabitsPerSecond == nil && value.Unlimited == nil && value.Count == nil &&
		value.Family == "" && value.Quantity == nil
}

func onlyDisk(value ComponentValueInput) bool {
	return value.Code == "" && value.Cores == nil && value.Megabytes == nil &&
		value.MegabitsPerSecond == nil && value.Unlimited == nil && value.Count == nil &&
		value.Family == "" && value.Quantity == nil
}

func onlyBandwidth(value ComponentValueInput) bool {
	return value.Code == "" && value.Cores == nil && value.Megabytes == nil &&
		value.Gigabytes == nil && value.Media == "" && value.Unlimited == nil &&
		value.Count == nil && value.Family == "" && value.Quantity == nil
}

func onlyTraffic(value ComponentValueInput) bool {
	return value.Code == "" && value.Cores == nil && value.Megabytes == nil &&
		value.Media == "" && value.MegabitsPerSecond == nil && value.Count == nil &&
		value.Family == "" && value.Quantity == nil
}

func onlyIP(value ComponentValueInput) bool {
	return value.Code == "" && value.Cores == nil && value.Megabytes == nil &&
		value.Gigabytes == nil && value.Media == "" && value.MegabitsPerSecond == nil &&
		value.Unlimited == nil && value.Quantity == nil
}

func onlyBackup(value ComponentValueInput) bool {
	return value.Code == "" && value.Cores == nil && value.Megabytes == nil &&
		value.Gigabytes == nil && value.Media == "" && value.MegabitsPerSecond == nil &&
		value.Unlimited == nil && value.Family == "" && value.Quantity == nil
}

func onlyOther(value ComponentValueInput) bool {
	return value.Cores == nil && value.Megabytes == nil && value.Gigabytes == nil &&
		value.Media == "" && value.MegabitsPerSecond == nil && value.Unlimited == nil &&
		value.Count == nil && value.Family == ""
}
