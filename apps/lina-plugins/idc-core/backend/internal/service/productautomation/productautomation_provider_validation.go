// This file invokes optional pure provider validation on canonical merchant
// configuration. The temporary context grants no configuration or outbound view.

package productautomation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) validateProviderProductConfiguration(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, values json.RawMessage) error {
	if !slices.Contains(exact.AuthorizedMethods, productfulfillmentcap.MethodValidateProductConfiguration) {
		return nil
	}
	payload, err := json.Marshal(productfulfillmentcap.ConfigurationValidationRequest{Version: productfulfillmentcap.ContractVersion, ProductConfiguration: values})
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	// Pure validation must not carry an interface identity: secret access uses
	// the identity independently from the public configuration view.
	runtimeContext, err := s.runtimeContexts.issue(exact, dynamicCandidateScope{}, capmodel.BusinessExtensionInvocationPurposeNewBusiness, productfulfillmentcap.MethodValidateProductConfiguration, nil)
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	defer s.runtimeContexts.remove(runtimeContext.id)
	result, err := s.extensions.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{
		PluginID: exact.PluginID, Point: exact.Point, ContractVersion: exact.ContractVersion,
		Method: productfulfillmentcap.MethodValidateProductConfiguration, ReleaseVersion: exact.ReleaseVersion,
		ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest,
		Purpose:        capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: runtimeContext.id}, Payload: payload,
	})
	if err != nil || result == nil || result.Version != productfulfillmentcap.ContractVersion || len(result.Payload) > productfulfillmentcap.MaximumEvidenceBytes {
		return bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	var validation productfulfillmentcap.ConfigurationValidationResult
	decoder := json.NewDecoder(bytes.NewReader(result.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&validation) != nil || decoder.Decode(&struct{}{}) != io.EOF || validation.Version != productfulfillmentcap.ContractVersion {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	if !validation.Valid && validation.Reason == productfulfillmentcap.ConfigurationValidationSupplierCapabilitiesUnconfirmed {
		return bizerr.NewCode(codeSupplierCapabilitiesUnconfirmed)
	}
	if !validation.Valid || validation.Reason != "" {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return nil
}
