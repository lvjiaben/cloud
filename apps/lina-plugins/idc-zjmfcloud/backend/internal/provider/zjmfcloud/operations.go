// This file maps supported resource operations to verified administrator v1 writes.

package zjmfcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type operationCall struct {
	method            string
	route             string
	query             url.Values
	form              url.Values
	ownershipKind     string
	ownershipRef      string
	ownershipType     string
	postconditionKind string
	postconditionRef  string
	requiresTask      bool
	queryAfterSuccess bool
}

const (
	maximumSnapshotNameBytes       = 128
	maximumOwnershipCandidateCount = 10_000
	maximumTrafficPackageGB        = 1_000_000
)

// Execute submits one supported administrator v1 action. It never retries an
// ambiguous write and returns an explicit unknown result when acceptance cannot be proven.
func (p *Provider) Execute(
	ctx context.Context,
	request resourcecap.OperationRequest,
) (*resourcecap.OperationResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if request.Capability == resourcecap.CapabilityResetPassword {
		return p.executeResetPassword(ctx, request)
	}
	if request.Capability == resourcecap.CapabilityProvision {
		return p.executeProvision(ctx, request)
	}
	if request.Capability == resourcecap.CapabilityEnterRescue {
		return p.executeEnterRescue(ctx, request)
	}
	if len(request.SecretBindings) != 0 {
		return nil, errInvalidConfig
	}
	call, err := buildOperationCall(request)
	if err != nil {
		return nil, err
	}
	if err := p.verifyOperationOwnership(ctx, request.ResourceRef, &call); err != nil {
		return nil, err
	}
	body := []byte(call.form.Encode())
	defer clear(body)
	return p.executeOperationBody(ctx, request, call, body)
}

func (p *Provider) executeOperationBody(
	ctx context.Context,
	request resourcecap.OperationRequest,
	call operationCall,
	body []byte,
) (*resourcecap.OperationResult, error) {
	response, err := p.authenticatedWriteRequest(ctx, call.method, call.route, call.query, body)
	if err != nil {
		var callError *resourcecap.ProviderCallError
		if errors.As(err, &callError) && !callError.ResultUnknown {
			return nil, resourcecap.NormalizeProviderError(callError)
		}
		if errors.Is(err, errAuthentication) {
			return nil, newProviderCallError(
				resourcecap.ProviderCallErrorCodeAuthentication,
				false,
				false,
			)
		}
		if errors.Is(err, errCircuitOpen) {
			return nil, newProviderCallError(
				resourcecap.ProviderCallErrorCodeTransport,
				true,
				false,
			)
		}
		return p.reconcileOperationPostcondition(ctx, request, call)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		if call.postconditionKind != "" && response.statusCode != http.StatusBadRequest &&
			response.statusCode != http.StatusForbidden && response.statusCode != http.StatusNotFound &&
			response.statusCode != http.StatusMethodNotAllowed {
			return p.reconcileOperationPostcondition(ctx, request, call)
		}
		return p.resultForRejectedStatus(request, response.statusCode)
	}
	return p.resultForAcceptedWrite(ctx, request, call, response.statusCode, response.body)
}

func (p *Provider) executeResetPassword(
	ctx context.Context,
	request resourcecap.OperationRequest,
) (*resourcecap.OperationResult, error) {
	parameters, err := decodeParameters(request.Parameters)
	if err != nil || len(parameters) != 0 || !validNumericReference(request.ResourceRef) {
		return nil, errInvalidConfig
	}
	handle, err := exactOperationSecret(request.SecretBindings, resetPasswordBinding)
	if err != nil {
		return nil, err
	}
	return p.executeSecretFormOperation(ctx, request, operationCall{
		method:       http.MethodPut,
		route:        "/clouds/" + request.ResourceRef + "/password",
		form:         url.Values{},
		requiresTask: true,
	}, handle, resetPasswordBinding)
}

func (p *Provider) executeProvision(
	ctx context.Context,
	request resourcecap.OperationRequest,
) (*resourcecap.OperationResult, error) {
	call, err := buildOperationCall(request)
	if err != nil {
		return nil, err
	}
	handle, err := exactOperationSecret(request.SecretBindings, resetPasswordBinding)
	if err != nil {
		return nil, err
	}
	return p.executeSecretFormOperation(ctx, request, call, handle, "rootpass")
}

func (p *Provider) executeEnterRescue(
	ctx context.Context,
	request resourcecap.OperationRequest,
) (*resourcecap.OperationResult, error) {
	call, err := buildOperationCall(request)
	if err != nil {
		return nil, err
	}
	handle, err := exactOperationSecret(request.SecretBindings, rescuePasswordBinding)
	if err != nil {
		return nil, err
	}
	return p.executeSecretFormOperation(ctx, request, call, handle, "temp_pass")
}

func (p *Provider) executeSecretFormOperation(
	ctx context.Context,
	request resourcecap.OperationRequest,
	call operationCall,
	handle secretcap.Handle,
	upstreamName string,
) (*resourcecap.OperationResult, error) {
	baseBody := []byte(call.form.Encode())
	defer clear(baseBody)
	var (
		result  *resourcecap.OperationResult
		callErr error
	)
	err := p.accessOperationSecret(ctx, handle, func(password []byte) error {
		body, encodeErr := appendConfidentialForm(baseBody, confidentialFormPair{
			name: upstreamName, value: password,
		})
		if encodeErr != nil {
			callErr = newProviderCallError(
				resourcecap.ProviderCallErrorCodeInternal,
				false,
				false,
			)
			return nil
		}
		defer clear(body)
		result, callErr = p.executeOperationBody(ctx, request, call, body)
		return nil
	})
	if err != nil {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeInternal, false, false)
	}
	return result, callErr
}

func newProviderCallError(
	code resourcecap.ProviderCallErrorCode,
	retryable bool,
	resultUnknown bool,
) error {
	callError, err := resourcecap.NewProviderCallError(code, retryable, resultUnknown)
	if err != nil {
		return resourcecap.NormalizeProviderError(err)
	}
	return callError
}

