// Package managedparameters invokes the managed connector's pure parameter mapper.
package managedparameters

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type dependencies struct {
	database   gdb.DB
	extensions pluginhost.BusinessExtensionRegistrar
}

// LazyBuilder is declared before startup and bound to the owner facade later.
type LazyBuilder struct {
	mu   sync.RWMutex
	deps *dependencies
}

func NewLazy() *LazyBuilder { return &LazyBuilder{} }

func (builder *LazyBuilder) Bind(database gdb.DB, extensions pluginhost.BusinessExtensionRegistrar) error {
	if builder == nil || database == nil || extensions == nil {
		return gerror.New("managed parameter builder dependencies are required")
	}
	builder.mu.Lock()
	defer builder.mu.Unlock()
	// Job declarations are collected again after plugin lifecycle changes.
	// Refreshing the same process-local dependencies is safe and keeps that
	// read-only collection path idempotent.
	builder.deps = &dependencies{database: database, extensions: extensions}
	return nil
}

func (builder *LazyBuilder) Build(ctx context.Context, channelNo, revisionNo string, plan resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	builder.mu.RLock()
	deps := builder.deps
	builder.mu.RUnlock()
	if deps == nil {
		return nil, gerror.New("managed parameter builder is unavailable")
	}
	reader, err := channelconfig.NewReader(deps.database)
	if err != nil {
		return nil, err
	}
	configuration, err := reader.ReadRevision(ctx, channelNo, revisionNo)
	if err != nil || configuration.RuntimeType != "dynamic" {
		return nil, gerror.New("managed parameter channel is unavailable")
	}
	payload, err := json.Marshal(plan)
	if err != nil {
		return nil, gerror.New("managed parameter plan is invalid")
	}
	settings, err := resourcecap.ProjectPublicInterfaceSettings(configuration.Config.Settings, true)
	if err != nil {
		return nil, gerror.New("managed parameter interface settings are invalid")
	}
	payload, err = resourcecap.NewDynamicProviderInvocationEnvelope(settings, payload)
	if err != nil {
		return nil, gerror.New("managed parameter plan is invalid")
	}
	deniedOutbound := func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
		return nil, gerror.New("outbound access is forbidden while building provision parameters")
	}
	var response []byte
	err = deps.extensions.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{
		ProviderPluginID: configuration.PluginID, Point: resourcecap.DynamicProviderPoint,
		ContractVersion: configuration.ContractVersion,
		ProviderMethod:  resourcecap.DynamicProviderMethodBuildProvisionParameters,
		OutboundMethod:  "resource-provider.provision-parameters-denied",
		ReleaseVersion:  configuration.ReleaseVersion, ReleaseDigest: configuration.ReleaseDigest,
		DeclarationDigest: configuration.DeclarationDigest, InterfaceInstanceID: channelNo,
		Purpose:      capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		ExpiresAfter: 5 * time.Second, AllowedResultFields: []string{"Denied"}, Handler: deniedOutbound,
	}, func(callCtx context.Context, runtimeContext *capmodel.BusinessExtensionRuntimeContext) error {
		result, invokeErr := deps.extensions.InvokeProviderMethod(callCtx, pluginhost.BusinessExtensionMethodInvokeInput{
			PluginID: configuration.PluginID, Point: resourcecap.DynamicProviderPoint,
			ContractVersion: configuration.ContractVersion,
			Method:          resourcecap.DynamicProviderMethodBuildProvisionParameters,
			ReleaseVersion:  configuration.ReleaseVersion, ReleaseDigest: configuration.ReleaseDigest,
			DeclarationDigest: configuration.DeclarationDigest,
			Purpose:           capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
			RuntimeContext:    runtimeContext, Payload: payload,
		})
		if invokeErr != nil || result == nil {
			return gerror.New("managed parameter mapper failed")
		}
		response = append([]byte(nil), result.Payload...)
		return nil
	})
	if err != nil {
		return nil, gerror.New("managed parameter mapper failed")
	}
	return resourcecap.BuildProvisionParameters(ctx, func(context.Context, resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
		return response, nil
	}, plan)
}
