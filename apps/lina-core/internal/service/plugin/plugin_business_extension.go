package plugin

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/internal/service/plugin/internal/businessdiscovery"
	"lina-core/internal/service/plugin/internal/confidentialoutbound"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
)

type sourceBusinessExtensionRegistrar struct {
	ownerPluginID string
	service       *serviceImpl
}

// ForBusinessExtensionOwner returns a facade bound to the source plugin's
// immutable registration identity.
func (s *serviceImpl) ForBusinessExtensionOwner(pluginID string) pluginhost.BusinessExtensionRegistrar {
	if s == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	return &sourceBusinessExtensionRegistrar{ownerPluginID: strings.TrimSpace(pluginID), service: s}
}

func (r *sourceBusinessExtensionRegistrar) ListProviders(ctx context.Context, input plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	page, err := businessdiscovery.ListNewBindingProviders(ctx, discoveryOwner(r.ownerPluginID, input.Point), input)
	if err != nil {
		return nil, err
	}
	r.localizeProviderEvidence(ctx, page.Items)
	return page, nil
}

func (r *sourceBusinessExtensionRegistrar) GetProviderExact(ctx context.Context, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	evidence, err := businessdiscovery.GetExactProvider(ctx, discoveryOwner(r.ownerPluginID, input.Point), input)
	if err != nil {
		return nil, err
	}
	r.localizeProviderEvidence(ctx, []*capmodel.BusinessExtensionProviderEvidence{evidence})
	return evidence, nil
}

func (r *sourceBusinessExtensionRegistrar) GetProvidersExact(ctx context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	owner := r.ownerPluginID
	if len(inputs) > 0 {
		owner = discoveryOwner(owner, inputs[0].Point)
	}
	evidence, err := businessdiscovery.GetExactProviders(ctx, owner, inputs)
	if err != nil {
		return nil, err
	}
	r.localizeProviderEvidence(ctx, evidence)
	return evidence, nil
}

func (r *sourceBusinessExtensionRegistrar) localizeProviderEvidence(ctx context.Context, items []*capmodel.BusinessExtensionProviderEvidence) {
	if r == nil || r.service == nil || r.service.i18nSvc == nil {
		return
	}
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.PluginID) == "" {
			continue
		}
		key := "plugin." + strings.TrimSpace(item.PluginID) + ".name"
		localized := r.service.i18nSvc.TranslateDynamicPluginSourceText(ctx, item.PluginID, key, item.DisplayName)
		item.DisplayName = r.service.i18nSvc.Translate(ctx, key, localized)
	}
}

// WithConfidentialOutboundContext creates one one-use host-only supplier-call
// binding and guarantees its removal when the owner callback returns.
func (r *sourceBusinessExtensionRegistrar) WithConfidentialOutboundContext(
	ctx context.Context,
	input pluginhost.ConfidentialOutboundContextInput,
	action pluginhost.ConfidentialOutboundAction,
) error {
	if r == nil || r.service == nil || r.service.confidentialOutbound == nil || action == nil {
		return gerror.New("confidential outbound action is unavailable")
	}
	contextID, revoke, err := r.service.confidentialOutbound.Register(confidentialoutbound.Registration{
		OwnerPluginID: r.ownerPluginID,
		Input:         input,
	})
	if err != nil {
		return err
	}
	defer revoke()
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: contextID})
}

// WithNewBusinessAdmission serializes the provider's lifecycle with the
// owner's local transaction callback and rechecks authoritative evidence while locked.
func (r *sourceBusinessExtensionRegistrar) WithNewBusinessAdmission(ctx context.Context, input pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	if r == nil || r.service == nil || action == nil {
		return gerror.New("business extension admission action is required")
	}
	providerID := strings.TrimSpace(input.PluginID)
	if providerID == "" {
		return gerror.New("business extension provider plugin ID is required")
	}
	return r.service.withPluginLifecycleLock(ctx, providerID, "business-extension-new-binding", func(lockedCtx context.Context) error {
		evidence, err := businessdiscovery.GetActiveProviderForAdmission(lockedCtx, discoveryOwner(r.ownerPluginID, input.Point), plugincap.BusinessExtensionProviderExactInput{
			PluginID: providerID, ReleaseVersion: input.ReleaseVersion, Point: input.Point,
			ContractVersion: input.ContractVersion, ReleaseDigest: input.ReleaseDigest,
			DeclarationDigest: input.DeclarationDigest,
		}, input.Method)
		if err != nil {
			return err
		}
		if evidence == nil {
			return gerror.New("business extension provider is not eligible for a new binding")
		}
		return action(lockedCtx, evidence)
	})
}

// IDC resource configuration consumes the product-fulfillment extension
// owned by idc-core. Keep that cross-domain owner explicit instead of making
// every caller bypass the owner check or inspect another plugin's tables.
func discoveryOwner(caller, point string) string {
	if caller == "idc-resource" && point == capmodel.BusinessExtensionProductFulfillment {
		return "idc-core"
	}
	return caller
}

var _ pluginhost.BusinessExtensionRegistrar = (*sourceBusinessExtensionRegistrar)(nil)