func buildOperationCall(request resourcecap.OperationRequest) (operationCall, error) {
	parameters, err := decodeParameters(request.Parameters)
	if err != nil {
		return operationCall{}, err
	}
	resourceRoute := func(suffix string) (string, error) {
		if !validNumericReference(request.ResourceRef) {
			return "", errInvalidConfig
		}
		return "/clouds/" + request.ResourceRef + suffix, nil
	}

	switch request.Capability {
	case resourcecap.CapabilityProvision:
		form, formErr := provisionForm(parameters)
		return operationCall{
			method: http.MethodPost, route: "/clouds", form: form, requiresTask: true,
		}, formErr
	case resourcecap.CapabilityStart:
		return noParameterResourceCall(parameters, http.MethodPost, resourceRoute, "/on", true)
	case resourcecap.CapabilityStop:
		return modeResourceCall(parameters, resourceRoute, "/off", "/hardoff")
	case resourcecap.CapabilityReboot:
		return modeResourceCall(parameters, resourceRoute, "/reboot", "/hard_reboot")
	case resourcecap.CapabilitySuspend:
		return suspendCall(parameters, resourceRoute)
	case resourcecap.CapabilityResume:
		return noParameterResourceCall(parameters, http.MethodPost, resourceRoute, "/unsuspend", true)
	case resourcecap.CapabilityResize:
		return resizeCall(parameters, resourceRoute)
	case resourcecap.CapabilityChangeIP:
		return changeIPCall(parameters, resourceRoute)
	case resourcecap.CapabilityManageSnapshot:
		return snapshotCall(parameters, request.ResourceRef, "snap")
	case resourcecap.CapabilityBackup:
		return snapshotCall(parameters, request.ResourceRef, "backup")
	case resourcecap.CapabilitySecurityGroup:
		return securityGroupCall(parameters, request.ResourceRef)
	case resourcecap.CapabilityNAT:
		return natForwardingCall(parameters, request.ResourceRef)
	case resourcecap.CapabilityVirtualMedia:
		return virtualMediaCall(parameters, request.ResourceRef)
	case resourcecap.CapabilityTrafficPackage:
		return trafficPackageCall(parameters, request.ResourceRef)
	case resourcecap.CapabilityConsole:
		return noParameterResourceCall(parameters, http.MethodPost, resourceRoute, "/vnc", false)
	case resourcecap.CapabilityReinstall:
		return reinstallCall(parameters, resourceRoute)
	case resourcecap.CapabilityEnterRescue:
		return enterRescueCall(parameters, resourceRoute)
	case resourcecap.CapabilityExitRescue:
		return noParameterResourceCall(parameters, http.MethodDelete, resourceRoute, "/rescue", true)
	case resourcecap.CapabilityTerminate:
		return noParameterResourceCall(parameters, http.MethodDelete, resourceRoute, "", true)
	default:
		return operationCall{}, errUnsupported
	}
}

func trafficPackageCall(parameters rawObject, resourceReference string) (operationCall, error) {
	if !validNumericReference(resourceReference) {
		return operationCall{}, errInvalidConfig
	}
	form, err := strictScalarForm(parameters, map[string]struct{}{
		"action": {}, "traffic_gb": {},
	})
	if err != nil || form.Get("action") != "add" || len(form) != 2 {
		return operationCall{}, errInvalidConfig
	}
	traffic := form.Get("traffic_gb")
	parsed, err := strconv.ParseUint(traffic, 10, 64)
	if !validNumericReference(traffic) || err != nil || parsed == 0 || parsed > maximumTrafficPackageGB {
		return operationCall{}, errInvalidConfig
	}
	return operationCall{
		method:            http.MethodPut,
		route:             "/clouds/" + resourceReference + "/temp_traffic",
		form:              url.Values{"traffic": {traffic}},
		ownershipKind:     "traffic_package_add",
		postconditionKind: "temporary_traffic",
	}, nil
}

func virtualMediaCall(parameters rawObject, resourceReference string) (operationCall, error) {
	if !validNumericReference(resourceReference) {
		return operationCall{}, errInvalidConfig
	}
	form, err := strictScalarForm(parameters, map[string]struct{}{
		"action": {}, "image_ref": {},
	})
	if err != nil {
		return operationCall{}, err
	}
	switch form.Get("action") {
	case "mount":
		isoReference := form.Get("image_ref")
		if !validNumericReference(isoReference) || len(form) != 2 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method:            http.MethodPost,
			route:             "/clouds/" + resourceReference + "/iso",
			form:              url.Values{"iso": {isoReference}},
			ownershipKind:     "virtual_media_mount",
			ownershipRef:      isoReference,
			postconditionKind: "virtual_media",
			postconditionRef:  isoReference,
		}, nil
	case "unmount":
		if len(form) != 1 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method:            http.MethodDelete,
			route:             "/clouds/" + resourceReference + "/iso",
			form:              url.Values{},
			ownershipKind:     "virtual_media_unmount",
			postconditionKind: "virtual_media",
			postconditionRef:  "0",
		}, nil
	default:
		return operationCall{}, errInvalidConfig
	}
}

func natForwardingCall(parameters rawObject, resourceReference string) (operationCall, error) {
	if !validNumericReference(resourceReference) {
		return operationCall{}, errInvalidConfig
	}
	form, err := strictScalarForm(parameters, map[string]struct{}{
		"action": {}, "nat_ref": {}, "name": {}, "external_port": {}, "internal_port": {}, "protocol": {},
	})
	if err != nil {
		return operationCall{}, err
	}
	switch form.Get("action") {
	case "create":
		name := form.Get("name")
		externalPort := form.Get("external_port")
		internalPort := form.Get("internal_port")
		protocol, ok := natProtocolValue(form.Get("protocol"))
		if !validNATName(name) || !validNATExternalPort(externalPort) ||
			!validNATPort(internalPort, false) || !ok || form.Get("nat_ref") != "" || len(form) != 5 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method: http.MethodPost,
			route:  "/clouds/" + resourceReference + "/nat_acl",
			form: url.Values{
				"name":     {name},
				"ext_port": {externalPort},
				"int_port": {internalPort},
				"protocol": {protocol},
			},
			ownershipKind:     "nat_create",
			postconditionKind: "nat_present",
		}, nil
	case "delete":
		natReference := form.Get("nat_ref")
		if !validNumericReference(natReference) || len(form) != 2 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method:            http.MethodDelete,
			route:             "/nat_acl/" + natReference,
			query:             url.Values{"hostid": {resourceReference}},
			form:              url.Values{},
			ownershipKind:     "nat_delete",
			ownershipRef:      natReference,
			postconditionKind: "nat_absent",
			postconditionRef:  natReference,
		}, nil
	default:
		return operationCall{}, errInvalidConfig
	}
}

