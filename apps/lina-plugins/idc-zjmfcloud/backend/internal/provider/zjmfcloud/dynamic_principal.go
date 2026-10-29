// This file maps confidential principal slots to exact administrator v1 calls.
package zjmfcloud

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"net/http"
	"net/url"
	"time"
)

func dynamicPrincipal(executor confidentialoutboundcap.Service, contextID, method string, payload json.RawMessage) (json.RawMessage, error) {
	create := method == resourcecap.DynamicProviderMethodCreatePrincipal
	var request resourcecap.DynamicPrincipalRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate(create) != nil {
		return nil, errInvalidConfig
	}
	route := "/user/check"
	form := url.Values{}
	bindings := map[string]string{resourcecap.PrincipalBindingUsername: resourcecap.PrincipalBindingUsername}
	if create {
		route = "/user"
		form.Set("state", "1")
		for _, name := range request.ConfidentialInputNames {
			bindings[name] = name
		}
	}
	response, err := dynamicCall(executor, contextID, 2, http.MethodPost, route, nil, form, bindings)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errInvalidResponse
	}
	object, err := decodeObject(response.Data)
	if err != nil {
		return nil, errInvalidResponse
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	now := time.Now().UTC()
	evidence := []resourcecap.Evidence{{Kind: "upstream_principal", Reference: request.RequestID, Summary: "exact upstream principal reference observed", ObservedAt: now}}
	if !create {
		if id, ok := metaInteger(object, "id"); ok && id == 0 {
			return json.Marshal(resourcecap.PrincipalLookupResult{Exists: false, ObservedAt: now, Evidence: evidence})
		}
		id, ok := firstReference(object, "id")
		if !ok {
			return nil, errInvalidResponse
		}
		return json.Marshal(resourcecap.PrincipalLookupResult{Exists: true, ExternalPrincipalRef: id, ObservedAt: now, Evidence: evidence})
	}
	id, ok := firstReference(object, "id")
	if !ok {
		return nil, errInvalidResponse
	}
	return json.Marshal(resourcecap.PrincipalCreateResult{ExternalPrincipalRef: id, ObservedAt: now, Evidence: evidence})
}
