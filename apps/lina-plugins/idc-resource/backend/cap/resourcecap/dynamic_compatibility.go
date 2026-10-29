// This file proves an optional principal-method addition preserves the old
// resource contract; real current host method authorization remains mandatory.
package resourcecap

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"slices"
)

func IsOptionalPrincipalUpgrade(previous, current *capmodel.BusinessExtensionProviderEvidence) bool {
	if previous == nil || current == nil || previous.PluginID != current.PluginID || previous.Point != DynamicProviderPoint || current.Point != DynamicProviderPoint || previous.ContractVersion != DynamicProviderContractVersion || current.ContractVersion != previous.ContractVersion {
		return false
	}
	if !validDynamicEvidenceContract(previous) || !validDynamicEvidenceContract(current) {
		return false
	}
	equal := func(left, right any) bool {
		a, e := json.Marshal(left)
		if e != nil {
			return false
		}
		b, e := json.Marshal(right)
		return e == nil && string(a) == string(b)
	}
	if previous.InterfaceConfiguration == nil || !equal(previous.InterfaceConfiguration, current.InterfaceConfiguration) || previous.InterfaceConfigurationCardinality != current.InterfaceConfigurationCardinality || previous.ProductConfiguration != nil || current.ProductConfiguration != nil || previous.CustomerConfiguration != nil || current.CustomerConfiguration != nil || !equal(previous.ProviderMetadata, current.ProviderMetadata) {
		return false
	}
	if slices.Contains(previous.AuthorizedMethods, DynamicProviderMethodLookupPrincipal) || slices.Contains(previous.AuthorizedMethods, DynamicProviderMethodCreatePrincipal) {
		return false
	}
	want := append(append([]string{}, previous.AuthorizedMethods...), DynamicProviderMethodLookupPrincipal, DynamicProviderMethodCreatePrincipal)
	slices.Sort(want)
	got := append([]string{}, current.AuthorizedMethods...)
	slices.Sort(got)
	return slices.Equal(want, got)
}

func validDynamicEvidenceContract(evidence *capmodel.BusinessExtensionProviderEvidence) bool {
	declaration := &capmodel.BusinessExtensionDeclaration{Point: evidence.Point, Version: evidence.ContractVersion, Role: capmodel.BusinessExtensionRoleProvider, Owner: OwnerPluginID, Methods: evidence.AuthorizedMethods, InterfaceConfiguration: evidence.InterfaceConfiguration, InterfaceConfigurationCardinality: evidence.InterfaceConfigurationCardinality, ProductConfiguration: evidence.ProductConfiguration, CustomerConfiguration: evidence.CustomerConfiguration, ProviderMetadata: evidence.ProviderMetadata}
	return ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}) == nil
}
