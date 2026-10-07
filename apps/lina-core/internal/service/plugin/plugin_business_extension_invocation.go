// This file implements the owner-bound business-extension method seam with
// exact release evidence, lifecycle checks, bounded JSON, and isolated dispatch.

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	pluginv1 "lina-core/api/plugin/v1"
	"lina-core/internal/service/plugin/internal/businessdiscovery"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/internal/service/plugin/internal/store"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	"lina-core/pkg/plugin/pluginhost"
	"lina-core/pkg/statusflag"
)

const (
	businessExtensionInvocationTimeout       = 5 * time.Second
	businessExtensionRequestMaxBytes         = 64 * 1024
	businessExtensionRequestPayloadMaxBytes  = businessExtensionRequestMaxBytes - 4096
	businessExtensionResponsePayloadMaxBytes = 1 * 1024 * 1024
	businessExtensionResponseMaxBytes        = businessExtensionResponsePayloadMaxBytes + 16*1024
	businessExtensionJSONMaxDepth            = 16
	businessExtensionJSONMaxArrayItems       = 512
	businessExtensionJSONMaxObjectFields     = 128
	businessExtensionJSONMaxTotalNodes       = 262144
	businessExtensionJSONMaxStringBytes      = 8 * 1024
	businessExtensionContextIDMinBytes       = 32
	businessExtensionContextIDMaxBytes       = 128
)

var businessExtensionContextIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var businessExtensionSensitiveKeys = map[string]struct{}{
	"authorization": {}, "cookie": {}, "cookies": {}, "credential": {}, "credentials": {},
	"password": {}, "secret": {}, "secrethandle": {}, "secretref": {}, "token": {},
	"accesstoken": {}, "refreshtoken": {},
}

// InvokeProviderMethod executes one exact provider method under the provider's
// lifecycle lock. New-business calls also require accepting state, while
// existing-business calls remain available after sales are stopped.
func (r *sourceBusinessExtensionRegistrar) InvokeProviderMethod(
	ctx context.Context,
	input pluginhost.BusinessExtensionMethodInvokeInput,
) (*capmodel.BusinessExtensionMethodResult, error) {
	if r == nil || r.service == nil {
		return nil, gerror.New("business extension registrar is unavailable")
	}
	if err := validateBusinessExtensionInvokeInput(input); err != nil {
		return nil, err
	}
	boundedCtx, cancel := newBusinessExtensionInvocationContext(ctx)
	defer cancel()

	var result *capmodel.BusinessExtensionMethodResult
	err := r.service.withPluginLifecycleLock(boundedCtx, input.PluginID, "business-extension-method", func(lockedCtx context.Context) error {
		evidence, err := r.service.recheckProviderInvocation(lockedCtx, r.ownerPluginID, input)
		if err != nil {
			return err
		}
		invocation := &capmodel.BusinessExtensionMethodInvocation{
			Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: r.ownerPluginID,
			ProviderPluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion,
			Method: input.Method, ReleaseVersion: evidence.ReleaseVersion, ReleaseDigest: evidence.ReleaseDigest,
			DeclarationDigest: evidence.DeclarationDigest, Purpose: input.Purpose,
			RuntimeContext: cloneBusinessExtensionRuntimeContext(input.RuntimeContext),
			Payload:        append(json.RawMessage(nil), input.Payload...),
		}
		result, err = r.service.dispatchProviderInvocation(lockedCtx, invocation)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func newBusinessExtensionInvocationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, businessExtensionInvocationTimeout)
}

func validateBusinessExtensionInvokeInput(input pluginhost.BusinessExtensionMethodInvokeInput) error {
	if strings.TrimSpace(input.PluginID) == "" || strings.TrimSpace(input.Point) == "" ||
		strings.TrimSpace(input.ContractVersion) == "" || strings.TrimSpace(input.Method) == "" ||
		strings.TrimSpace(input.ReleaseVersion) == "" || strings.TrimSpace(input.ReleaseDigest) == "" ||
		strings.TrimSpace(input.DeclarationDigest) == "" {
		return gerror.New("complete business extension provider evidence and method are required")
	}
	if plugintypes.ValidatePluginID(strings.TrimSpace(input.PluginID)) != nil ||
		len(input.Point) > 128 || len(input.ContractVersion) > 64 || len(input.Method) > 128 ||
		len(input.ReleaseVersion) > 64 || len(input.ReleaseDigest) != 64 || len(input.DeclarationDigest) != 64 {
		return gerror.New("business extension provider evidence or method shape is invalid")
	}
	if input.Purpose != capmodel.BusinessExtensionInvocationPurposeNewBusiness &&
		input.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness {
		return gerror.New("business extension invocation purpose is invalid")
	}
	if err := validateBusinessExtensionRuntimeContext(input.RuntimeContext); err != nil {
		return err
	}
	return validateBusinessExtensionJSON(input.Payload, businessExtensionRequestPayloadMaxBytes)
}

func validateBusinessExtensionRuntimeContext(runtimeContext *capmodel.BusinessExtensionRuntimeContext) error {
	if runtimeContext == nil {
		return nil
	}
	contextID := runtimeContext.ContextID
	if len(contextID) < businessExtensionContextIDMinBytes || len(contextID) > businessExtensionContextIDMaxBytes ||
		contextID != strings.TrimSpace(contextID) || !businessExtensionContextIDPattern.MatchString(contextID) {
		return gerror.New("business extension runtime context ID is invalid")
	}
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(contextID))
	for _, forbidden := range []string{"instance", "revision", "config", "secret", "token"} {
		if strings.Contains(normalized, forbidden) {
			return gerror.New("business extension runtime context ID contains forbidden material")
		}
	}
	return nil
}

