// This file forwards execution using the real package identity resolved from a
// saved declaration. Host admission and confidential runtime checks stay intact.
package providerexecution

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
)

// compatibleProvider resolves the current enabled implementation by its stable
// plugin and contract identity; historical package records do not gate execution.
func compatibleProvider(input plugincap.BusinessExtensionProviderExactInput, items []*capmodel.BusinessExtensionProviderEvidence, _ *capmodel.BusinessExtensionProviderEvidence) (*capmodel.BusinessExtensionProviderEvidence, error) {
	if input.PluginID == "" || input.Point == "" || input.ContractVersion == "" {
		return nil, gerror.New("provider identity is incomplete")
	}
	var selected *capmodel.BusinessExtensionProviderEvidence
	for _, item := range items {
		if item == nil || item.PluginID != input.PluginID || item.Point != input.Point || item.ContractVersion != input.ContractVersion {
			continue
		}
		if selected != nil || item.ReleaseVersion == "" || len(item.ReleaseDigest) != 64 || len(item.AuthorizedMethods) == 0 {
			return nil, gerror.New("current provider is ambiguous or incomplete")
		}
		selected = item
	}
	if selected == nil {
		return nil, gerror.New("current provider is unavailable")
	}
	return selected, nil
}

// WithNewBusinessAdmission retains the host lifecycle guard around the actual package.
func (r *registrar) WithNewBusinessAdmission(ctx context.Context, input pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	if r == nil || r.BusinessExtensionRegistrar == nil || action == nil || ctx == nil {
		return gerror.New("provider admission action is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// The adapter resolves current packages by stable identity. The host already
	// reloads that one current declaration while holding the lifecycle lock;
	// historical hashes must not trigger another all-provider discovery first.
	input.ReleaseVersion, input.ReleaseDigest, input.DeclarationDigest = "", "", ""
	var entered atomic.Bool
	return waitForAdmission(ctx, func() (bool, error) {
		err := r.BusinessExtensionRegistrar.WithNewBusinessAdmission(ctx, input, func(actionCtx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
			entered.Store(true)
			selected, err := compatibleProvider(plugincap.BusinessExtensionProviderExactInput{PluginID: input.PluginID, Point: input.Point, ContractVersion: input.ContractVersion}, []*capmodel.BusinessExtensionProviderEvidence{evidence}, nil)
			if err != nil {
				return err
			}
			if input.Method != "" && !slices.Contains(selected.AuthorizedMethods, input.Method) {
				return gerror.New("provider admission method is unavailable")
			}
			scopedCtx, release := r.withAdmissionEvidence(actionCtx, selected)
			defer release()
			return action(scopedCtx, evidence)
		})
		return entered.Load(), err
	})
}

// InvokeProviderMethod forwards the declared method with genuine executable evidence.
func (r *registrar) InvokeProviderMethod(ctx context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	exact, err := r.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{PluginID: input.PluginID, Point: input.Point, ContractVersion: input.ContractVersion, ReleaseVersion: input.ReleaseVersion, ReleaseDigest: input.ReleaseDigest, DeclarationDigest: input.DeclarationDigest})
	if err != nil {
		return nil, err
	}
	if !slices.Contains(exact.AuthorizedMethods, input.Method) {
		return nil, gerror.New("provider method is unavailable")
	}
	input.ReleaseVersion, input.ReleaseDigest, input.DeclarationDigest = exact.ReleaseVersion, exact.ReleaseDigest, exact.DeclarationDigest
	var result *capmodel.BusinessExtensionMethodResult
	observation, observed := ctx.Value(callObservationKey{}).(*callObservation)
	err = waitForAdmission(ctx, func() (bool, error) {
		var invokeErr error
		result, invokeErr = r.BusinessExtensionRegistrar.InvokeProviderMethod(ctx, input)
		// A caller-supplied confidential context without this owner's callback
		// observation cannot establish that dispatch never entered a callback.
		entered := result != nil || (!observed && input.RuntimeContext != nil)
		if observed {
			entered = entered || observation.callbackEntered.Load()
		}
		return entered, invokeErr
	})
	if err == nil && result != nil {
		observeOperationResult(ctx, input.Method, result.Payload)
	}
	return result, err
}

// WithConfidentialOutboundContext binds confidential transport to the actual executable package.
func (r *registrar) WithConfidentialOutboundContext(ctx context.Context, input pluginhost.ConfidentialOutboundContextInput, action pluginhost.ConfidentialOutboundAction) (invokeErr error) {
	started := time.Now().UTC()
	observation := &callObservation{}
	ctx = context.WithValue(ctx, callObservationKey{}, observation)
	defer func() { r.recordCall(ctx, input, started, invokeErr, observation.resultCode()) }()
	exact, err := r.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{PluginID: input.ProviderPluginID, Point: input.Point, ContractVersion: input.ContractVersion, ReleaseVersion: input.ReleaseVersion, ReleaseDigest: input.ReleaseDigest, DeclarationDigest: input.DeclarationDigest})
	if err != nil {
		return err
	}
	if !slices.Contains(exact.AuthorizedMethods, input.ProviderMethod) {
		return gerror.New("provider outbound method is unavailable")
	}
	input.ReleaseVersion, input.ReleaseDigest, input.DeclarationDigest = exact.ReleaseVersion, exact.ReleaseDigest, exact.DeclarationDigest
	handler := input.Handler
	if handler != nil {
		input.Handler = func(callCtx context.Context, request *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			observation.callbackEntered.Store(true)
			return handler(callCtx, request)
		}
	}
	return r.BusinessExtensionRegistrar.WithConfidentialOutboundContext(ctx, input, action)
}