func securityGroupCall(parameters rawObject, resourceReference string) (operationCall, error) {
	if !validNumericReference(resourceReference) {
		return operationCall{}, errInvalidConfig
	}
	form, err := strictScalarForm(parameters, map[string]struct{}{
		"action": {}, "security_group_ref": {},
	})
	if err != nil {
		return operationCall{}, err
	}
	switch form.Get("action") {
	case "attach":
		groupReference := form.Get("security_group_ref")
		if !validNumericReference(groupReference) || len(form) != 2 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method: http.MethodPost,
			route:  "/security_groups/" + groupReference + "/links",
			form: url.Values{
				"cloud": {resourceReference},
				"type":  {"1"},
			},
			ownershipKind:     "security_group_attach",
			ownershipRef:      groupReference,
			postconditionKind: "security_group",
			postconditionRef:  groupReference,
		}, nil
	case "detach":
		if len(form) != 1 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method:            http.MethodDelete,
			route:             "/clouds/" + resourceReference + "/security_groups",
			form:              url.Values{},
			ownershipKind:     "security_group_detach",
			postconditionKind: "security_group",
			postconditionRef:  "0",
		}, nil
	default:
		return operationCall{}, errInvalidConfig
	}
}

func snapshotCall(
	parameters rawObject,
	resourceReference string,
	snapshotType string,
) (operationCall, error) {
	if !validNumericReference(resourceReference) ||
		(snapshotType != "snap" && snapshotType != "backup") {
		return operationCall{}, errInvalidConfig
	}
	form, err := strictScalarForm(parameters, map[string]struct{}{
		"action": {}, "disk_ref": {}, "snapshot_ref": {}, "name": {},
	})
	if err != nil {
		return operationCall{}, err
	}
	switch form.Get("action") {
	case "create":
		diskReference := form.Get("disk_ref")
		name := form.Get("name")
		if !validNumericReference(diskReference) || !validSnapshotName(name) ||
			form.Get("snapshot_ref") != "" || len(form) != 3 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method: http.MethodPost,
			route:  "/disks/" + diskReference + "/snapshots",
			form: url.Values{
				"type": {snapshotType},
				"name": {name},
			},
			ownershipKind: "disk",
			ownershipRef:  diskReference,
			requiresTask:  true,
		}, nil
	case "delete":
		snapshotReference := form.Get("snapshot_ref")
		if !validNumericReference(snapshotReference) || len(form) != 2 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method:        http.MethodDelete,
			route:         "/snapshots/" + snapshotReference,
			form:          url.Values{},
			ownershipKind: "snapshot", ownershipRef: snapshotReference, ownershipType: snapshotType,
			requiresTask: true,
		}, nil
	case "restore":
		snapshotReference := form.Get("snapshot_ref")
		if !validNumericReference(snapshotReference) || len(form) != 2 {
			return operationCall{}, errInvalidConfig
		}
		return operationCall{
			method:        http.MethodPost,
			route:         "/snapshots/" + snapshotReference + "/restore",
			query:         url.Values{"hostid": {resourceReference}},
			form:          url.Values{},
			ownershipKind: "snapshot", ownershipRef: snapshotReference, ownershipType: snapshotType,
			requiresTask: true,
		}, nil
	default:
		return operationCall{}, errInvalidConfig
	}
}

func (p *Provider) verifyOperationOwnership(
	ctx context.Context,
	resourceReference string,
	call *operationCall,
) error {
	if call == nil || call.ownershipKind == "" {
		return nil
	}
	if !validNumericReference(resourceReference) {
		return errInvalidConfig
	}
	switch call.ownershipKind {
	case "disk":
		if !validNumericReference(call.ownershipRef) {
			return errInvalidConfig
		}
		response, err := p.authenticatedRequest(
			ctx, http.MethodGet, "/clouds/"+resourceReference, nil, nil,
		)
		if err != nil {
			return normalizeReadCallError(err)
		}
		defer clear(response.body)
		if response.statusCode < 200 || response.statusCode >= 300 {
			return readStatusError(response.statusCode)
		}
		object, err := decodeObject(response.body)
		if err != nil {
			return errInvalidResponse
		}
		object = unwrapObject(object)
		defer clearRawObject(object)
		disks, err := decodeRawObjectArray(object["disk"])
		if err != nil {
			return errInvalidResponse
		}
		defer clearRawObjects(disks)
		return verifyOwnedReference(disks, call.ownershipRef, "")
	case "snapshot":
		if !validNumericReference(call.ownershipRef) {
			return errInvalidConfig
		}
		response, err := p.authenticatedRequest(
			ctx,
			http.MethodGet,
			"/clouds/"+resourceReference+"/snapshots",
			url.Values{"page": {"1"}, "per_page": {"9999"}},
			nil,
		)
		if err != nil {
			return normalizeReadCallError(err)
		}
		defer clear(response.body)
		if response.statusCode < 200 || response.statusCode >= 300 {
			return readStatusError(response.statusCode)
		}
		items, meta, err := decodeObjectPage(response.body)
		if err != nil {
			return errInvalidResponse
		}
		defer clearRawObjects(items)
		defer clearRawObject(meta)
		if len(items) >= 9999 {
			return errInvalidResponse
		}
		return verifyOwnedReference(items, call.ownershipRef, call.ownershipType)
	case "security_group_attach", "security_group_detach":
		return p.verifySecurityGroupOwnership(ctx, resourceReference, call)
	case "nat_create":
		return p.verifyNATCreateOwnership(ctx, resourceReference, *call)
	case "nat_delete":
		return p.verifyNATDeleteOwnership(ctx, resourceReference, call.ownershipRef)
	case "virtual_media_mount", "virtual_media_unmount":
		return p.verifyVirtualMediaOwnership(ctx, resourceReference, call)
	case "traffic_package_add":
		identity, err := p.readCloudTrafficIdentity(ctx, resourceReference)
		if err != nil {
			return err
		}
		amount, err := strconv.ParseUint(call.form.Get("traffic"), 10, 64)
		if err != nil || amount == 0 || amount > maximumTrafficPackageGB || identity.trafficQuota == 0 ||
			identity.temporaryTraffic > ^uint64(0)-amount {
			return errInvalidConfig
		}
		call.postconditionRef = strconv.FormatUint(identity.temporaryTraffic+amount, 10)
		return nil
	default:
		return errInvalidConfig
	}
}

