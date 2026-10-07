// This file resolves product configuration candidates through the resource
// owner's one-use confidential outbound context and returns only bounded,
// bilingual label/value projections.

package zjmfcloud

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

const maximumDynamicCandidateItems = 512

type dynamicCandidateRequest struct {
	SchemaVersion     string                  `json:"schemaVersion"`
	CurrentSelections []json.RawMessage       `json:"currentSelections"`
	Requests          []dynamicCandidateQuery `json:"requests"`
}

type dynamicCandidateQuery struct {
	FieldKey       string   `json:"fieldKey"`
	ParentValues   []string `json:"parentValues"`
	SelectedValues []string `json:"selectedValues"`
}

func dynamicConfigurationCandidates(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var request dynamicCandidateRequest
	if decoder.Decode(&request) != nil || request.SchemaVersion != "v1" || len(request.Requests) == 0 || len(request.Requests) > 3 {
		return nil, errInvalidConfig
	}
	fields := make([]resourceinterfaceprojectioncap.CandidateField, 0, len(request.Requests))
	total := 0
	seenFields := make(map[string]struct{}, len(request.Requests))
	for index, query := range request.Requests {
		path, values, err := dynamicCandidatePath(query)
		if err != nil {
			return nil, err
		}
		if _, exists := seenFields[query.FieldKey]; exists {
			return nil, errInvalidConfig
		}
		seenFields[query.FieldKey] = struct{}{}
		response, err := dynamicCall(executor, contextID, uint8(index+2), http.MethodGet, path, values, nil)
		if err != nil {
			return nil, err
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return nil, readStatusError(response.StatusCode)
		}
		items, err := projectDynamicCandidatePage(response.Data)
		if err != nil || len(items) == 0 {
			return nil, errInvalidResponse
		}
		total += len(items)
		if total > maximumDynamicCandidateItems {
			return nil, errInvalidResponse
		}
		fields = append(fields, resourceinterfaceprojectioncap.CandidateField{FieldKey: query.FieldKey, Items: items})
	}
	return json.Marshal(resourceinterfaceprojectioncap.CandidateResult{Fields: fields})
}

func dynamicCandidatePath(query dynamicCandidateQuery) (string, url.Values, error) {
	values := url.Values{"page": {"1"}, "per_page": {"512"}}
	switch query.FieldKey {
	case "region":
		if len(query.ParentValues) != 0 {
			return "", nil, errInvalidConfig
		}
		return "/areas", values, nil
	case "line":
		if len(query.ParentValues) > 1 {
			return "", nil, errInvalidConfig
		}
		if len(query.ParentValues) == 1 {
			values.Set("area_id", query.ParentValues[0])
		}
		return "/nodes", values, nil
	case "operating_system":
		if len(query.ParentValues) != 0 {
			return "", nil, errInvalidConfig
		}
		return "/image", values, nil
	default:
		return "", nil, errUnsupported
	}
}

func projectDynamicCandidatePage(payload json.RawMessage) ([]resourceinterfaceprojectioncap.Candidate, error) {
	var envelope struct {
		Data []struct {
			ID   json.RawMessage `json:"id"`
			Name string          `json:"name"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &envelope) != nil || len(envelope.Data) > maximumDynamicCandidateItems {
		return nil, errInvalidResponse
	}
	items := make([]resourceinterfaceprojectioncap.Candidate, 0, len(envelope.Data))
	seen := make(map[string]struct{}, len(envelope.Data))
	for _, entry := range envelope.Data {
		value := dynamicCandidateScalar(entry.ID)
		label := strings.TrimSpace(entry.Name)
		if value == "" || label == "" || len(value) > 256 || len(label) > 256 {
			return nil, errInvalidResponse
		}
		if _, exists := seen[value]; exists {
			return nil, errInvalidResponse
		}
		seen[value] = struct{}{}
		items = append(items, resourceinterfaceprojectioncap.Candidate{Value: value, LabelZhCN: label, LabelEnUS: label})
	}
	return items, nil
}

func dynamicCandidateScalar(raw json.RawMessage) string {
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
