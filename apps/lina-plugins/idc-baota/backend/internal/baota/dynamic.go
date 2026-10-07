package baota

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var errUnsupported = errors.New("BT Panel capability is unsupported")

var responseFields = []string{"status", "data", "id", "name", "path", "siteStatus", "siteId", "msg"}

func ExecuteResource(executor confidentialoutboundcap.Service, contextID, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case resourcecap.DynamicProviderMethodVerifyConnection:
		response, err := call(executor, contextID, 1, "/data", map[string]string{
			"action": "getData", "table": "sites", "type": "-1", "p": "1", "limit": "1", "list": "true",
		})
		if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, errInvalidConfiguration
		}
		return json.Marshal(resourcecap.ConnectionVerification{Verified: true, ObservedAt: time.Now().UTC()})
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		return json.Marshal(capabilities())
	case resourcecap.DynamicProviderMethodBuildProvisionParameters:
		var plan resourcecap.ServerProvisionPlan
		if strict(payload, &plan) != nil || plan.Validate() != nil {
			return nil, errInvalidConfiguration
		}
		return BuildSitePlan(plan.ProductConfiguration, plan.CustomerConfiguration)
	case resourcecap.DynamicProviderMethodExecute:
		return executeOperation(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodQueryResource:
		return executeResourceQuery(executor, contextID, payload)
	default:
		return nil, errUnsupported
	}
}

func capabilities() resourcecap.CapabilitySnapshot {
	codes := []resourcecap.CapabilityCode{resourcecap.CapabilityAuthentication, resourcecap.CapabilityDiscovery, resourcecap.CapabilityProviderCapacity, resourcecap.CapabilityCost, resourcecap.CapabilityBalance, resourcecap.CapabilityExpiry, resourcecap.CapabilityProvision, resourcecap.CapabilityResourceQuery, resourcecap.CapabilityStart, resourcecap.CapabilityStop, resourcecap.CapabilityReboot, resourcecap.CapabilitySuspend, resourcecap.CapabilityResume, resourcecap.CapabilityRenew, resourcecap.CapabilityResetPassword, resourcecap.CapabilityConsole, resourcecap.CapabilityReinstall, resourcecap.CapabilityEnterRescue, resourcecap.CapabilityExitRescue, resourcecap.CapabilityResize, resourcecap.CapabilityChangeIP, resourcecap.CapabilityManageSnapshot, resourcecap.CapabilityBackup, resourcecap.CapabilitySecurityGroup, resourcecap.CapabilityNAT, resourcecap.CapabilityVirtualMedia, resourcecap.CapabilityTrafficPackage, resourcecap.CapabilityUpstreamTicket, resourcecap.CapabilityTerminate}
	items := make([]resourcecap.CapabilityDescriptor, 0, len(codes))
	for _, code := range codes {
		item := resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelUnsupported, UnsupportedReason: "BT Panel server connector does not publish this capability"}
		if code == resourcecap.CapabilityAuthentication || code == resourcecap.CapabilityDiscovery || code == resourcecap.CapabilityProvision || code == resourcecap.CapabilityResourceQuery {
			item.Support = resourcecap.SupportLevelSupported
			item.UnsupportedReason = ""
		}
		items = append(items, item)
	}
	return resourcecap.CapabilitySnapshot{AdapterVersion: "bt-panel-v11-site-v1", Capabilities: items, ObservedAt: time.Now().UTC()}
}

func executeOperation(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.DynamicOperationRequest
	if executor == nil || json.Unmarshal(payload, &request) != nil || request.Validate() != nil || request.Capability != resourcecap.CapabilityProvision || len(request.ConfidentialInputNames) != 0 || request.ProviderCorrelation != nil {
		return nil, errInvalidConfiguration
	}
	var plan FrozenSitePlan
	if strict(request.Parameters, &plan) != nil || validateFrozenSitePlan(plan) != nil {
		return nil, errInvalidConfiguration
	}
	existing, err := querySite(executor, contextID, 1, plan.Domains[0])
	if err != nil {
		return json.Marshal(failedResult(request.RequestID))
	}
	if existing.ID != "" {
		return json.Marshal(successResult(request.RequestID, existing.ID, "BT Panel query recovered the existing site"))
	}
	webname, err := json.Marshal(struct {
		Domain     string   `json:"domain"`
		DomainList []string `json:"domainlist"`
		Count      int      `json:"count"`
	}{Domain: plan.Domains[0], DomainList: plan.Domains[1:], Count: len(plan.Domains) - 1})
	if err != nil {
		return nil, errInvalidConfiguration
	}
	form := map[string]string{"action": "AddSite", "webname": string(webname), "path": plan.Path, "version": plan.PHPVersion, "port": "80", "type_id": "0", "ps": plan.Domains[0], "ftp": "false", "sql": "false", "codeing": "utf8"}
	if plan.DeploymentType == "php_site" {
		form["type"] = "PHP"
	} else {
		form["type"] = ""
	}
	response, err := call(executor, contextID, 2, "/site", form)
	if err != nil || response.StatusCode >= 500 {
		return json.Marshal(unknownResult(request.RequestID))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return json.Marshal(failedResult(request.RequestID))
	}
	siteID := findScalar(response.Data, "siteId")
	if findBool(response.Data, "siteStatus") && positiveDecimal(siteID) {
		return json.Marshal(successResult(request.RequestID, siteID, "BT Panel created the requested site"))
	}
	if hasExplicitFailure(response.Data) {
		return json.Marshal(failedResult(request.RequestID))
	}
	return json.Marshal(unknownResult(request.RequestID))
}

