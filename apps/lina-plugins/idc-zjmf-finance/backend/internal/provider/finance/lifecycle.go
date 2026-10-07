package finance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type Call struct {
	Method string
	Path   string
	Query  url.Values
	Form   url.Values
}

func LifecycleCall(request resourcecap.DynamicOperationRequest) (Call, error) {
	if request.Validate() != nil || request.ResourceRef == "" || !positiveDecimal(request.ResourceRef) {
		return Call{}, errInvalidConfig
	}
	var function string
	switch request.Capability {
	case resourcecap.CapabilityStart:
		function = "on"
	case resourcecap.CapabilityStop:
		function = modeFunction(request.Parameters, "off", "hard_off")
	case resourcecap.CapabilityReboot:
		function = modeFunction(request.Parameters, "reboot", "hard_reboot")
	case resourcecap.CapabilitySuspend:
		function = "suspend"
	case resourcecap.CapabilityResume:
		function = "unsuspend"
	case resourcecap.CapabilityTerminate:
		return Call{Method: http.MethodPost, Path: "/host/cancel", Form: url.Values{
			"id": {request.ResourceRef}, "type": {"Immediate"}, "reason": {"Immediate cancellation"},
		}}, nil
	default:
		return Call{}, errUnsupported
	}
	if function == "" {
		return Call{}, errInvalidConfig
	}
	form := url.Values{"id": {request.ResourceRef}, "func": {function}, "is_api": {"1"}}
	return Call{Method: http.MethodPost, Path: "/provision/default", Form: form}, nil
}

func ResourceQueryCall(query resourcecap.ResourceQuery) (Call, error) {
	if query.Validate() != nil || !positiveDecimal(query.ExternalResourceRef) {
		return Call{}, errInvalidConfig
	}
	return Call{Method: http.MethodPost, Path: "/provision/default", Form: url.Values{
		"id": {query.ExternalResourceRef}, "func": {"status"}, "is_api": {"1"},
	}}, nil
}

func modeFunction(raw json.RawMessage, graceful, hard string) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return graceful
	}
	var input struct {
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		return ""
	}
	switch strings.ToLower(input.Mode) {
	case "", "graceful":
		return graceful
	case "hard":
		return hard
	default:
		return ""
	}
}

func NormalizePowerState(value string) resourcecap.ResourceState {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "running", "active":
		return resourcecap.ResourceStateActive
	case "off", "stopped":
		return resourcecap.ResourceStateStopped
	case "suspended":
		return resourcecap.ResourceStateSuspended
	case "pending":
		return resourcecap.ResourceStatePending
	case "terminated", "cancelled", "canceled", "deleted":
		return resourcecap.ResourceStateTerminated
	case "failed", "error":
		return resourcecap.ResourceStateFailed
	default:
		return resourcecap.ResourceStateUnknown
	}
}
