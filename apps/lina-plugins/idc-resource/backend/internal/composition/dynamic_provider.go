// This file invokes frozen resource connectors through confidential owner-side
// callbacks; connection proofs require owner-confirmed authentication and clocks.
package composition

import (
	"context"
	"encoding/json"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"net/http"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

// DynamicOutboundFactory creates the owner-side confidential supplier handler
// for one exact channel and method. Implementations retain endpoints, secret
// handles, authentication sessions, raw tokens and raw supplier evidence.
type DynamicOutboundFactory func(
	ctx context.Context,
	configuration channelconfig.Configuration,
	method string,
	operationSecrets []resourcecap.SecretBinding,
) (pluginhost.ConfidentialOutboundHandler, func(), error)

type dynamicProvider struct {
	extensions    pluginhost.BusinessExtensionRegistrar
	configuration channelconfig.Configuration
	outbound      DynamicOutboundFactory
}

func (p *dynamicProvider) VerifyConnection(ctx context.Context) (*resourcecap.ConnectionVerification, error) {
	var result resourcecap.ConnectionVerification
	if err := p.invoke(ctx, resourcecap.DynamicProviderMethodVerifyConnection, struct{}{}, nil, &result); err != nil {
		return nil, err
	}
	return &result, result.ValidateAt(time.Now().UTC())
}

func (p *dynamicProvider) DiscoverCapabilities(ctx context.Context) (*resourcecap.CapabilitySnapshot, error) {
	var result resourcecap.CapabilitySnapshot
	if err := p.invoke(ctx, resourcecap.DynamicProviderMethodDiscoverCapabilities, struct{}{}, nil, &result); err != nil {
		return nil, err
	}
	return &result, result.ValidateAt(time.Now().UTC())
}

func (p *dynamicProvider) ListProviderCapacity(ctx context.Context, request resourcecap.ProviderCapacityRequest) (*resourcecap.ProviderCapacityPage, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	var result resourcecap.ProviderCapacityPage
	if err := p.invoke(ctx, resourcecap.DynamicProviderMethodListProviderCapacity, request, nil, &result); err != nil {
		return nil, err
	}
	return &result, result.Validate(request)
}

func (p *dynamicProvider) Execute(ctx context.Context, request resourcecap.OperationRequest) (*resourcecap.OperationResult, error) {
	projected, err := resourcecap.ProjectDynamicOperationRequest(request)
	if err != nil {
		return nil, err
	}
	if err = projected.Validate(); err != nil {
		return nil, err
	}
	var result resourcecap.OperationResult
	if err = p.invoke(ctx, resourcecap.DynamicProviderMethodExecute, projected, request.SecretBindings, &result); err != nil {
		return nil, err
	}
	return &result, result.Validate()
}

func (p *dynamicProvider) QueryTask(ctx context.Context, request resourcecap.TaskQuery) (*resourcecap.OperationResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	var result resourcecap.OperationResult
	if err := p.invoke(ctx, resourcecap.DynamicProviderMethodQueryTask, request, nil, &result); err != nil {
		return nil, err
	}
	return &result, result.Validate()
}

func (p *dynamicProvider) QueryResource(ctx context.Context, request resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	var result resourcecap.ResourceSnapshot
	if err := p.invoke(ctx, resourcecap.DynamicProviderMethodQueryResource, request, nil, &result); err != nil {
		return nil, err
	}
	return &result, result.Validate()
}

func (p *dynamicProvider) invoke(ctx context.Context, method string, request any, operationSecrets []resourcecap.SecretBinding, target any) error {
	if p == nil || p.extensions == nil || p.outbound == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("dynamic resource Provider is unavailable")
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return gerror.New("dynamic resource Provider request is invalid")
	}
	settings, err := resourcecap.ProjectPublicInterfaceSettings(p.configuration.Config.Settings, true)
	if err != nil {
		return gerror.New("dynamic resource Provider interface settings are invalid")
	}
	payload, err = resourcecap.NewDynamicProviderInvocationEnvelope(settings, payload)
	if err != nil {
		return gerror.New("dynamic resource Provider request is invalid")
	}
	handler, cleanup, err := p.outbound(ctx, p.configuration, method, operationSecrets)
	if err != nil || handler == nil || cleanup == nil {
		if safe := resourcecap.SafeProviderConnectionError(err); safe != nil {
			return safe
		}
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	defer cleanup()

	var (
		response        []byte
		failureMu       sync.Mutex
		ownerFailure    error
		authenticatedAt time.Time
	)
	// Capture only owner-reviewed codes before the WASM host sanitizes errors.
	capture := func(callCtx context.Context, request *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
		result, callErr := handler(callCtx, request)
		if method == resourcecap.DynamicProviderMethodVerifyConnection {
			logger.Debugf(callCtx, "Provider owner connection check completed success=%t authenticated=%t", callErr == nil, confirmedAuthenticationStep(request, result))
			if metadata, ok := bizerr.As(callErr); ok {
				logger.Debugf(callCtx, "Provider owner connection check rejected code=%s", metadata.RuntimeCode())
			}
		}
		// The guest clock is isolated and cannot establish wall-clock freshness.
		// Only an owner-confirmed successful authentication in this invocation
		// can supply the observation time for a connection proof.
		if method == resourcecap.DynamicProviderMethodVerifyConnection && callErr == nil && confirmedAuthenticationStep(request, result) {
			failureMu.Lock()
			authenticatedAt = time.Now().UTC()
			failureMu.Unlock()
		}
		if safe := resourcecap.SafeProviderConnectionError(callErr); safe != nil {
			failureMu.Lock()
			ownerFailure = safe
			failureMu.Unlock()
		}
		return result, callErr
	}
	err = p.extensions.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{
		ProviderPluginID: p.configuration.PluginID,
		Point:            resourcecap.DynamicProviderPoint, ContractVersion: p.configuration.ContractVersion,
		ProviderMethod: method, OutboundMethod: "resource-provider." + method,
		ReleaseVersion: p.configuration.ReleaseVersion, ReleaseDigest: p.configuration.ReleaseDigest,
		DeclarationDigest:   p.configuration.DeclarationDigest,
		InterfaceInstanceID: p.configuration.Config.ChannelRef,
		Purpose:             capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		ExpiresAfter:        5 * time.Second, AllowedResultFields: allowedDynamicResultFields(method),
		Handler: capture,
	}, func(callCtx context.Context, runtimeContext *capmodel.BusinessExtensionRuntimeContext) error {
		result, invokeErr := p.extensions.InvokeProviderMethod(callCtx, pluginhost.BusinessExtensionMethodInvokeInput{
			PluginID: p.configuration.PluginID, Point: resourcecap.DynamicProviderPoint,
			ContractVersion: p.configuration.ContractVersion, Method: method,
			ReleaseVersion: p.configuration.ReleaseVersion, ReleaseDigest: p.configuration.ReleaseDigest,
			DeclarationDigest: p.configuration.DeclarationDigest,
			Purpose:           capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
			RuntimeContext:    runtimeContext, Payload: payload,
		})
		if invokeErr != nil || result == nil {
			if metadata, ok := bizerr.As(invokeErr); ok {
				logger.Warningf(callCtx, "Provider invocation rejected code=%s", metadata.RuntimeCode())
			}
			return gerror.New("dynamic resource Provider invocation failed")
		}
		response = append(response[:0], result.Payload...)
		return nil
	})
	if err != nil || len(response) == 0 {
		failureMu.Lock()
		safe := ownerFailure
		failureMu.Unlock()
		if safe != nil {
			return safe
		}
		return gerror.New("dynamic resource Provider invocation failed")
	}
	if err = json.Unmarshal(response, target); err != nil {
		return bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
	}
	switch typed := target.(type) {
	case *resourcecap.ConnectionVerification:
		failureMu.Lock()
		safe, observedAt := ownerFailure, authenticatedAt
		failureMu.Unlock()
		if !typed.Verified || observedAt.IsZero() {
			if safe != nil {
				return safe
			}
			return bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
		}
		typed.ObservedAt = observedAt
	case *resourcecap.CapabilitySnapshot:
		// A capability declaration is observed on receipt; this does not prove
		// that an upstream connection was attempted or authenticated.
		typed.ObservedAt = time.Now().UTC()
	}
	return nil
}

