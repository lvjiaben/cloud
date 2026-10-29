// This file maps optional documented networking, login port and disk limits
// to administrator v1 fields. Historical frozen values remain untouched.

package zjmfcloud

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var diskPerformanceFields = map[string]string{
	"system_read_mbps": "system_read_bytes_sec", "system_write_mbps": "system_write_bytes_sec",
	"system_read_iops": "system_read_iops_sec", "system_write_iops": "system_write_iops_sec",
	"data_read_mbps": "data_read_bytes_sec", "data_write_mbps": "data_write_bytes_sec",
	"data_read_iops": "data_read_iops_sec", "data_write_iops": "data_write_iops_sec",
}

func appendFrozenAdvancedParameters(parameters map[string]any, plan resourcecap.ServerProvisionPlan) error {
	for _, payload := range []json.RawMessage{plan.ProductConfiguration, plan.CustomerConfiguration} {
		if len(bytes.TrimSpace(payload)) == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(payload, &fields) != nil {
			return errInvalidConfig
		}
		for _, key := range []string{"network_type", "port", "node_priority", "traffic_bill_type"} {
			if raw, exists := fields[key]; exists {
				var value string
				if json.Unmarshal(raw, &value) != nil {
					return errInvalidConfig
				}
				if value != "" {
					parameters[key] = value
				}
			}
		}
		for _, disk := range []string{"system", "data"} {
			if raw, exists := fields[disk+"_disk_io_limit"]; exists {
				var profile string
				if json.Unmarshal(raw, &profile) != nil {
					return errInvalidConfig
				}
				parts := strings.Split(profile, ",")
				if len(parts) != 4 {
					return errInvalidConfig
				}
				for i, suffix := range []string{"read_mbps", "write_mbps", "read_iops", "write_iops"} {
					if _, conflict := fields[disk+"_"+suffix]; conflict {
						return errInvalidConfig
					}
					n, err := strconv.ParseUint(parts[i], 10, 31)
					if err != nil || strconv.FormatUint(n, 10) != parts[i] {
						return errInvalidConfig
					}
					parameters[diskPerformanceFields[disk+"_"+suffix]] = int64(n)
				}
			}
		}
		for field, upstream := range diskPerformanceFields {
			if raw, exists := fields[field]; exists {
				var value json.Number
				if len(raw) == 0 || raw[0] == '"' || json.Unmarshal(raw, &value) != nil {
					return errInvalidConfig
				}
				n, err := value.Int64()
				if err != nil || n < 0 || n > 2147483647 || value.String() != strconv.FormatInt(n, 10) {
					return errInvalidConfig
				}
				// The official finance mapping submits MB/s numbers directly under the
				// v1 *_bytes_sec names; multiplying again would overstate the limit.
				parameters[upstream] = n
			}
		}
	}
	if parameters["network_type"] == "vpc" {
		// A new cloud receives its own deterministic network through the documented
		// cloud-create vpc_name field. No existing customer network is repurposed.
		sum := sha256.Sum256([]byte("cloud-vpc/v1\x00" + plan.ExternalPrincipalRef + "\x00" + plan.Hostname))
		parameters["vpc_name"] = "VPC-" + hex.EncodeToString(sum[:8])
	}
	form := url.Values{}
	for _, key := range []string{"network_type", "port", "node_priority", "traffic_bill_type"} {
		if value, present := parameters[key]; present {
			text, ok := value.(string)
			if !ok && key == "node_priority" {
				if n, numeric := value.(int64); numeric {
					text = strconv.FormatInt(n, 10)
					ok = true
				}
			}
			if !ok {
				return errInvalidConfig
			}
			form.Set(key, text)
		}
	}
	return validateAdvancedProvisionForm(form)
}

func validateAdvancedProvisionForm(form url.Values) error {
	if value := form.Get("reset_flow_day"); value != "" {
		day, err := strconv.ParseUint(value, 10, 8)
		if err != nil || day < 1 || day > 31 || strconv.FormatUint(day, 10) != value {
			return errInvalidConfig
		}
	}

	if value := form.Get("node_priority"); value != "" && value != "1" && value != "2" && value != "3" && value != "4" {
		return errInvalidConfig
	}
	if value := form.Get("traffic_bill_type"); value != "" && value != "month" && value != "last_30days" {
		return errInvalidConfig
	}
	if value := form.Get("network_type"); value != "" && value != "normal" && value != "vpc" {
		return errInvalidConfig
	}
	if value := form.Get("port"); value != "" && value != "auto" {
		n, err := strconv.ParseUint(value, 10, 16)
		if err != nil || n < 1 || strconv.FormatUint(n, 10) != value {
			return errInvalidConfig
		}
	}
	for _, key := range diskPerformanceFields {
		if value := form.Get(key); value != "" {
			n, err := strconv.ParseUint(value, 10, 31)
			if err != nil || strconv.FormatUint(n, 10) != value {
				return errInvalidConfig
			}
		}
	}
	return nil
}

// ValidateOptionalProvisionConfiguration is a pure pre-payment validation of
// optional connector-owned fields, without network, principals or persistence.
func ValidateOptionalProvisionConfiguration(payload json.RawMessage) error {
	parameters := map[string]any{}
	plan := resourcecap.ServerProvisionPlan{ProductConfiguration: payload}
	if err := appendFrozenNetworkParameters(parameters, plan); err != nil {
		return err
	}
	return appendFrozenAdvancedParameters(parameters, plan)
}

// ValidateOptionalProvisionChoices checks bounded alternatives in one pure call.
// The base already contains generation policy; choices may replace declared
// non-secret profile text only and cannot alter authentication or routing.
func ValidateOptionalProvisionChoices(base json.RawMessage, choices map[string][]json.RawMessage) error {
	if err := ValidateOptionalProvisionConfiguration(base); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(base, &fields) != nil {
		return errInvalidConfig
	}
	count := 0
	for key, values := range choices {
		if key != "system_disk_io_limit" && key != "data_disk_io_limit" && key != "port" {
			return errInvalidConfig
		}
		original, exists := fields[key]
		for _, value := range values {
			count++
			if count > 512 {
				return errInvalidConfig
			}
			fields[key] = value
			payload, err := json.Marshal(fields)
			if err != nil {
				return err
			}
			if err = ValidateOptionalProvisionConfiguration(payload); err != nil {
				return err
			}
		}
		if exists {
			fields[key] = original
		} else {
			delete(fields, key)
		}
	}
	return nil
}

// appendTrafficResetDay translates the merchant billing policy into the native
// v1 reset day. Calendar months reset on day 1; service months use creation day.
func appendTrafficResetDay(parameters map[string]any, creationDay int) error {
	raw, exists := parameters["traffic_bill_type"]
	if !exists {
		return nil
	}
	cycle, ok := raw.(string)
	if !ok || creationDay < 1 || creationDay > 31 {
		return errInvalidConfig
	}
	switch cycle {
	case "month":
		parameters["reset_flow_day"] = int64(1)
	case "last_30days":
		parameters["reset_flow_day"] = int64(creationDay)
	default:
		return errInvalidConfig
	}
	delete(parameters, "traffic_bill_type")
	return nil
}