func executeResourceQuery(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourcecap.ResourceQuery
	if executor == nil || json.Unmarshal(payload, &query) != nil || query.Validate() != nil {
		return nil, errInvalidConfiguration
	}
	site, err := querySite(executor, contextID, 1, query.ExternalResourceRef)
	if err != nil || site.ID == "" {
		return nil, errInvalidConfiguration
	}
	state := resourcecap.ResourceStateActive
	if site.Status == "0" {
		state = resourcecap.ResourceStateStopped
	} else if site.Status != "1" {
		state = resourcecap.ResourceStateUnknown
	}
	data, err := json.Marshal(map[string]string{"name": site.Name, "path": site.Path})
	if err != nil {
		return nil, errInvalidConfiguration
	}
	now := time.Now().UTC()
	return json.Marshal(resourcecap.ResourceSnapshot{ExternalResourceRef: site.ID, State: state, Data: data, ObservedAt: now, Evidence: []resourcecap.Evidence{{Kind: "bt_panel_site", Reference: site.ID, Summary: "BT Panel site status observed", ObservedAt: now}}})
}

type siteProjection struct{ ID, Name, Path, Status string }

func querySite(executor confidentialoutboundcap.Service, contextID string, sequence uint8, search string) (siteProjection, error) {
	response, err := call(executor, contextID, sequence, "/data", map[string]string{"action": "getData", "table": "sites", "type": "-1", "search": search, "p": "1", "limit": "20", "list": "true"})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return siteProjection{}, errInvalidConfiguration
	}
	var envelope struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(response.Data, &envelope) != nil || len(envelope.Data) > 20 {
		return siteProjection{}, errInvalidConfiguration
	}
	for _, row := range envelope.Data {
		id, name := scalar(row["id"]), scalar(row["name"])
		if !positiveDecimal(id) {
			continue
		}
		if search != id && !strings.EqualFold(strings.TrimSpace(search), strings.TrimSpace(name)) {
			continue
		}
		return siteProjection{ID: id, Name: name, Path: scalar(row["path"]), Status: scalar(row["status"])}, nil
	}
	return siteProjection{}, nil
}

func call(executor confidentialoutboundcap.Service, contextID string, sequence uint8, path string, form map[string]string) (resourcecap.ConfidentialHTTPResponse, error) {
	payload, err := json.Marshal(resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: path, Form: form, ResponseFields: responseFields, UseAuthentication: true})
	if err != nil {
		return resourcecap.ConfidentialHTTPResponse{}, err
	}
	result, err := executor.Execute(contextID, sequence, payload)
	if err != nil {
		return resourcecap.ConfidentialHTTPResponse{}, err
	}
	var response resourcecap.ConfidentialHTTPResponse
	if json.Unmarshal(result, &response) != nil {
		return response, errInvalidConfiguration
	}
	return response, nil
}

func validateFrozenSitePlan(plan FrozenSitePlan) error {
	product, err := json.Marshal(ProductConfiguration{DeploymentType: plan.DeploymentType, PHPVersion: plan.PHPVersion})
	if err != nil {
		return errInvalidConfiguration
	}
	customer, err := json.Marshal(CustomerConfiguration{PrimaryDomain: first(plan.Domains), AdditionalDomains: strings.Join(plan.Domains[1:], " ")})
	if err != nil {
		return errInvalidConfiguration
	}
	encoded, err := BuildSitePlan(product, customer)
	if err != nil {
		return err
	}
	var rebuilt FrozenSitePlan
	if json.Unmarshal(encoded, &rebuilt) != nil || rebuilt.Version != plan.Version || rebuilt.DeploymentType != plan.DeploymentType ||
		rebuilt.Path != plan.Path || rebuilt.PHPVersion != plan.PHPVersion || !slices.Equal(rebuilt.Domains, plan.Domains) {
		return errInvalidConfiguration
	}
	return nil
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func scalar(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if decoder.Decode(&number) == nil {
		return number.String()
	}
	return ""
}

func findScalar(raw json.RawMessage, key string) string {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	return scalar(values[key])
}

func findBool(raw json.RawMessage, key string) bool {
	var values map[string]json.RawMessage
	var value bool
	return json.Unmarshal(raw, &values) == nil && json.Unmarshal(values[key], &value) == nil && value
}

func hasExplicitFailure(raw json.RawMessage) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	if value, exists := values["siteStatus"]; exists {
		var status bool
		return json.Unmarshal(value, &status) == nil && !status
	}
	return false
}

func positiveDecimal(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 63)
	return err == nil && parsed > 0 && value == strconv.FormatUint(parsed, 10)
}

func successResult(requestID, siteID, summary string) resourcecap.OperationResult {
	now := time.Now().UTC()
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: siteID, Evidence: []resourcecap.Evidence{{Kind: "bt_panel_site", Reference: requestID, Summary: summary, ObservedAt: now}}}
}
func failedResult(requestID string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusFailed, Failure: &resourcecap.ProviderFailure{Code: "BT_PANEL_REJECTED", Message: "BT Panel rejected the site operation"}, Evidence: []resourcecap.Evidence{{Kind: "bt_panel_rejection", Reference: requestID, Summary: "BT Panel explicitly rejected the site operation", ObservedAt: time.Now().UTC()}}}
}
func unknownResult(requestID string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, Failure: &resourcecap.ProviderFailure{Code: "BT_PANEL_RESULT_UNKNOWN", Message: "BT Panel site creation requires manual reconciliation", ResultUnknown: true}, Evidence: []resourcecap.Evidence{{Kind: "bt_panel_result_unknown", Reference: requestID, Summary: "BT Panel site creation outcome could not be proven", ObservedAt: time.Now().UTC()}}}
}