func allowedDynamicResultFields(method string) []string {
	proxyFields := []string{"StatusCode", "Data", "Authenticated"}
	switch method {
	case resourcecap.DynamicProviderMethodVerifyConnection:
		return append(proxyFields, "Verified", "ObservedAt")
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		return append(proxyFields, "AdapterVersion", "Capabilities", "ObservedAt")
	case resourcecap.DynamicProviderMethodListProviderCapacity:
		return append(proxyFields, "Items", "NextCursor", "Complete", "Evidence")
	case resourcecap.DynamicProviderMethodExecute, resourcecap.DynamicProviderMethodQueryTask:
		return append(proxyFields, "Status", "ExternalTaskRef", "ExternalResourceRef", "Resource", "Failure", "Progress", "Evidence", "ResidualResources")
	case resourcecap.DynamicProviderMethodQueryResource:
		return append(proxyFields, "ExternalResourceRef", "State", "ObservedAt", "Evidence")
	default:
		return nil
	}
}

// confirmedAuthenticationStep inspects only the owner-produced HTTP outcome;
// a guest business request or successful transport alone is never authentication.
func confirmedAuthenticationStep(request *pluginhost.ConfidentialOutboundRequest, result *pluginhost.ConfidentialOutboundResult) bool {
	if request == nil || result == nil {
		return false
	}
	var (
		step     resourcecap.ConfidentialHTTPRequest
		observed resourcecap.ConfidentialHTTPResponse
	)
	if json.Unmarshal(request.Payload, &step) != nil || step.Kind != resourcecap.ConfidentialHTTPKindAuthenticate || json.Unmarshal(result.Payload, &observed) != nil {
		return false
	}
	return observed.Authenticated && observed.StatusCode >= http.StatusOK && observed.StatusCode < http.StatusMultipleChoices
}
