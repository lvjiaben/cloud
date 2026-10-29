// This file validates merchant upgrade limits and rejects changes to disabled
// fields before any upgrade invoice or supplier operation can be created.

package configurationcontract

import (
	"bytes"
	"encoding/json"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"slices"
	"strconv"
)

func validateUpgradeMetadata(field *capmodel.ConfigurationFieldDeclaration, p Presentation) error {
	if p.UpgradeMinimum != "" || p.UpgradeMaximum != "" {
		if field == nil || field.Type != capmodel.ConfigurationFieldInteger {
			return gerror.New("upgrade bounds require a numeric field")
		}
		var minimum, maximum *int64
		for _, item := range []struct {
			value, technical string
			lower            bool
		}{
			{p.UpgradeMinimum, field.Minimum, true}, {p.UpgradeMaximum, field.Maximum, false},
		} {
			if item.value == "" {
				continue
			}
			number, err := strconv.ParseInt(item.value, 10, 64)
			if err != nil || strconv.FormatInt(number, 10) != item.value {
				return gerror.New("upgrade bound is invalid")
			}
			if item.technical != "" {
				bound, e := strconv.ParseInt(item.technical, 10, 64)
				if e != nil || item.lower && number < bound || !item.lower && number > bound {
					return gerror.New("upgrade bound exceeds sellable range")
				}
			}
			if item.lower {
				minimum = &number
			} else {
				maximum = &number
			}
		}
		if minimum != nil && maximum != nil && *minimum > *maximum {
			return gerror.New("upgrade range is reversed")
		}
	}
	if len(p.UpgradeCandidateValues) > 0 {
		if field == nil || field.Type != capmodel.ConfigurationFieldSingleSelect || len(p.UpgradeCandidateValues) > 512 {
			return gerror.New("upgrade choices require a single choice field")
		}
		seen := map[string]bool{}
		for _, value := range p.UpgradeCandidateValues {
			if value == "" || len(value) > 256 || seen[value] {
				return gerror.New("upgrade choice is invalid")
			}
			seen[value] = true
			if field.DynamicCandidates == nil && !slices.ContainsFunc(field.StaticCandidates, func(candidate *capmodel.ConfigurationCandidate) bool {
				return candidate != nil && candidate.Value == value
			}) {
				return gerror.New("upgrade choice is outside sellable choices")
			}
		}
	}
	return nil
}

// ValidateUpgradeChanges applies the saved switches and target ranges to the
// actual changed values. Unchanged fields may retain values outside a new range.
func ValidateUpgradeChanges(currentRaw, targetRaw json.RawMessage, presentations map[string]Presentation) error {
	var current, target map[string]json.RawMessage
	if json.Unmarshal(currentRaw, &current) != nil || current == nil || json.Unmarshal(targetRaw, &target) != nil || target == nil {
		return gerror.New("upgrade configuration is invalid")
	}
	keys := map[string]bool{}
	for key := range current {
		keys[key] = true
	}
	for key := range target {
		keys[key] = true
	}
	for key := range keys {
		if bytes.Equal(bytes.TrimSpace(current[key]), bytes.TrimSpace(target[key])) {
			continue
		}
		p := presentations[key]
		if p.UpgradeEnabled != nil && !*p.UpgradeEnabled {
			return gerror.New("configuration upgrade is disabled for this field")
		}
		if p.UpgradeMinimum != "" || p.UpgradeMaximum != "" {
			var number json.Number
			if json.Unmarshal(target[key], &number) != nil {
				return gerror.New("upgrade target must be numeric")
			}
			value, err := number.Int64()
			if err != nil {
				return gerror.New("upgrade target must be an integer")
			}
			if p.UpgradeMinimum != "" {
				minimum, err := strconv.ParseInt(p.UpgradeMinimum, 10, 64)
				if err != nil || value < minimum {
					return gerror.New("upgrade target is below the allowed range")
				}
			}
			if p.UpgradeMaximum != "" {
				maximum, err := strconv.ParseInt(p.UpgradeMaximum, 10, 64)
				if err != nil || value > maximum {
					return gerror.New("upgrade target exceeds the allowed range")
				}
			}
		}
		if len(p.UpgradeCandidateValues) > 0 {
			var choice string
			if json.Unmarshal(target[key], &choice) != nil || !slices.Contains(p.UpgradeCandidateValues, choice) {
				return gerror.New("upgrade target choice is not allowed")
			}
		}
	}
	return nil
}
