package v10

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var errInvalidPlan = errors.New("V10 provisioning plan is invalid")

type selectorEnvelope struct {
	Provision selector `json:"provision"`
}
type selector struct {
	Provider     string                    `json:"provider"`
	BillingCycle string                    `json:"billing_cycle"`
	Options      map[string]upstreamOption `json:"options"`
}
type upstreamOption struct {
	OptionID string            `json:"option_id"`
	Values   map[string]string `json:"values"`
}
type frozenProductConfiguration struct {
	UpstreamProductID      string      `json:"upstream_product_id"`
	UpstreamRegionOptionID string      `json:"upstream_region_option_id"`
	UpstreamLineOptionID   string      `json:"upstream_line_option_id"`
	UpstreamOSOptionID     string      `json:"upstream_os_option_id"`
	Region                 string      `json:"region"`
	Line                   string      `json:"line"`
	CPU                    json.Number `json:"cpu"`
	Memory                 json.Number `json:"memory"`
	SystemDisk             json.Number `json:"system_disk"`
	DataDisk               json.Number `json:"data_disk"`
	Bandwidth              json.Number `json:"bandwidth"`
}
type frozenCustomerConfiguration struct {
	OperatingSystem string `json:"operating_system"`
}

var safeCode = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)

// BuildProvisionParameters freezes the exact V10 console cart facts proven by
// the official supplier implementation. Authentication and transport remain in
// the resource owner's confidential outbound host service.
func BuildProvisionParameters(plan resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
	var selected selectorEnvelope
	var product frozenProductConfiguration
	var customer frozenCustomerConfiguration
	if plan.Validate() != nil || strictDecode(plan.Selector, &selected) != nil ||
		strictDecode(plan.ProductConfiguration, &product) != nil || strictDecode(plan.CustomerConfiguration, &customer) != nil ||
		selected.Provision.Provider != ProviderID || !safeCode.MatchString(selected.Provision.BillingCycle) ||
		!positiveDecimal(product.UpstreamProductID) || !positiveDecimal(product.UpstreamRegionOptionID) ||
		!positiveDecimal(product.UpstreamLineOptionID) || !positiveDecimal(product.UpstreamOSOptionID) ||
		!safeCode.MatchString(product.Region) || !safeCode.MatchString(product.Line) ||
		!safeCode.MatchString(customer.OperatingSystem) || !positiveDecimal(plan.ExternalPrincipalRef) {
		return nil, errInvalidPlan
	}
	configOption := map[string]json.RawMessage{}
	for key, value := range map[string]string{
		product.UpstreamRegionOptionID: product.Region,
		product.UpstreamLineOptionID:   product.Line,
		product.UpstreamOSOptionID:     customer.OperatingSystem,
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, errInvalidPlan
		}
		configOption[key] = encoded
	}
	for _, component := range plan.Components {
		mapping, exists := selected.Provision.Options[string(component.Type)]
		if !exists {
			continue
		}
		code, err := componentCode(component)
		if err != nil || !positiveDecimal(mapping.OptionID) || !safeCode.MatchString(mapping.Values[code]) {
			return nil, errInvalidPlan
		}
		if _, duplicate := configOption[mapping.OptionID]; duplicate {
			return nil, errInvalidPlan
		}
		encoded, err := json.Marshal(mapping.Values[code])
		if err != nil {
			return nil, errInvalidPlan
		}
		configOption[mapping.OptionID] = encoded
	}
	return json.Marshal(struct {
		Workflow           string                     `json:"workflow"`
		PID                string                     `json:"pid"`
		BillingCycle       string                     `json:"billingcycle"`
		ConfigOption       map[string]json.RawMessage `json:"configoption"`
		DownstreamClientID string                     `json:"downstream_client_id"`
		CreateSubAccount   int                        `json:"create_sub_account"`
		Hostname           string                     `json:"hostname,omitempty"`
	}{"console_cart_v1", product.UpstreamProductID, selected.Provision.BillingCycle, configOption, plan.ExternalPrincipalRef, 1, plan.Hostname})
}

func componentCode(component resourcecap.ServerProvisionComponent) (string, error) {
	switch component.Type {
	case resourcecap.ServerProvisionComponentRegion, resourcecap.ServerProvisionComponentLine,
		resourcecap.ServerProvisionComponentDatacenter, resourcecap.ServerProvisionComponentImage,
		resourcecap.ServerProvisionComponentOperatingSystem:
		if component.SelectionQuantity != 1 || !safeCode.MatchString(component.Code) {
			return "", errInvalidPlan
		}
		return component.Code, nil
	case resourcecap.ServerProvisionComponentCPU:
		return multiplied(component.Cores, component.SelectionQuantity)
	case resourcecap.ServerProvisionComponentMemory:
		return multiplied(component.Megabytes, component.SelectionQuantity)
	case resourcecap.ServerProvisionComponentSystemDisk, resourcecap.ServerProvisionComponentDataDisk:
		return multiplied(component.Gigabytes, component.SelectionQuantity)
	case resourcecap.ServerProvisionComponentBandwidth:
		return multiplied(component.MegabitsPerSecond, component.SelectionQuantity)
	default:
		return "", errInvalidPlan
	}
}

func multiplied(value, quantity int64) (string, error) {
	if value < 0 || quantity < 1 || value > (1<<63-1)/quantity {
		return "", errInvalidPlan
	}
	return strconv.FormatInt(value*quantity, 10), nil
}

func positiveDecimal(value string) bool {
	n, err := strconv.ParseUint(value, 10, 63)
	return err == nil && n > 0 && value == strconv.FormatUint(n, 10)
}

func strictDecode(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errInvalidPlan
	}
	return nil
}
