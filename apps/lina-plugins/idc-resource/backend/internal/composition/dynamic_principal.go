// This file binds optional dynamic principal methods to owner-only secret slots.
package composition

import (
	"context"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"strings"
)

var _ resourcecap.PrincipalProvider = (*dynamicProvider)(nil)

func (p *dynamicProvider) LookupPrincipal(ctx context.Context, request resourcecap.PrincipalLookupRequest) (*resourcecap.PrincipalLookupResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	input := resourcecap.DynamicPrincipalRequest{RequestID: principalInvocationReference(request.RequestID), ConfidentialInputNames: []string{resourcecap.PrincipalBindingUsername}}
	var result resourcecap.PrincipalLookupResult
	if err := p.invoke(ctx, resourcecap.DynamicProviderMethodLookupPrincipal, input, []resourcecap.SecretBinding{{Name: resourcecap.PrincipalBindingUsername, Handle: request.Username}}, &result); err != nil {
		return nil, err
	}
	return &result, result.Validate()
}

func (p *dynamicProvider) CreatePrincipal(ctx context.Context, request resourcecap.PrincipalCreateRequest) (*resourcecap.PrincipalCreateResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	input := resourcecap.DynamicPrincipalRequest{RequestID: principalInvocationReference(request.RequestID), ConfidentialInputNames: make([]string, 0, len(request.SecretBindings))}
	for _, binding := range request.SecretBindings {
		input.ConfidentialInputNames = append(input.ConfidentialInputNames, binding.Name)
	}
	if err := input.Validate(true); err != nil {
		return nil, err
	}
	var result resourcecap.PrincipalCreateResult
	if err := p.invoke(ctx, resourcecap.DynamicProviderMethodCreatePrincipal, input, request.SecretBindings, &result); err != nil {
		return nil, err
	}
	return &result, result.Validate()
}

// Existing frozen guests already recognize the task_ wire format. Keep the
// principal owner's durable prt_ identity intact and project only this opaque
// correlation so those guests need no executable or channel-history rewrite.
func principalInvocationReference(reference string) string {
	if len(reference) != 36 || !strings.HasPrefix(reference, "prt_") {
		return reference
	}
	for _, character := range reference[4:] {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return reference
		}
	}
	return "task_" + reference[4:]
}