func cloneBusinessExtensionRuntimeContext(runtimeContext *capmodel.BusinessExtensionRuntimeContext) *capmodel.BusinessExtensionRuntimeContext {
	if runtimeContext == nil {
		return nil
	}
	return &capmodel.BusinessExtensionRuntimeContext{ContextID: runtimeContext.ContextID}
}

func (s *serviceImpl) recheckProviderInvocation(
	ctx context.Context,
	ownerPluginID string,
	input pluginhost.BusinessExtensionMethodInvokeInput,
) (*capmodel.BusinessExtensionProviderEvidence, error) {
	registry, err := s.storeSvc.GetRegistry(ctx, input.PluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "read business extension provider state failed")
	}
	if registry == nil || registry.Installed != statusflag.Installed.Int() || registry.Status != statusflag.EnabledValue.Int() ||
		registry.CurrentState != plugintypes.HostStateEnabled.String() {
		return nil, gerror.New("business extension provider is not enabled")
	}
	if input.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness &&
		registry.NewBusinessState != plugintypes.NewBusinessStateAccepting.String() {
		return nil, gerror.New("business extension provider is not accepting new business")
	}
	release, err := s.storeSvc.GetActiveRelease(ctx, input.PluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "read active business extension provider release failed")
	}
	if release == nil || release.ReleaseVersion != strings.TrimSpace(input.ReleaseVersion) ||
		release.Checksum != strings.TrimSpace(input.ReleaseDigest) {
		return nil, gerror.New("business extension provider release evidence is no longer active")
	}
	evidence, err := businessdiscovery.GetExactProvider(ctx, ownerPluginID, plugincap.BusinessExtensionProviderExactInput{
		PluginID: input.PluginID, ReleaseVersion: input.ReleaseVersion, Point: input.Point,
		ContractVersion: input.ContractVersion, ReleaseDigest: input.ReleaseDigest,
		DeclarationDigest: input.DeclarationDigest,
	})
	if err != nil {
		return nil, err
	}
	if evidence == nil || !businessExtensionMethodAuthorized(evidence.AuthorizedMethods, input.Method) {
		return nil, gerror.New("business extension provider method is not authorized by exact evidence")
	}
	return evidence, nil
}