type cloudTrafficIdentity struct {
	trafficQuota     uint64
	temporaryTraffic uint64
}

func (p *Provider) readCloudTrafficIdentity(
	ctx context.Context,
	resourceReference string,
) (cloudTrafficIdentity, error) {
	response, err := p.authenticatedRequest(
		ctx, http.MethodGet, "/clouds/"+resourceReference, nil, nil,
	)
	if err != nil {
		return cloudTrafficIdentity{}, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return cloudTrafficIdentity{}, readStatusError(response.statusCode)
	}
	object, err := decodeObject(response.body)
	if err != nil {
		return cloudTrafficIdentity{}, errInvalidResponse
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	trafficQuota, quotaOK := nonNegativeUint64Scalar(object["traffic_quota"])
	temporaryTraffic, temporaryOK := nonNegativeUint64Scalar(object["tmp_traffic"])
	if !quotaOK || !temporaryOK {
		return cloudTrafficIdentity{}, errInvalidResponse
	}
	return cloudTrafficIdentity{trafficQuota: trafficQuota, temporaryTraffic: temporaryTraffic}, nil
}

func nonNegativeUint64Scalar(raw json.RawMessage) (uint64, bool) {
	value, ok := formScalar(raw)
	if !ok || value == "" || len(value) > 20 || (len(value) > 1 && value[0] == '0') {
		return 0, false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}

func (p *Provider) verifySecurityGroupOwnership(
	ctx context.Context,
	resourceReference string,
	call *operationCall,
) error {
	identity, err := p.readCloudSecurityIdentity(ctx, resourceReference)
	if err != nil {
		return err
	}
	if call.ownershipKind == "security_group_detach" {
		if !validNumericReference(identity.securityGroupReference) {
			return errInvalidConfig
		}
		call.ownershipRef = identity.securityGroupReference
		return nil
	}
	if call.ownershipKind != "security_group_attach" || !validNumericReference(call.ownershipRef) ||
		!validNumericReference(identity.userReference) || identity.resourceType == "" ||
		identity.securityGroupReference != "0" {
		return errInvalidConfig
	}
	response, err := p.authenticatedRequest(
		ctx,
		http.MethodGet,
		"/security_groups",
		url.Values{
			"list_type": {"all"},
			"type":      {identity.resourceType},
			"user":      {identity.userReference},
		},
		nil,
	)
	if err != nil {
		return normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return readStatusError(response.statusCode)
	}
	items, meta, err := decodeObjectPage(response.body)
	if err != nil {
		return errInvalidResponse
	}
	defer clearRawObjects(items)
	defer clearRawObject(meta)
	return verifyOwnedReference(items, call.ownershipRef, identity.resourceType)
}

func (p *Provider) verifyVirtualMediaOwnership(
	ctx context.Context,
	resourceReference string,
	call *operationCall,
) error {
	identity, err := p.readCloudVirtualMediaIdentity(ctx, resourceReference)
	if err != nil {
		return err
	}
	if call.ownershipKind == "virtual_media_unmount" {
		if !validNumericReference(identity.isoReference) {
			return errInvalidConfig
		}
		call.ownershipRef = identity.isoReference
		call.form.Set("iso", identity.isoReference)
		return nil
	}
	if call.ownershipKind != "virtual_media_mount" || identity.isoReference != "0" ||
		!validNumericReference(identity.nodeReference) || !validNumericReference(call.ownershipRef) {
		return errInvalidConfig
	}
	return p.verifyNodeISO(ctx, identity.nodeReference, call.ownershipRef)
}

type cloudVirtualMediaIdentity struct {
	nodeReference string
	isoReference  string
}

func (p *Provider) readCloudVirtualMediaIdentity(
	ctx context.Context,
	resourceReference string,
) (cloudVirtualMediaIdentity, error) {
	response, err := p.authenticatedRequest(
		ctx, http.MethodGet, "/clouds/"+resourceReference, nil, nil,
	)
	if err != nil {
		return cloudVirtualMediaIdentity{}, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return cloudVirtualMediaIdentity{}, readStatusError(response.statusCode)
	}
	object, err := decodeObject(response.body)
	if err != nil {
		return cloudVirtualMediaIdentity{}, errInvalidResponse
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	nodeReference, _ := firstReference(object, "node_id", "node")
	isoReference, ok := nonNegativeReference(object["iso"])
	if !ok {
		return cloudVirtualMediaIdentity{}, errInvalidResponse
	}
	return cloudVirtualMediaIdentity{nodeReference: nodeReference, isoReference: isoReference}, nil
}

func (p *Provider) verifyNodeISO(
	ctx context.Context,
	nodeReference string,
	isoReference string,
) error {
	response, err := p.authenticatedRequest(
		ctx,
		http.MethodGet,
		"/node_isos",
		url.Values{"id": {nodeReference}, "type": {"node"}},
		nil,
	)
	if err != nil {
		return normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return readStatusError(response.statusCode)
	}
	root, err := decodeObject(response.body)
	if err != nil {
		return errInvalidResponse
	}
	defer clearRawObject(root)
	groups, err := decodeRawObjectArray(root["data"])
	if err != nil {
		return errInvalidResponse
	}
	defer clearRawObjects(groups)
	if len(groups) > maximumOwnershipCandidateCount {
		return errInvalidResponse
	}
	matches := 0
	candidates := 0
	for _, group := range groups {
		items, decodeErr := decodeRawObjectArray(group["info"])
		if decodeErr != nil {
			return errInvalidResponse
		}
		for _, item := range items {
			candidates++
			if candidates > maximumOwnershipCandidateCount {
				clearRawObjects(items)
				return errInvalidResponse
			}
			itemReference, ok := firstReference(item, "id")
			if !ok {
				clearRawObjects(items)
				return errInvalidResponse
			}
			if itemReference == isoReference {
				matches++
			}
		}
		clearRawObjects(items)
	}
	if matches != 1 {
		return errInvalidConfig
	}
	return nil
}

func (p *Provider) verifyNATCreateOwnership(
	ctx context.Context,
	resourceReference string,
	call operationCall,
) error {
	entries, err := p.readNATForwardings(ctx, resourceReference)
	if err != nil {
		return err
	}
	target, err := natForwardingFromCall(call)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.externalPort == target.externalPort &&
			natProtocolsOverlap(entry.protocol, target.protocol) {
			return errInvalidConfig
		}
	}
	return nil
}

func (p *Provider) verifyNATDeleteOwnership(
	ctx context.Context,
	resourceReference string,
	natReference string,
) error {
	if !validNumericReference(natReference) {
		return errInvalidConfig
	}
	entries, err := p.readNATForwardings(ctx, resourceReference)
	if err != nil {
		return err
	}
	matches := 0
	for _, entry := range entries {
		if entry.reference == natReference {
			matches++
		}
	}
	if matches != 1 {
		return errInvalidConfig
	}
	return nil
}

type natForwarding struct {
	reference    string
	name         string
	externalPort string
	internalPort string
	protocol     string
}

func (p *Provider) readNATForwardings(
	ctx context.Context,
	resourceReference string,
) ([]natForwarding, error) {
	response, err := p.authenticatedRequest(
		ctx,
		http.MethodGet,
		"/clouds/"+resourceReference+"/nat_acl",
		url.Values{"list_type": {"all"}},
		nil,
	)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return nil, readStatusError(response.statusCode)
	}
	items, meta, err := decodeObjectPage(response.body)
	if err != nil {
		return nil, errInvalidResponse
	}
	defer clearRawObjects(items)
	defer clearRawObject(meta)
	if len(items) > maximumOwnershipCandidateCount {
		return nil, errInvalidResponse
	}
	entries := make([]natForwarding, 0, len(items))
	for _, item := range items {
		reference, referenceOK := firstReference(item, "id")
		name, nameOK := stringScalar(item["name"])
		externalPort, externalPortOK := formScalar(item["ext_port"])
		internalPort, internalPortOK := formScalar(item["int_port"])
		protocol, protocolOK := formScalar(item["protocol"])
		if !referenceOK || !nameOK || !validNATName(name) || !externalPortOK ||
			!validNATPort(externalPort, true) || !internalPortOK || !validNATPort(internalPort, true) ||
			!protocolOK || !validNATProtocolValue(protocol) {
			return nil, errInvalidResponse
		}
		entries = append(entries, natForwarding{
			reference: reference, name: name, externalPort: externalPort,
			internalPort: internalPort, protocol: protocol,
		})
	}
	return entries, nil
}

func natForwardingFromCall(call operationCall) (natForwarding, error) {
	target := natForwarding{
		name:         call.form.Get("name"),
		externalPort: call.form.Get("ext_port"),
		internalPort: call.form.Get("int_port"),
		protocol:     call.form.Get("protocol"),
	}
	if !validNATName(target.name) || !validNATExternalPort(target.externalPort) ||
		!validNATPort(target.internalPort, false) || !validNATProtocolValue(target.protocol) {
		return natForwarding{}, errInvalidConfig
	}
	return target, nil
}

func validNATName(value string) bool {
	return validSnapshotName(value)
}

func validNATPort(value string, allowZero bool) bool {
	if value == "0" {
		return allowZero
	}
	if !validNumericReference(value) {
		return false
	}
	parsed, err := strconv.ParseUint(value, 10, 16)
	return err == nil && parsed <= 65535
}

func validNATExternalPort(value string) bool {
	if !validNATPort(value, false) {
		return false
	}
	return value != "22" && value != "80" && value != "443"
}

func natProtocolValue(value string) (string, bool) {
	switch value {
	case "tcp":
		return "1", true
	case "udp":
		return "2", true
	case "tcp_udp":
		return "3", true
	default:
		return "", false
	}
}

func validNATProtocolValue(value string) bool {
	return value == "1" || value == "2" || value == "3"
}

func natProtocolsOverlap(left string, right string) bool {
	return left == "3" || right == "3" || left == right
}

type cloudSecurityIdentity struct {
	userReference          string
	resourceType           string
	securityGroupReference string
}

func (p *Provider) readCloudSecurityIdentity(
	ctx context.Context,
	resourceReference string,
) (cloudSecurityIdentity, error) {
	response, err := p.authenticatedRequest(
		ctx, http.MethodGet, "/clouds/"+resourceReference, nil, nil,
	)
	if err != nil {
		return cloudSecurityIdentity{}, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return cloudSecurityIdentity{}, readStatusError(response.statusCode)
	}
	object, err := decodeObject(response.body)
	if err != nil {
		return cloudSecurityIdentity{}, errInvalidResponse
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	userReference, _ := firstReference(object, "user_id", "userid", "uid")
	resourceType, _ := stringScalar(object["type"])
	if resourceType != "" && !validUpstreamType(resourceType) {
		return cloudSecurityIdentity{}, errInvalidResponse
	}
	securityGroupReference, ok := nonNegativeReference(object["security"])
	if !ok {
		return cloudSecurityIdentity{}, errInvalidResponse
	}
	return cloudSecurityIdentity{
		userReference:          userReference,
		resourceType:           resourceType,
		securityGroupReference: securityGroupReference,
	}, nil
}

func validUpstreamType(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for index := range value {
		character := value[index]
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func nonNegativeReference(raw json.RawMessage) (string, bool) {
	value, ok := formScalar(raw)
	if !ok || value == "" || len(value) > 20 {
		return "", false
	}
	if value == "0" {
		return value, true
	}
	return value, validNumericReference(value)
}

func verifyOwnedReference(items []rawObject, reference string, expectedType string) error {
	if len(items) > maximumOwnershipCandidateCount {
		return errInvalidResponse
	}
	matches := 0
	for _, item := range items {
		itemReference, ok := firstReference(item, "id")
		if !ok {
			return errInvalidResponse
		}
		if itemReference != reference {
			continue
		}
		if expectedType != "" {
			itemType, ok := stringScalar(item["type"])
			if !ok || itemType != expectedType {
				return errInvalidConfig
			}
		}
		matches++
	}
	if matches != 1 {
		return errInvalidConfig
	}
	return nil
}

func validSnapshotName(value string) bool {
	if value == "" || len(value) > maximumSnapshotNameBytes || !utf8.ValidString(value) ||
		value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func enterRescueCall(
	parameters rawObject,
	routeBuilder func(string) (string, error),
) (operationCall, error) {
	form, err := strictScalarForm(parameters, map[string]struct{}{"system": {}})
	if err != nil {
		return operationCall{}, err
	}
	system := form.Get("system")
	switch system {
	case "linux":
		form.Set("type", "1")
	case "windows":
		form.Set("type", "2")
	default:
		return operationCall{}, errInvalidConfig
	}
	form.Del("system")
	route, err := routeBuilder("/rescue")
	return operationCall{
		method: http.MethodPost, route: route, form: form, requiresTask: true,
	}, err
}

func decodeParameters(raw json.RawMessage) (rawObject, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return rawObject{}, nil
	}
	return decodeObject(raw)
}

func provisionForm(parameters rawObject) (url.Values, error) {
	allowed := map[string]struct{}{
		"area": {}, "node": {}, "node_group": {}, "node_priority": {}, "os": {}, "cpu": {}, "memory": {},
		"system_disk_size": {}, "data_disk_size": {}, "network_type": {}, "vpc": {}, "client": {}, "hostname": {}, "ip_group": {},
		"ip_num": {}, "traffic_type": {}, "in_bw": {}, "out_bw": {}, "traffic_quota": {}, "backup_num": {},
		"snap_num": {}, "nat_acl_limit": {}, "nat_web_limit": {}, "advanced_cpu": {}, "advanced_bw": {}, "num": {},
	}
	form, err := strictScalarForm(parameters, allowed)
	if err != nil {
		return nil, err
	}
	for _, required := range []string{"cpu", "memory", "os", "client"} {
		if !positiveDecimal(form.Get(required)) {
			return nil, errInvalidConfig
		}
	}
	if !validHostname(form.Get("hostname")) {
		return nil, errInvalidConfig
	}
	memory, err := strconv.ParseUint(form.Get("memory"), 10, 63)
	if err != nil || memory < 128 {
		return nil, errInvalidConfig
	}
	if number := form.Get("num"); number != "" && number != "1" {
		return nil, errInvalidConfig
	}
	if value := form.Get("data_disk_size"); value != "" {
		capacity, parseErr := strconv.ParseUint(value, 10, 63)
		if parseErr != nil || capacity > 1048576 || value != strconv.FormatUint(capacity, 10) {
			return nil, errInvalidConfig
		}
	}
	// These quota fields use supplier sentinel values, not capacities:
	// -1 disables the feature, 0 is unlimited, positive integers are quotas.
	for _, key := range []string{"backup_num", "snap_num", "nat_acl_limit", "nat_web_limit"} {
		if value, present := form[key]; present {
			quota, parseErr := strconv.ParseInt(value[0], 10, 64)
			if parseErr != nil || quota < -1 || value[0] != strconv.FormatInt(quota, 10) {
				return nil, errInvalidConfig
			}
		}
	}
	form.Set("num", "1")
	for _, key := range []string{"area", "node", "node_group", "vpc", "ip_group"} {
		if value := form.Get(key); value != "" && !positiveDecimal(value) {
			return nil, errInvalidConfig
		}
	}
	return form, nil
}

func validHostname(value string) bool {
	if value == "" || len(value) > 253 || value[0] == '.' || value[len(value)-1] == '.' {
		return false
	}
	labelLength := 0
	for index := range value {
		character := value[index]
		if character == '.' {
			if labelLength == 0 || labelLength > 63 || value[index-1] == '-' {
				return false
			}
			labelLength = 0
			continue
		}
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' {
			return false
		}
		if labelLength == 0 && character == '-' {
			return false
		}
		labelLength++
	}
	return labelLength > 0 && labelLength <= 63 && value[len(value)-1] != '-'
}

func noParameterResourceCall(
	parameters rawObject,
	method string,
	routeBuilder func(string) (string, error),
	suffix string,
	requiresTask bool,
) (operationCall, error) {
	if len(parameters) != 0 {
		return operationCall{}, errInvalidConfig
	}
	route, err := routeBuilder(suffix)
	return operationCall{method: method, route: route, form: url.Values{}, requiresTask: requiresTask}, err
}

func modeResourceCall(
	parameters rawObject,
	routeBuilder func(string) (string, error),
	gracefulSuffix string,
	hardSuffix string,
) (operationCall, error) {
	form, err := strictScalarForm(parameters, map[string]struct{}{"mode": {}})
	if err != nil {
		return operationCall{}, err
	}
	mode := form.Get("mode")
	if mode == "" || mode == "graceful" {
		mode = "graceful"
	} else if mode != "hard" {
		return operationCall{}, errInvalidConfig
	}
	suffix := gracefulSuffix
	if mode == "hard" {
		suffix = hardSuffix
	}
	route, err := routeBuilder(suffix)
	return operationCall{method: http.MethodPost, route: route, form: url.Values{}, requiresTask: true}, err
}

func suspendCall(
	parameters rawObject,
	routeBuilder func(string) (string, error),
) (operationCall, error) {
	form, err := strictScalarForm(parameters, map[string]struct{}{"reason": {}, "free_gpu": {}})
	if err != nil {
		return operationCall{}, err
	}
	reason := form.Get("reason")
	if reason == "" {
		reason = "other"
	}
	if reason != "traffic" && reason != "due" && reason != "other" {
		return operationCall{}, errInvalidConfig
	}
	form.Del("reason")
	form.Set("type", reason)
	if freeGPU := form.Get("free_gpu"); freeGPU != "" && freeGPU != "true" && freeGPU != "false" {
		return operationCall{}, errInvalidConfig
	}
	route, err := routeBuilder("/suspend")
	return operationCall{method: http.MethodPost, route: route, form: form, requiresTask: true}, err
}

func resizeCall(
	parameters rawObject,
	routeBuilder func(string) (string, error),
) (operationCall, error) {
	allowed := map[string]struct{}{
		"cpu": {}, "memory": {}, "in_bw": {}, "out_bw": {}, "system_disk_size": {},
	}
	form, err := strictScalarForm(parameters, allowed)
	if err != nil || len(form) == 0 {
		return operationCall{}, errInvalidConfig
	}
	for _, values := range form {
		if len(values) != 1 || !positiveDecimal(values[0]) {
			return operationCall{}, errInvalidConfig
		}
	}
	route, err := routeBuilder("")
	return operationCall{method: http.MethodPut, route: route, form: form, requiresTask: true}, err
}

func changeIPCall(
	parameters rawObject,
	routeBuilder func(string) (string, error),
) (operationCall, error) {
	form, err := strictScalarForm(parameters, map[string]struct{}{"ip_num": {}, "ip_group": {}})
	if err != nil || !positiveDecimal(form.Get("ip_num")) {
		return operationCall{}, errInvalidConfig
	}
	if group := form.Get("ip_group"); group != "" && !positiveDecimal(group) {
		return operationCall{}, errInvalidConfig
	}
	form.Set("num", form.Get("ip_num"))
	form.Del("ip_num")
	route, err := routeBuilder("/ip")
	return operationCall{
		method: http.MethodPut, route: route, form: form, queryAfterSuccess: true,
	}, err
}

func reinstallCall(
	parameters rawObject,
	routeBuilder func(string) (string, error),
) (operationCall, error) {
	form, err := strictScalarForm(parameters, map[string]struct{}{
		"os": {}, "port": {}, "format_data_disk": {}, "system_disk_size": {},
	})
	if err != nil || !positiveDecimal(form.Get("os")) {
		return operationCall{}, errInvalidConfig
	}
	if port := form.Get("port"); port != "" {
		parsed, parseErr := strconv.ParseUint(port, 10, 16)
		if parseErr != nil || parsed == 0 {
			return operationCall{}, errInvalidConfig
		}
	}
	if formatDataDisk := form.Get("format_data_disk"); formatDataDisk != "" && formatDataDisk != "0" && formatDataDisk != "1" {
		return operationCall{}, errInvalidConfig
	}
	if systemDiskSize := form.Get("system_disk_size"); systemDiskSize != "" && !positiveDecimal(systemDiskSize) {
		return operationCall{}, errInvalidConfig
	}
	route, err := routeBuilder("/reinstall")
	return operationCall{
		method: http.MethodPut, route: route, form: form, requiresTask: true,
	}, err
}

func strictScalarForm(parameters rawObject, allowed map[string]struct{}) (url.Values, error) {
	form := url.Values{}
	for key, raw := range parameters {
		if _, ok := allowed[key]; !ok {
			return nil, errInvalidConfig
		}
		value, ok := formScalar(raw)
		if !ok {
			return nil, errInvalidConfig
		}
		form.Set(key, value)
	}
	return form, nil
}

func positiveDecimal(value string) bool {
	if !validNumericReference(value) {
		return false
	}
	parsed, err := strconv.ParseUint(value, 10, 63)
	return err == nil && parsed > 0
}

func (p *Provider) resultForAcceptedWrite(
	ctx context.Context,
	request resourcecap.OperationRequest,
	call operationCall,
	statusCode int,
	body []byte,
) (*resourcecap.OperationResult, error) {
	if call.queryAfterSuccess {
		return p.unknownWriteResult(request)
	}
	object, err := decodeObject(body)
	if err != nil {
		return p.reconcileOperationPostcondition(ctx, request, call)
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	taskReference, hasTask := firstReference(object, "taskid", "task_id")
	resourceReference, hasResource := firstReference(object, "id", "hostid", "host_id")
	if request.ResourceRef != "" {
		resourceReference = request.ResourceRef
		hasResource = true
	}
	if call.requiresTask && !hasTask {
		return p.unknownWriteResult(request)
	}
	if call.postconditionKind != "" && !hasTask {
		return p.reconcileOperationPostcondition(ctx, request, call)
	}
	if request.Capability == resourcecap.CapabilityConsole &&
		(statusCode == http.StatusAccepted || hasTask) {
		return p.unknownWriteResult(request)
	}
	status := resourcecap.TaskStatusSucceeded
	if hasTask {
		status = resourcecap.TaskStatusAccepted
	}
	result := &resourcecap.OperationResult{
		Status:              status,
		ExternalTaskRef:     taskReference,
		ExternalResourceRef: resourceReference,
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_acceptance",
			Reference:  request.RequestID,
			Summary:    "upstream acceptance identifiers were field-whitelisted",
			ObservedAt: p.now(),
		}},
	}
	if !hasResource {
		result.ExternalResourceRef = ""
	}
	switch request.Capability {
	case resourcecap.CapabilityReinstall:
		plaintext, outputErr := buildReinstallCredential(object)
		if outputErr != nil {
			return p.unknownWriteResult(request)
		}
		output, outputErr := p.storeSecretOutput(
			ctx,
			request,
			reinstallOutputName,
			resourcecap.SecretOutputPolicyOneTimeCredential,
			time.Time{},
			plaintext,
		)
		if outputErr != nil {
			return p.unknownWriteResult(request)
		}
		result.SecretOutputs = []resourcecap.SecretOutput{output}
	case resourcecap.CapabilityConsole:
		plaintext, outputErr := p.buildConsoleTicket(object)
		if outputErr != nil {
			return p.unknownWriteResult(request)
		}
		expiresAt := p.now().Add(consoleTicketLifetime)
		output, outputErr := p.storeSecretOutput(
			ctx,
			request,
			consoleOutputName,
			resourcecap.SecretOutputPolicyShortLivedTicket,
			expiresAt,
			plaintext,
		)
		if outputErr != nil {
			return p.unknownWriteResult(request)
		}
		result.SecretOutputs = []resourcecap.SecretOutput{output}
	}
	if err := result.ValidateFor(request); err != nil {
		return nil, errInvalidResponse
	}
	return result, nil
}

func (p *Provider) reconcileOperationPostcondition(
	ctx context.Context,
	request resourcecap.OperationRequest,
	call operationCall,
) (*resourcecap.OperationResult, error) {
	proved := false
	var summary string
	switch call.postconditionKind {
	case "security_group":
		if call.postconditionRef != "0" && !validNumericReference(call.postconditionRef) {
			return p.unknownWriteResult(request)
		}
		identity, err := p.readCloudSecurityIdentity(ctx, request.ResourceRef)
		proved = err == nil && identity.securityGroupReference == call.postconditionRef
		summary = "upstream resource state proved the requested security-group relation"
	case "nat_present":
		target, err := natForwardingFromCall(call)
		if err != nil {
			return p.unknownWriteResult(request)
		}
		entries, err := p.readNATForwardings(ctx, request.ResourceRef)
		if err == nil {
			matches := 0
			for _, entry := range entries {
				if entry.name == target.name && entry.externalPort == target.externalPort &&
					entry.internalPort == target.internalPort && entry.protocol == target.protocol {
					matches++
				}
			}
			proved = matches == 1
		}
		summary = "upstream instance NAT list proved the requested forwarding exists exactly once"
	case "nat_absent":
		if !validNumericReference(call.postconditionRef) {
			return p.unknownWriteResult(request)
		}
		entries, err := p.readNATForwardings(ctx, request.ResourceRef)
		if err == nil {
			proved = true
			for _, entry := range entries {
				if entry.reference == call.postconditionRef {
					proved = false
					break
				}
			}
		}
		summary = "upstream instance NAT list proved the requested forwarding is absent"
	case "virtual_media":
		if call.postconditionRef != "0" && !validNumericReference(call.postconditionRef) {
			return p.unknownWriteResult(request)
		}
		identity, err := p.readCloudVirtualMediaIdentity(ctx, request.ResourceRef)
		proved = err == nil && identity.isoReference == call.postconditionRef
		summary = "upstream resource state proved the requested virtual-media relation"
	case "temporary_traffic":
		target, err := strconv.ParseUint(call.postconditionRef, 10, 64)
		if err != nil {
			return p.unknownWriteResult(request)
		}
		identity, err := p.readCloudTrafficIdentity(ctx, request.ResourceRef)
		proved = err == nil && identity.temporaryTraffic == target
		summary = "upstream resource state proved the requested temporary-traffic increment"
	default:
		return p.unknownWriteResult(request)
	}
	if !proved {
		return p.unknownWriteResult(request)
	}
	result := &resourcecap.OperationResult{
		Status:              resourcecap.TaskStatusSucceeded,
		ExternalResourceRef: request.ResourceRef,
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_postcondition",
			Reference:  request.RequestID,
			Summary:    summary,
			ObservedAt: p.now(),
		}},
	}
	if err := result.ValidateFor(request); err != nil {
		return nil, errInvalidResponse
	}
	return result, nil
}

