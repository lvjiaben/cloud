// This file reuses the host-locked current declaration only within its actual
// admission callback. It is not a request/global cache: retained contexts lose
// the fact immediately on return, and another registrar or identity cannot use it.
package providerexecution

import (
	"context"
	"slices"
	"sync/atomic"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
)

type admissionEvidenceKey struct{}
type admissionEvidenceScope struct {
	owner    *registrar
	evidence capmodel.BusinessExtensionProviderEvidence
	active   atomic.Bool
}

// withAdmissionEvidence keeps a detached identity/method slice. Declarations
// remain immutable contracts; callback callers cannot replace the held identity
// or extend the admitted method list through a returned evidence value.
func (r *registrar) withAdmissionEvidence(ctx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) (context.Context, func()) {
	scope := &admissionEvidenceScope{owner: r, evidence: *evidence}
	scope.evidence.AuthorizedMethods = slices.Clone(evidence.AuthorizedMethods)
	scope.active.Store(true)
	return context.WithValue(ctx, admissionEvidenceKey{}, scope), func() { scope.active.Store(false) }
}

func (r *registrar) admittedProvider(ctx context.Context, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, bool) {
	if r == nil || ctx == nil || ctx.Err() != nil {
		return nil, false
	}
	scope, ok := ctx.Value(admissionEvidenceKey{}).(*admissionEvidenceScope)
	if !ok || scope == nil || scope.owner != r || !scope.active.Load() {
		return nil, false
	}
	evidence := scope.evidence
	if evidence.PluginID != input.PluginID || evidence.Point != input.Point || evidence.ContractVersion != input.ContractVersion {
		return nil, false
	}
	evidence.AuthorizedMethods = slices.Clone(evidence.AuthorizedMethods)
	return &evidence, true
}

// A partial batch cannot mix locked facts with a fresh discovery. Fall back to
// the ordinary single collection read unless every requested identity is held.
func (r *registrar) admittedProviders(ctx context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, bool) {
	result := make([]*capmodel.BusinessExtensionProviderEvidence, len(inputs))
	for index, input := range inputs {
		evidence, ok := r.admittedProvider(ctx, input)
		if !ok {
			return nil, false
		}
		result[index] = evidence
	}
	return result, true
}