func (s *serviceImpl) dispatchProviderInvocation(
	ctx context.Context,
	invocation *capmodel.BusinessExtensionMethodInvocation,
) (*capmodel.BusinessExtensionMethodResult, error) {
	registry, err := s.storeSvc.GetRegistry(ctx, invocation.ProviderPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "reload business extension provider failed")
	}
	if registry == nil {
		return nil, gerror.New("business extension provider registry is missing")
	}
	switch plugintypes.NormalizeType(registry.Type) {
	case pluginv1.PluginTypeSource:
		return s.dispatchSourceProviderInvocation(ctx, invocation)
	case pluginv1.PluginTypeDynamic:
		return s.dispatchDynamicProviderInvocation(ctx, registry, invocation)
	default:
		return nil, gerror.New("business extension provider runtime type is unsupported")
	}
}

func (s *serviceImpl) dispatchSourceProviderInvocation(
	ctx context.Context,
	invocation *capmodel.BusinessExtensionMethodInvocation,
) (*capmodel.BusinessExtensionMethodResult, error) {
	manifest, err := s.catalogSvc.GetDesiredManifest(invocation.ProviderPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "load source business extension manifest failed")
	}
	if manifest == nil || plugintypes.NormalizeType(manifest.Type) != pluginv1.PluginTypeSource ||
		manifest.Version != invocation.ReleaseVersion || s.catalogSvc.BuildRegistryChecksum(manifest) != invocation.ReleaseDigest {
		return nil, gerror.New("source business extension code does not match the active release evidence")
	}
	definition, ok := pluginhost.GetSourcePlugin(invocation.ProviderPluginID)
	if !ok || definition == nil {
		return nil, gerror.New("source business extension provider is not registered")
	}
	handler := definition.GetBusinessExtensionMethodHandler()
	if handler == nil {
		return nil, gerror.New("source business extension provider method dispatcher is not registered")
	}
	result, invokeErr := handler(ctx, invocation)
	if invokeErr != nil {
		return nil, gerror.New("source business extension provider invocation failed")
	}
	return validateBusinessExtensionResult(result)
}

func (s *serviceImpl) dispatchDynamicProviderInvocation(
	ctx context.Context,
	registry *store.PluginRecord,
	invocation *capmodel.BusinessExtensionMethodInvocation,
) (*capmodel.BusinessExtensionMethodResult, error) {
	manifest, err := s.runtimeSvc.LoadActiveDynamicPluginManifest(ctx, registry)
	if err != nil {
		return nil, gerror.Wrap(err, "load active dynamic business extension release failed")
	}
	if manifest == nil || manifest.ID != invocation.ProviderPluginID || manifest.Version != invocation.ReleaseVersion ||
		manifest.RuntimeArtifact == nil || manifest.RuntimeArtifact.Checksum != invocation.ReleaseDigest {
		return nil, gerror.New("dynamic business extension artifact does not match exact release evidence")
	}
	requestContent, err := json.Marshal(businessExtensionBridgeRequestFromInvocation(invocation))
	if err != nil {
		return nil, gerror.Wrap(err, "encode dynamic business extension request failed")
	}
	if len(requestContent) > businessExtensionRequestMaxBytes {
		return nil, gerror.New("dynamic business extension request envelope is too large")
	}
	response, err := s.runtimeSvc.ExecuteBusinessExtensionMethod(ctx, manifest, requestContent)
	if err != nil {
		return nil, gerror.New("dynamic business extension provider invocation failed")
	}
	if !bridgeResponseSuccessful(response) {
		return nil, gerror.New("dynamic business extension provider rejected the governed invocation")
	}
	if len(response.Body) == 0 || len(response.Body) > businessExtensionResponseMaxBytes ||
		!strings.HasPrefix(strings.ToLower(strings.TrimSpace(response.ContentType)), "application/json") {
		return nil, gerror.New("dynamic business extension provider returned an invalid response envelope")
	}
	bridgeResult := &capmodel.BusinessExtensionBridgeResponse{}
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(bridgeResult); err != nil {
		return nil, gerror.Wrap(err, "decode dynamic business extension response failed")
	}
	if err = ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	return validateBusinessExtensionResult(&capmodel.BusinessExtensionMethodResult{
		Version: bridgeResult.Version,
		Payload: bridgeResult.Payload,
	})
}