func (p *Provider) resultForRejectedStatus(
	request resourcecap.OperationRequest,
	statusCode int,
) (*resourcecap.OperationResult, error) {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusForbidden &&
		statusCode != http.StatusNotFound && statusCode != http.StatusMethodNotAllowed {
		return p.unknownWriteResult(request)
	}
	result := &resourcecap.OperationResult{
		Status:              resourcecap.TaskStatusFailed,
		ExternalResourceRef: request.ResourceRef,
		Failure: &resourcecap.ProviderFailure{
			Code:      "UPSTREAM_REJECTED",
			Message:   "upstream rejected the operation before acceptance was proven",
			Retryable: false,
		},
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_rejection",
			Reference:  request.RequestID,
			Summary:    "upstream returned a documented rejection status",
			ObservedAt: p.now(),
		}},
	}
	if err := result.ValidateFor(request); err != nil {
		return nil, errInvalidResponse
	}
	return result, nil
}

func (p *Provider) unknownWriteResult(
	request resourcecap.OperationRequest,
) (*resourcecap.OperationResult, error) {
	result := &resourcecap.OperationResult{
		Status:              resourcecap.TaskStatusUnknown,
		ExternalResourceRef: request.ResourceRef,
		Failure: &resourcecap.ProviderFailure{
			Code:          "UPSTREAM_RESULT_UNKNOWN",
			Message:       "upstream write result is unknown and requires task or resource reconciliation",
			Retryable:     false,
			ResultUnknown: true,
		},
		Evidence: []resourcecap.Evidence{{
			Kind:       "unknown_write",
			Reference:  request.RequestID,
			Summary:    "automatic replay is blocked until reconciliation",
			ObservedAt: p.now(),
		}},
	}
	if err := result.ValidateFor(request); err != nil {
		return nil, errInvalidResponse
	}
	return result, nil
}
