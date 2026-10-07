// This file decodes untrusted upstream JSON into bounded scalar projections.

package zjmfcloud

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

type rawObject map[string]json.RawMessage

func decodeObject(body []byte) (rawObject, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var object rawObject
	if err := decoder.Decode(&object); err != nil || object == nil || decoder.Decode(&struct{}{}) != io.EOF {
		clearRawObject(object)
		return nil, errInvalidResponse
	}
	return object, nil
}

func clearRawObject(object rawObject) {
	for key, raw := range object {
		clear(raw)
		delete(object, key)
	}
}

func clearRawObjects(objects []rawObject) {
	for _, object := range objects {
		clearRawObject(object)
	}
}

func unwrapObject(object rawObject) rawObject {
	data, exists := object["data"]
	if !exists || len(data) == 0 || data[0] != '{' {
		return object
	}
	var nested rawObject
	if json.Unmarshal(data, &nested) != nil || nested == nil {
		clearRawObject(nested)
		return object
	}
	clearRawObject(object)
	return nested
}

func decodeObjectPage(body []byte) ([]rawObject, rawObject, error) {
	root, err := decodeObject(body)
	if err != nil {
		return nil, nil, err
	}
	defer clearRawObject(root)
	data := root["data"]
	meta := rawObject{}
	if rawMeta, exists := root["meta"]; exists && len(rawMeta) > 0 && rawMeta[0] == '{' {
		if json.Unmarshal(rawMeta, &meta) != nil {
			return nil, nil, errInvalidResponse
		}
	}
	if len(data) > 0 && data[0] == '{' {
		var nested rawObject
		if json.Unmarshal(data, &nested) != nil {
			clearRawObject(nested)
			return nil, nil, errInvalidResponse
		}
		defer clearRawObject(nested)
		if nestedData, exists := nested["data"]; exists {
			data = nestedData
		}
		if len(meta) == 0 {
			if nestedMeta, exists := nested["meta"]; exists && len(nestedMeta) > 0 && nestedMeta[0] == '{' {
				if json.Unmarshal(nestedMeta, &meta) != nil {
					return nil, nil, errInvalidResponse
				}
			}
		}
	}
	if len(data) == 0 || data[0] != '[' {
		return nil, nil, errInvalidResponse
	}
	var items []rawObject
	if json.Unmarshal(data, &items) != nil {
		clearRawObjects(items)
		return nil, nil, errInvalidResponse
	}
	return items, meta, nil
}

func projectScalars(object rawObject, allowed []string) (json.RawMessage, error) {
	projected := make(map[string]any, len(allowed))
	for _, key := range allowed {
		raw, exists := object[key]
		if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		value, ok := decodeScalar(raw)
		if !ok {
			continue
		}
		projected[key] = value
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		return nil, errInvalidResponse
	}
	return encoded, nil
}

func decodeScalar(raw json.RawMessage) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, false
	}
	switch typed := value.(type) {
	case string:
		if typed == "" || typed != strings.TrimSpace(typed) || strings.ContainsAny(typed, "\r\n\x00") {
			return nil, false
		}
		return typed, true
	case json.Number:
		if _, err := typed.Int64(); err == nil {
			return typed, true
		}
		if _, err := typed.Float64(); err == nil {
			return typed, true
		}
		return nil, false
	case bool:
		return typed, true
	default:
		return nil, false
	}
}

func firstReference(object rawObject, keys ...string) (string, bool) {
	for _, key := range keys {
		raw, exists := object[key]
		if !exists {
			continue
		}
		if reference, ok := numericReference(raw); ok {
			return reference, true
		}
	}
	return "", false
}

func numericReference(raw json.RawMessage) (string, bool) {
	value, ok := decodeScalar(raw)
	if !ok {
		return "", false
	}
	var candidate string
	switch typed := value.(type) {
	case string:
		candidate = typed
	case json.Number:
		candidate = typed.String()
	default:
		return "", false
	}
	if !validNumericReference(candidate) {
		return "", false
	}
	return candidate, true
}

func validNumericReference(value string) bool {
	if value == "" || len(value) > 20 || value[0] == '0' {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func metaInteger(meta rawObject, key string) (int, bool) {
	raw, exists := meta[key]
	if !exists {
		return 0, false
	}
	value, ok := decodeScalar(raw)
	if !ok {
		return 0, false
	}
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case json.Number:
		text = typed.String()
	default:
		return 0, false
	}
	parsed, err := strconv.Atoi(text)
	if err != nil || parsed < 0 {
		return 0, false
	}
	return parsed, true
}

func formScalar(raw json.RawMessage) (string, bool) {
	value, ok := decodeScalar(raw)
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return typed, true
	case json.Number:
		return typed.String(), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}
