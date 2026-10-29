package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strconv"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var safeCode = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)

type selectorEnvelope struct {
	Provision provisionSelector `json:"provision"`
}

type provisionSelector struct {
	Provider     string                    `json:"provider"`
	BillingCycle string                    `json:"billing_cycle"`
	Options      map[string]upstreamOption `json:"options"`
}

type upstreamOption struct {
	OptionID string            `json:"option_id"`
	Values   map[string]string `json:"values"`
}

type productConfiguration struct {
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

type customerConfiguration struct {
	OperatingSystem string `json:"operating_system"`
}

func BuildProvisionParameters(ctx context.Context, plan resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
	if ctx == nil || ctx.Err() != nil || plan.Validate() != nil {
		return nil, errInvalidConfig
	}
	selector, err := decodeSelector(plan.Selector)
	if err != nil || selector.Provider != ProviderID || !safeCode.MatchString(selector.BillingCycle) {
		return nil, errInvalidConfig
	}
	product, customer, err := decodeFrozenConfigurations(plan.ProductConfiguration, plan.CustomerConfiguration)
	if err != nil {
		return nil, errInvalidConfig
	}
	parameters := map[string]any{
		"workflow": "serialized_cart_v1", "pid": product.UpstreamProductID, "billingcycle": selector.BillingCycle,
		"host": plan.Hostname, "qty": 1,
	}
	configOptions := map[string]string{
		product.UpstreamRegionOptionID: product.Region,
		product.UpstreamLineOptionID:   product.Line,
		product.UpstreamOSOptionID:     customer.OperatingSystem,
	}
	if len(configOptions) != 3 {
		return nil, errInvalidConfig
	}
	for _, component := range plan.Components {
		switch component.Type {
		case resourcecap.ServerProvisionComponentRegion, resourcecap.ServerProvisionComponentLine,
			resourcecap.ServerProvisionComponentImage, resourcecap.ServerProvisionComponentOperatingSystem:
			continue
		}
		mapping, exists := selector.Options[string(component.Type)]
		if !exists {
			continue
		}
		if !positiveDecimal(mapping.OptionID) {
			return nil, errInvalidConfig
		}
		code, err := componentCode(component)
		if err != nil {
			return nil, err
		}
		value := mapping.Values[code]
		if value == "" || !safeCode.MatchString(value) {
			return nil, errInvalidConfig
		}
		if _, exists := configOptions[mapping.OptionID]; exists {
			return nil, errInvalidConfig
		}
		configOptions[mapping.OptionID] = value
	}
	if len(configOptions) == 0 {
		return nil, errInvalidConfig
	}
	parameters["configoption"] = configOptions
	return json.Marshal(parameters)
}

func decodeFrozenConfigurations(productRaw, customerRaw json.RawMessage) (productConfiguration, customerConfiguration, error) {
	var product productConfiguration
	var customer customerConfiguration
	if strictDecode(productRaw, &product) != nil || strictDecode(customerRaw, &customer) != nil ||
		!positiveDecimal(product.UpstreamProductID) || !positiveDecimal(product.UpstreamRegionOptionID) ||
		!positiveDecimal(product.UpstreamLineOptionID) || !positiveDecimal(product.UpstreamOSOptionID) ||
		!safeCode.MatchString(product.Region) || !safeCode.MatchString(product.Line) ||
		!safeCode.MatchString(customer.OperatingSystem) {
		return productConfiguration{}, customerConfiguration{}, errInvalidConfig
	}
	values := []struct {
		value   json.Number
		minimum int64
		maximum int64
	}{
		{product.CPU, 1, 1024}, {product.Memory, 128, 1048576},
		{product.SystemDisk, 1, 1048576}, {product.DataDisk, 0, 1048576},
		{product.Bandwidth, 1, 1000000},
	}
	for _, item := range values {
		value, err := item.value.Int64()
		if err != nil || value < item.minimum || value > item.maximum {
			return productConfiguration{}, customerConfiguration{}, errInvalidConfig
		}
	}
	return product, customer, nil
}

func strictDecode(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errInvalidConfig
	}
	return nil
}

func decodeSelector(raw json.RawMessage) (provisionSelector, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var envelope selectorEnvelope
	if decoder.Decode(&envelope) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return provisionSelector{}, errInvalidConfig
	}
	return envelope.Provision, nil
}

func componentCode(component resourcecap.ServerProvisionComponent) (string, error) {
	switch component.Type {
	case resourcecap.ServerProvisionComponentRegion, resourcecap.ServerProvisionComponentLine,
		resourcecap.ServerProvisionComponentDatacenter, resourcecap.ServerProvisionComponentImage,
		resourcecap.ServerProvisionComponentOperatingSystem:
		if component.SelectionQuantity != 1 || !safeCode.MatchString(component.Code) {
			return "", errInvalidConfig
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
		return "", errInvalidConfig
	}
}

func multiplied(value, quantity int64) (string, error) {
	if value < 0 || quantity < 1 || value > (1<<63-1)/quantity {
		return "", errInvalidConfig
	}
	return strconv.FormatInt(value*quantity, 10), nil
}

func positiveDecimal(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 63)
	return err == nil && parsed > 0 && value == strconv.FormatUint(parsed, 10)
}
