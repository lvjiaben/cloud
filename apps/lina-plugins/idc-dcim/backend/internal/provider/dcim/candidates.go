// This file projects bounded server-group and operating-system choices.

package dcim

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

type candidateRequest struct {
	SchemaVersion     string               `json:"schemaVersion"`
	CurrentSelections []candidateSelection `json:"currentSelections"`
	Requests          []candidateQuery     `json:"requests"`
}

type candidateSelection struct {
	FieldKey string   `json:"fieldKey"`
	Values   []string `json:"values"`
}

type candidateQuery struct {
	FieldKey       string   `json:"fieldKey"`
	ParentValues   []string `json:"parentValues"`
	SelectedValues []string `json:"selectedValues"`
}

type candidateCall struct {
	FieldKey string
	Method   string
	Path     string
	Query    url.Values
	Form     url.Values
}

func decodeCandidateRequest(raw json.RawMessage) (candidateRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request candidateRequest
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		request.SchemaVersion != "v1" || len(request.Requests) == 0 || len(request.Requests) > 2 || len(request.CurrentSelections) > 16 {
		return candidateRequest{}, errInvalidConfiguration
	}
	seen := make(map[string]struct{}, len(request.Requests))
	for _, query := range request.Requests {
		if query.FieldKey != "server_group" && query.FieldKey != "operating_system" || len(query.ParentValues) > 8 || len(query.SelectedValues) > maxCandidateItems {
			return candidateRequest{}, errUnsupported
		}
		if _, duplicate := seen[query.FieldKey]; duplicate {
			return candidateRequest{}, errInvalidConfiguration
		}
		seen[query.FieldKey] = struct{}{}
	}
	for _, selection := range request.CurrentSelections {
		if selection.FieldKey != "server_group" && selection.FieldKey != "operating_system" || len(selection.Values) > maxCandidateItems {
			return candidateRequest{}, errInvalidConfiguration
		}
		for _, value := range selection.Values {
			if !positiveDecimal(value) {
				return candidateRequest{}, errInvalidConfiguration
			}
		}
	}
	return request, nil
}

func candidateCalls(protocol Protocol, request candidateRequest) []candidateCall {
	calls := make([]candidateCall, 0, len(request.Requests))
	for _, query := range request.Requests {
		if protocol == ProtocolLegacy {
			action := "getSaleGroup"
			if query.FieldKey == "operating_system" {
				action = "getAllMirrorOs"
			}
			calls = append(calls, candidateCall{FieldKey: query.FieldKey, Method: http.MethodPost, Path: "/index.php", Query: url.Values{"m": {"api"}, "a": {action}}})
			continue
		}
		path := "/bms/lcGroup"
		if query.FieldKey == "operating_system" {
			path = "/bms/os"
		}
		calls = append(calls, candidateCall{FieldKey: query.FieldKey, Method: http.MethodGet, Path: path, Query: url.Values{"listpages": {"9999"}, "sort": {"asc"}}})
	}
	return calls
}

func projectCandidates(protocol Protocol, fieldKey string, raw json.RawMessage) ([]resourceinterfaceprojectioncap.Candidate, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root any
	if decoder.Decode(&root) != nil {
		return nil, errInvalidResponse
	}
	arrays := findCandidateArrays(root)
	items := make([]resourceinterfaceprojectioncap.Candidate, 0)
	seen := make(map[string]struct{})
	for _, array := range arrays {
		for _, value := range array {
			object, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if protocol == ProtocolBMS && fieldKey == "operating_system" {
				if firstScalar(object, "status") != "1" {
					continue
				}
				imageName := firstScalar(object, "os_name")
				if imageName == "Windows_Rescue.raw" || imageName == "Linux_Rescue.raw" {
					continue
				}
			}
			id := firstScalar(object, "id", "value", "group_id", "os_id")
			label := firstScalar(object, "name", "title", "label", "show_name")
			if !positiveDecimal(id) || label == "" || len(label) > 256 {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			items = append(items, resourceinterfaceprojectioncap.Candidate{Value: id, LabelZhCN: label, LabelEnUS: label})
			if len(items) > maxCandidateItems {
				return nil, errInvalidResponse
			}
		}
	}
	if len(items) == 0 {
		return nil, errInvalidResponse
	}
	return items, nil
}

func findCandidateArrays(value any) [][]any {
	var result [][]any
	var walk func(any, int)
	walk = func(current any, depth int) {
		if depth > 4 {
			return
		}
		switch typed := current.(type) {
		case []any:
			result = append(result, typed)
		case map[string]any:
			for _, key := range []string{"data", "list", "rows", "groups", "os"} {
				if child, ok := typed[key]; ok {
					walk(child, depth+1)
				}
			}
		}
	}
	walk(value, 0)
	return result
}

func firstScalar(object map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := object[key].(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		case json.Number:
			return value.String()
		}
	}
	return ""
}
