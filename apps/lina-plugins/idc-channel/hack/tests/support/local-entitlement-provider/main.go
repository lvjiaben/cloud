//go:build wasip1

// This file initializes the managed guest once and refuses every ABI entry
// if startup dependencies cannot be constructed.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"os"
)

var guestRuntime, guestRuntimeError = newGuestRuntime()

func newGuestRuntime() (pluginbridge.GuestRuntime, error) {
	handler := func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		var input capmodel.BusinessExtensionBridgeRequest
		if request == nil || request.Request == nil || json.Unmarshal(request.Request.Body, &input) != nil {
			return protocol.NewBadRequestResponse("Invalid fixture invocation"), nil
		}
		if input.Method != "provision" && input.Method != "query" {
			return protocol.NewBadRequestResponse("Unsupported fixture method"), nil
		}
		var requestPayload struct {
			Version                   string `json:"version"`
			IntentNo                  string `json:"intentNo"`
			PreviousExternalReference string `json:"previousExternalReference"`
		}
		if json.Unmarshal(input.Payload, &requestPayload) != nil || requestPayload.Version != "v1" || requestPayload.IntentNo == "" {
			return protocol.NewBadRequestResponse("Invalid fixture payload"), nil
		}
		digest := sha256.Sum256([]byte(requestPayload.IntentNo))
		reference := "fixture-" + hex.EncodeToString(digest[:16])
		payload := struct {
			Version           string `json:"version"`
			Status            string `json:"status"`
			ExternalReference string `json:"externalReference,omitempty"`
		}{Version: "v1", Status: "accepted", ExternalReference: reference}
		if input.Method == "query" {
			payload.Status = "not_found"
			payload.ExternalReference = ""
			if requestPayload.PreviousExternalReference == reference {
				payload.Status = "succeeded"
				payload.ExternalReference = reference
			}
		}
		result, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(capmodel.BusinessExtensionBridgeResponse{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: result})
		if err != nil {
			return nil, err
		}
		return protocol.NewJSONResponse(200, body), nil
	}
	return pluginbridge.NewGuestRuntime(handler), nil
}

//go:wasmexport lina_dynamic_route_alloc
func DynamicRouteAlloc(size uint32) uint32 {
	if guestRuntimeError != nil {
		return 0
	}
	return guestRuntime.Alloc(size)
}

//go:wasmexport lina_dynamic_route_execute
func DynamicRouteExecute(length uint32) uint64 {
	if guestRuntimeError != nil {
		return 0
	}
	pointer, responseLength, err := guestRuntime.Execute(length)
	if err != nil {
		return 0
	}
	return uint64(pointer)<<32 | uint64(responseLength)
}

//go:wasmexport lina_host_call_alloc
func HostCallAlloc(size uint32) uint32 {
	if guestRuntimeError != nil {
		return 0
	}
	return guestRuntime.HostCallAlloc(size)
}

func main() {
	if guestRuntimeError != nil {
		os.Exit(1)
	}
}
