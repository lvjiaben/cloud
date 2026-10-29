// Package descriptor implements the public v1 protobuf bridge for a read-only
// template descriptor. It uses only Go's standard library and no host sources.
package descriptor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"strings"
)

// invocation is the public business-extension JSON contract, not host state.
type invocation struct {
	Version           string `json:"version"`
	Owner             string `json:"ownerPluginId"`
	Provider          string `json:"providerPluginId"`
	Point             string `json:"point"`
	Contract          string `json:"contractVersion"`
	Method            string `json:"method"`
	Release           string `json:"releaseVersion"`
	ReleaseDigest     string `json:"releaseDigest"`
	DeclarationDigest string `json:"declarationDigest"`
	Purpose           string `json:"purpose"`
	Runtime           *struct {
		ContextID string `json:"contextId"`
	} `json:"runtimeContext"`
	Payload json.RawMessage `json:"payload"`
}

// Handle accepts only the descriptor method bound to the exact installed plugin.
// Invalid wire data or invocation identity fails closed without invoking services.
func Handle(data []byte, pluginID, owner, point string) []byte {
	envelope, ok := fields(data)
	if !ok || string(envelope[1]) != pluginID {
		return response(400, nil)
	}
	route, ok := fields(envelope[2])
	if !ok || string(route[7]) != "BusinessExtensionBridgeRequest" {
		return response(404, nil)
	}
	request, ok := fields(envelope[3])
	if !ok {
		return response(400, nil)
	}
	var in invocation
	decoder := json.NewDecoder(bytes.NewReader(request[13]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return response(400, nil)
	}
	if in.Version != "v1" || in.Owner != owner || in.Provider != pluginID || in.Point != point || in.Contract != "v1" || in.Method != "capabilities.describe" || strings.TrimSpace(in.Release) == "" || len(in.ReleaseDigest) != 64 || len(in.DeclarationDigest) != 64 || in.Runtime == nil || strings.TrimSpace(in.Runtime.ContextID) == "" || (in.Purpose != "new-business" && in.Purpose != "existing-business") {
		return response(400, nil)
	}
	body, err := json.Marshal(struct {
		Version string `json:"version"`
		Payload struct {
			PluginID string `json:"pluginId"`
			Version  string `json:"version"`
		} `json:"payload"`
	}{Version: "v1", Payload: struct {
		PluginID string `json:"pluginId"`
		Version  string `json:"version"`
	}{pluginID, "v1"}})
	if err != nil {
		return response(500, nil)
	}
	return response(200, body)
}

// fields decodes protobuf's public wire format, ignoring unknown scalar fields.
// Duplicate byte fields and malformed/group wire data are rejected deterministically.
func fields(data []byte) (map[uint64][]byte, bool) {
	out := make(map[uint64][]byte)
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 || tag>>3 == 0 {
			return nil, false
		}
		data = data[n:]
		switch tag & 7 {
		case 0:
			_, n = binary.Uvarint(data)
			if n <= 0 {
				return nil, false
			}
			data = data[n:]
		case 1:
			if len(data) < 8 {
				return nil, false
			}
			data = data[8:]
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 || size > uint64(len(data)-n) {
				return nil, false
			}
			data = data[n:]
			if _, exists := out[tag>>3]; exists {
				return nil, false
			}
			out[tag>>3] = data[:int(size)]
			data = data[int(size):]
		case 5:
			if len(data) < 4 {
				return nil, false
			}
			data = data[4:]
		default:
			return nil, false
		}
	}
	return out, true
}

// response encodes the published bridge response field numbers (status/type/body).
func response(status uint64, body []byte) []byte {
	out := binary.AppendUvarint(nil, 8)
	out = binary.AppendUvarint(out, status)
	out = appendBytes(out, 2, []byte("application/json; charset=utf-8"))
	if len(body) == 0 {
		body = []byte(`{"errorCode":"TEMPLATE_DESCRIPTOR_INVALID","messageKey":"template.descriptor.invalid","message":"Template description request is invalid"}`)
	}
	return appendBytes(out, 4, body)
}

// appendBytes emits one protobuf length-delimited field without external codecs.
func appendBytes(out []byte, field uint64, value []byte) []byte {
	out = binary.AppendUvarint(out, field<<3|2)
	out = binary.AppendUvarint(out, uint64(len(value)))
	return append(out, value...)
}
