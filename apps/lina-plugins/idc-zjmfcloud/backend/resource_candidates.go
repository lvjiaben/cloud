// This file defines the strict secret-free request and response boundary for
// resource-owned product configuration candidate discovery.

package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

const (
	maximumCandidateRequest  = 60 * 1024
	maximumCandidateResponse = 1 << 20
	maximumCandidateItems    = 512
)

type resourceCandidateRequest struct {
	SchemaVersion     string                       `json:"schemaVersion"`
	CurrentSelections []resourceCandidateSelection `json:"currentSelections"`
	Requests          []resourceCandidateQuery     `json:"requests"`
}

type resourceCandidateSelection struct {
	FieldKey string   `json:"fieldKey"`
	Values   []string `json:"values"`
}

type resourceCandidateQuery struct {
	FieldKey       string   `json:"fieldKey"`
	ParentValues   []string `json:"parentValues"`
	SelectedValues []string `json:"selectedValues"`
}

func decodeResourceCandidateRequest(payload json.RawMessage) (resourceCandidateRequest, error) {
	var request resourceCandidateRequest
	if len(payload) == 0 || len(payload) > maximumCandidateRequest {
		return request, io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return request, err
	}
	if request.SchemaVersion != "v1" || len(request.Requests) == 0 || len(request.Requests) > 3 || len(request.CurrentSelections) > 3 {
		return request, io.ErrUnexpectedEOF
	}
	seen := make(map[string]struct{}, len(request.Requests))
	for _, query := range request.Requests {
		if !validCandidateField(query.FieldKey) || !validCandidateValues(query.ParentValues, 1) || !validCandidateValues(query.SelectedValues, maximumCandidateItems) {
			return request, io.ErrUnexpectedEOF
		}
		if query.FieldKey != "line" && len(query.ParentValues) != 0 {
			return request, io.ErrUnexpectedEOF
		}
		if _, exists := seen[query.FieldKey]; exists {
			return request, io.ErrUnexpectedEOF
		}
		seen[query.FieldKey] = struct{}{}
	}
	for _, selection := range request.CurrentSelections {
		if !validCandidateField(selection.FieldKey) || !validCandidateValues(selection.Values, maximumCandidateItems) {
			return request, io.ErrUnexpectedEOF
		}
	}
	return request, nil
}

func validateResourceCandidateResult(payload, requestPayload json.RawMessage) error {
	request, err := decodeResourceCandidateRequest(requestPayload)
	if err != nil || len(payload) == 0 || len(payload) > maximumCandidateResponse {
		return io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var result resourceinterfaceprojectioncap.CandidateResult
	if err = decoder.Decode(&result); err != nil {
		return err
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || len(result.Fields) != len(request.Requests) {
		return io.ErrUnexpectedEOF
	}
	wanted := make(map[string]struct{}, len(request.Requests))
	for _, query := range request.Requests {
		wanted[query.FieldKey] = struct{}{}
	}
	total := 0
	for _, field := range result.Fields {
		if _, ok := wanted[field.FieldKey]; !ok || len(field.Items) == 0 {
			return io.ErrUnexpectedEOF
		}
		delete(wanted, field.FieldKey)
		seen := make(map[string]struct{}, len(field.Items))
		for _, item := range field.Items {
			if !safeCandidateText(item.Value) || !safeCandidateText(item.LabelZhCN) || !safeCandidateText(item.LabelEnUS) {
				return io.ErrUnexpectedEOF
			}
			if _, exists := seen[item.Value]; exists {
				return io.ErrUnexpectedEOF
			}
			seen[item.Value] = struct{}{}
			total++
			if total > maximumCandidateItems {
				return io.ErrUnexpectedEOF
			}
		}
	}
	if len(wanted) != 0 {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func validCandidateField(value string) bool {
	return value == "region" || value == "line" || value == "operating_system"
}

func validCandidateValues(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	for _, value := range values {
		if !safeCandidateText(value) {
			return false
		}
	}
	return true
}

func safeCandidateText(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 256 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
