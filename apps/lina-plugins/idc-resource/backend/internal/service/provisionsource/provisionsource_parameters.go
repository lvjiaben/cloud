package provisionsource

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strconv"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// ManagedProvisionParameterBuilder invokes the exact frozen managed connector
// without granting it credentials or supplier-network access.
type ManagedProvisionParameterBuilder interface {
	Build(context.Context, string, string, resourcecap.ServerProvisionPlan) (json.RawMessage, error)
}

type parameterConstraintWire struct {
	Name          string   `json:"name"`
	Required      bool     `json:"required"`
	AllowedValues []string `json:"allowedValues"`
	Minimum       *int64   `json:"minimum"`
	Maximum       *int64   `json:"maximum"`
	Pattern       string   `json:"pattern"`
}

func validateCapabilityParameters(constraintsJSON string, parameters json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewBufferString(constraintsJSON))
	decoder.DisallowUnknownFields()
	var wires []parameterConstraintWire
	if err := decoder.Decode(&wires); err != nil || len(wires) > 64 {
		return provisionresourcecap.ErrCapabilityUnavailable
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return provisionresourcecap.ErrCapabilityUnavailable
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(parameters, &values) != nil {
		return provisionresourcecap.ErrParametersUnavailable
	}
	for _, wire := range wires {
		constraint := resourcecap.ParameterConstraint{
			Name: wire.Name, Required: wire.Required, AllowedValues: wire.AllowedValues,
			Minimum: wire.Minimum, Maximum: wire.Maximum, Pattern: wire.Pattern,
		}
		if constraint.Validate() != nil {
			return provisionresourcecap.ErrCapabilityUnavailable
		}
		raw, exists := values[constraint.Name]
		if !exists {
			if constraint.Required {
				return provisionresourcecap.ErrParametersUnavailable
			}
			continue
		}
		value, numeric, ok := scalarConstraintValue(raw)
		if !ok {
			return provisionresourcecap.ErrParametersUnavailable
		}
		if len(constraint.AllowedValues) > 0 && !containsString(constraint.AllowedValues, value) {
			return provisionresourcecap.ErrParametersUnavailable
		}
		if constraint.Minimum != nil && (!numeric || numericValue(value) < *constraint.Minimum) {
			return provisionresourcecap.ErrParametersUnavailable
		}
		if constraint.Maximum != nil && (!numeric || numericValue(value) > *constraint.Maximum) {
			return provisionresourcecap.ErrParametersUnavailable
		}
		if constraint.Pattern != "" {
			pattern, err := regexp.Compile(constraint.Pattern)
			if err != nil || !pattern.MatchString(value) {
				return provisionresourcecap.ErrParametersUnavailable
			}
		}
	}
	return nil
}

func scalarConstraintValue(raw json.RawMessage) (string, bool, bool) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		_, err := strconv.ParseInt(text, 10, 64)
		return text, err == nil, text != ""
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&number) != nil {
		return "", false, false
	}
	if _, err := number.Int64(); err != nil {
		return "", false, false
	}
	return number.String(), true, true
}

func numericValue(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
