package finance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

const maxCandidateItems = 512

type CandidateRequest struct {
	SchemaVersion     string               `json:"schemaVersion"`
	CurrentSelections []CandidateSelection `json:"currentSelections"`
	Requests          []CandidateQuery     `json:"requests"`
}

type CandidateSelection struct {
	FieldKey string   `json:"fieldKey"`
	Values   []string `json:"values"`
}

type CandidateQuery struct {
	FieldKey       string   `json:"fieldKey"`
	ParentValues   []string `json:"parentValues"`
	SelectedValues []string `json:"selectedValues"`
}

type CandidateCall struct {
	FieldKey string
	Method   string
	Path     string
	Query    url.Values
}

var candidateFields = map[string]struct{}{
	"upstream_product_id": {}, "upstream_region_option_id": {}, "upstream_line_option_id": {},
	"upstream_os_option_id": {}, "region": {}, "line": {}, "operating_system": {},
}

func DecodeCandidateRequest(raw json.RawMessage) (CandidateRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request CandidateRequest
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		request.SchemaVersion != "v1" || len(request.Requests) == 0 || len(request.Requests) > 8 ||
		len(request.CurrentSelections) > 16 {
		return CandidateRequest{}, errInvalidConfig
	}
	seen := make(map[string]struct{}, len(request.Requests))
	for _, item := range request.Requests {
		if _, ok := candidateFields[item.FieldKey]; !ok || len(item.ParentValues) > 8 || len(item.SelectedValues) > maxCandidateItems {
			return CandidateRequest{}, errInvalidConfig
		}
		if _, duplicate := seen[item.FieldKey]; duplicate {
			return CandidateRequest{}, errInvalidConfig
		}
		seen[item.FieldKey] = struct{}{}
	}
	selectedFields := make(map[string]struct{}, len(request.CurrentSelections))
	for _, item := range request.CurrentSelections {
		if _, ok := candidateFields[item.FieldKey]; !ok || len(item.Values) > maxCandidateItems {
			return CandidateRequest{}, errInvalidConfig
		}
		if _, duplicate := selectedFields[item.FieldKey]; duplicate {
			return CandidateRequest{}, errInvalidConfig
		}
		selectedFields[item.FieldKey] = struct{}{}
		for _, value := range item.Values {
			if !safeCode.MatchString(value) {
				return CandidateRequest{}, errInvalidConfig
			}
		}
	}
	return request, nil
}

func CandidateCalls(request CandidateRequest) ([]CandidateCall, error) {
	selections := make(map[string]string, len(request.CurrentSelections))
	for _, selection := range request.CurrentSelections {
		if len(selection.Values) == 1 {
			selections[selection.FieldKey] = selection.Values[0]
		}
	}
	calls := make([]CandidateCall, 0, len(request.Requests))
	for _, query := range request.Requests {
		if query.FieldKey == "upstream_product_id" {
			calls = append(calls, CandidateCall{FieldKey: query.FieldKey, Method: http.MethodGet, Path: "/cart/all"})
			continue
		}
		productID := selections["upstream_product_id"]
		if !positiveDecimal(productID) {
			return nil, errInvalidConfig
		}
		calls = append(calls, CandidateCall{FieldKey: query.FieldKey, Method: http.MethodGet, Path: "/cart/get_product_config", Query: url.Values{"pid": {productID}}})
	}
	return calls, nil
}

func ProjectProductCandidates(raw json.RawMessage) ([]resourceinterfaceprojectioncap.Candidate, error) {
	var envelope struct {
		Data struct {
			Products []struct {
				Name     string `json:"name"`
				Products []struct {
					ID   json.RawMessage `json:"id"`
					Name string          `json:"name"`
				} `json:"products"`
			} `json:"products"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, errInvalidResponse
	}
	items := make([]resourceinterfaceprojectioncap.Candidate, 0)
	seen := make(map[string]struct{})
	for _, group := range envelope.Data.Products {
		for _, product := range group.Products {
			value := scalar(product.ID)
			label := strings.TrimSpace(product.Name)
			if !positiveDecimal(value) || label == "" || len(label) > 256 || len(items) >= maxCandidateItems {
				return nil, errInvalidResponse
			}
			if _, duplicate := seen[value]; duplicate {
				return nil, errInvalidResponse
			}
			seen[value] = struct{}{}
			items = append(items, resourceinterfaceprojectioncap.Candidate{Value: value, LabelZhCN: label, LabelEnUS: label})
		}
	}
	if len(items) == 0 {
		return nil, errInvalidResponse
	}
	return items, nil
}

func ProjectConfigurationCandidates(fieldKey string, selections []CandidateSelection, raw json.RawMessage) ([]resourceinterfaceprojectioncap.Candidate, error) {
	var envelope struct {
		Data struct {
			Groups []struct {
				Options []struct {
					ID         json.RawMessage `json:"id"`
					OptionName string          `json:"option_name"`
					Sub        []struct {
						ID         json.RawMessage `json:"id"`
						OptionName string          `json:"option_name"`
					} `json:"sub"`
				} `json:"options"`
			} `json:"config_groups"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, errInvalidResponse
	}
	var optionField string
	switch fieldKey {
	case "upstream_region_option_id", "upstream_line_option_id", "upstream_os_option_id":
		optionField = ""
	case "region":
		optionField = "upstream_region_option_id"
	case "line":
		optionField = "upstream_line_option_id"
	case "operating_system":
		optionField = "upstream_os_option_id"
	default:
		return nil, errUnsupported
	}
	selectedOption := selectionValue(selections, optionField)
	if optionField != "" && !positiveDecimal(selectedOption) {
		return nil, errInvalidConfig
	}
	items := make([]resourceinterfaceprojectioncap.Candidate, 0)
	seen := make(map[string]struct{})
	for _, group := range envelope.Data.Groups {
		for _, option := range group.Options {
			optionID := scalar(option.ID)
			if optionField == "" {
				candidate, err := candidate(optionID, option.OptionName)
				if err != nil || appendUniqueCandidate(&items, seen, candidate) != nil {
					return nil, errInvalidResponse
				}
				continue
			}
			if optionID != selectedOption {
				continue
			}
			for _, sub := range option.Sub {
				candidate, err := candidate(scalar(sub.ID), sub.OptionName)
				if err != nil || appendUniqueCandidate(&items, seen, candidate) != nil {
					return nil, errInvalidResponse
				}
			}
		}
	}
	if len(items) == 0 {
		return nil, errInvalidResponse
	}
	return items, nil
}

func selectionValue(selections []CandidateSelection, key string) string {
	for _, selection := range selections {
		if selection.FieldKey == key && len(selection.Values) == 1 {
			return selection.Values[0]
		}
	}
	return ""
}

func candidate(value, label string) (resourceinterfaceprojectioncap.Candidate, error) {
	label = strings.TrimSpace(label)
	if !positiveDecimal(value) || label == "" || len(label) > 256 {
		return resourceinterfaceprojectioncap.Candidate{}, errInvalidResponse
	}
	return resourceinterfaceprojectioncap.Candidate{Value: value, LabelZhCN: label, LabelEnUS: label}, nil
}

func appendUniqueCandidate(items *[]resourceinterfaceprojectioncap.Candidate, seen map[string]struct{}, value resourceinterfaceprojectioncap.Candidate) error {
	if len(*items) >= maxCandidateItems {
		return errInvalidResponse
	}
	if _, duplicate := seen[value.Value]; duplicate {
		return errInvalidResponse
	}
	seen[value.Value] = struct{}{}
	*items = append(*items, value)
	return nil
}

func scalar(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		return number.String()
	}
	return ""
}