func businessExtensionBridgeRequestFromInvocation(invocation *capmodel.BusinessExtensionMethodInvocation) *capmodel.BusinessExtensionBridgeRequest {
	if invocation == nil {
		return nil
	}
	return &capmodel.BusinessExtensionBridgeRequest{
		Version: invocation.Version, OwnerPluginID: invocation.OwnerPluginID,
		ProviderPluginID: invocation.ProviderPluginID, Point: invocation.Point,
		ContractVersion: invocation.ContractVersion, Method: invocation.Method,
		ReleaseVersion: invocation.ReleaseVersion, ReleaseDigest: invocation.ReleaseDigest,
		DeclarationDigest: invocation.DeclarationDigest, Purpose: invocation.Purpose,
		RuntimeContext: cloneBusinessExtensionRuntimeContext(invocation.RuntimeContext),
		Payload:        append(json.RawMessage(nil), invocation.Payload...),
	}
}

func businessExtensionMethodAuthorized(methods []string, method string) bool {
	method = strings.TrimSpace(method)
	for _, candidate := range methods {
		if candidate == method {
			return true
		}
	}
	return false
}

func validateBusinessExtensionResult(result *capmodel.BusinessExtensionMethodResult) (*capmodel.BusinessExtensionMethodResult, error) {
	if result == nil || result.Version != capmodel.BusinessExtensionInvocationProtocolV1 {
		return nil, gerror.New("business extension provider response version is invalid")
	}
	if err := validateBusinessExtensionJSON(result.Payload, businessExtensionResponsePayloadMaxBytes); err != nil {
		return nil, err
	}
	return &capmodel.BusinessExtensionMethodResult{
		Version: result.Version,
		Payload: append(json.RawMessage(nil), result.Payload...),
	}, nil
}

func validateBusinessExtensionJSON(content []byte, maxBytes int) error {
	if len(content) == 0 || len(content) > maxBytes {
		return gerror.New("business extension JSON payload size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return gerror.Wrap(err, "decode business extension JSON payload failed")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return gerror.New("business extension JSON payload must be one object")
	}
	nodes := 0
	return validateBusinessExtensionJSONValue(object, 1, &nodes)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return gerror.New("business extension JSON payload contains multiple values")
		}
		return gerror.Wrap(err, "decode trailing business extension JSON failed")
	}
	return nil
}

func validateBusinessExtensionJSONValue(value any, depth int, nodes *int) error {
	(*nodes)++
	if depth > businessExtensionJSONMaxDepth || *nodes > businessExtensionJSONMaxTotalNodes {
		return gerror.New("business extension JSON payload is too complex")
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) > businessExtensionJSONMaxObjectFields {
			return gerror.New("business extension JSON object has too many fields")
		}
		for key, child := range typed {
			if key == "" || len(key) > 128 || isSensitiveBusinessExtensionKey(key) {
				return gerror.New("business extension JSON contains a forbidden field")
			}
			if err := validateBusinessExtensionJSONValue(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case []any:
		if len(typed) > businessExtensionJSONMaxArrayItems {
			return gerror.New("business extension JSON array has too many items")
		}
		for _, child := range typed {
			if err := validateBusinessExtensionJSONValue(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > businessExtensionJSONMaxStringBytes {
			return gerror.New("business extension JSON string is too large")
		}
	case nil, bool, json.Number:
	default:
		return gerror.New("business extension JSON contains an unsupported value")
	}
	return nil
}

func isSensitiveBusinessExtensionKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	_, forbidden := businessExtensionSensitiveKeys[normalized]
	return forbidden
}

func bridgeResponseSuccessful(response *bridgecontract.BridgeResponseEnvelopeV1) bool {
	return response != nil && response.Failure == nil && response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices
}
