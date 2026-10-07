package composition

import (
	"context"
	"encoding/json"
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
		return gerror.New("dynamic resource Provider outbound handler is unavailable")
	}
	defer cleanup()

	var response []byte
	err = p.extensions.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{
		ProviderPluginID: p.configuration.PluginID,
		Point:            resourcecap.DynamicProviderPoint, ContractVersion: p.configuration.ContractVersion,
		ProviderMethod: method, OutboundMethod: "resource-provider." + method,
		ReleaseVersion: p.configuration.ReleaseVersion, ReleaseDigest: p.configuration.ReleaseDigest,
		DeclarationDigest:   p.configuration.DeclarationDigest,
		InterfaceInstanceID: p.configuration.Config.ChannelRef,
		Purpose:             capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		ExpiresAfter:        5 * time.Second, AllowedResultFields: allowedDynamicResultFields(method),
		Handler: handler,
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
			return gerror.New("dynamic resource Provider invocation failed")
		}
		response = append(response[:0], result.Payload...)
		return nil
	})
	if err != nil || len(response) == 0 {
		return gerror.New("dynamic resource Provider invocation failed")
	}
	if err = json.Unmarshal(response, target); err != nil {
		return gerror.New("dynamic resource Provider result is invalid")
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
