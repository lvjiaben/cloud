// This file projects published server login settings and exact resource option
// roles without exposing provider keys, credentials, or editable runtime state.
package catalog

import (
	"context"
	"encoding/json"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseserverlogin"
)

// PublicServerLoginRules describes generation defaults; manual input follows
// supplier character and length limits rather than a generated-name prefix.
type PublicServerLoginRules struct {
	Prefix     string   `json:"prefix,omitempty"`
	Length     int      `json:"length,omitempty"`
	Characters []string `json:"characters,omitempty"`
}

// PublicServerLoginPolicy enables customer inputs independently and preserves
// automatic generation whenever either permitted input is left empty.
type PublicServerLoginPolicy struct {
	HostnameEnabled bool                   `json:"hostnameEnabled"`
	PasswordEnabled bool                   `json:"passwordEnabled"`
	Hostname        PublicServerLoginRules `json:"hostname"`
	Password        PublicServerLoginRules `json:"password"`
}

const resourceCandidateMethod = "configuration.candidates"

func publicPurchasePresentation(field *capmodel.ConfigurationFieldDeclaration) string {
	if field == nil || field.DynamicCandidates == nil || field.DynamicCandidates.Version != "v1" || field.DynamicCandidates.Method != resourceCandidateMethod {
		return ""
	}
	switch field.Key {
	case "operating_system", "region":
		return field.Key
	}
	return ""
}

// publicPurchasePresentationRoles traces merchant-local product mappings back
// to their frozen candidate source. Materialized OS choices retain their native
// field identity under the exact cloud provider's owned declaration contract.
func publicPurchasePresentationRoles(target publicStructuredConfigurationTarget, customer *capmodel.ConfigurationSchemaDeclaration, presentations map[string]configurationcontract.Presentation) (map[string]string, error) {
	product, err := configurationcontract.DecodeDeclaration(target.ProductDeclaration)
	if err != nil {
		return nil, err
	}
	sources := map[string]*capmodel.ConfigurationFieldDeclaration{}
	if product != nil {
		for _, field := range product.Fields {
			sources[field.Key] = field
		}
	}
	roles := make(map[string]string, len(customer.Fields))
	for _, field := range customer.Fields {
		role := publicPurchasePresentation(field)
		if role == "" {
			sourceKey := presentations[field.Key].ProductFieldKey
			if sourceKey == "" {
				sourceKey = field.Key
			}
			role = publicPurchasePresentation(sources[sourceKey])
		}
		if role == "" && target.BindingPluginID == managedCloudPresentationProvider && field.Key == "operating_system" && field.Type == capmodel.ConfigurationFieldSingleSelect {
			role = "operating_system"
		}
		if role != "" {
			roles[field.Key] = role
		}
	}
	return roles, nil
}

// managedCloudPresentationProvider owns the native operating_system field even
// after the published customer schema freezes its dynamic choices as static.
const managedCloudPresentationProvider = "idc-zjmfcloud"

func publicServerLoginPolicy(raw string) *PublicServerLoginPolicy {
	var values struct {
		HostnameEnabled    bool     `json:"hostname_customer_input"`
		PasswordEnabled    bool     `json:"password_customer_input"`
		HostnamePrefix     string   `json:"hostname_prefix"`
		HostnameLength     int      `json:"hostname_length"`
		HostnameCharacters []string `json:"hostname_characters"`
		PasswordLength     int      `json:"password_length"`
		PasswordCharacters []string `json:"password_characters"`
	}
	if json.Unmarshal([]byte(raw), &values) != nil || (!values.HostnameEnabled && !values.PasswordEnabled) {
		return nil
	}
	return &PublicServerLoginPolicy{HostnameEnabled: values.HostnameEnabled, PasswordEnabled: values.PasswordEnabled,
		Hostname: PublicServerLoginRules{Prefix: values.HostnamePrefix, Length: values.HostnameLength, Characters: values.HostnameCharacters},
		Password: PublicServerLoginRules{Length: values.PasswordLength, Characters: values.PasswordCharacters}}
}

// ResolvePurchaseServerLoginTarget checks the exact current published revision
// before the credential owner stores a customer-provided server login.
func (s *publicCatalogReaderImpl) ResolvePurchaseServerLoginTarget(ctx context.Context, slug, revision string) (*purchaseserverlogin.Target, error) {
	snapshot, err := s.loadStructuredConfigurationForEvaluation(ctx, slug, revision)
	if err != nil {
		return nil, err
	}
	policy := publicServerLoginPolicy(snapshot.Target.MerchantValues)
	if policy == nil {
		return nil, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	var route struct {
		ChannelNo string `orm:"channel_no"`
	}
	err = s.database.GetScan(ctx, &route, `SELECT resource.resource_channel_ref AS channel_no FROM plugin_idc_core_catalog_product_version_fulfillment_binding binding JOIN plugin_idc_core_product_automation_interface_instance resource ON resource.interface_instance_no=binding.interface_instance_no AND resource.tenant_id=binding.tenant_id WHERE binding.tenant_id=0 AND binding.version_no=? AND resource.resource_owner_id='idc-resource'`, snapshot.Target.VersionNo)
	if err != nil || route.ChannelNo == "" {
		return nil, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	return &purchaseserverlogin.Target{ProductVersionNo: snapshot.Target.VersionNo, ConfigurationRevision: snapshot.Revision, ChannelNo: route.ChannelNo,
		HostnameEnabled: policy.HostnameEnabled, PasswordEnabled: policy.PasswordEnabled,
		HostnameRules: purchaseserverlogin.HostnameRules{Prefix: policy.Hostname.Prefix, Length: policy.Hostname.Length, Characters: policy.Hostname.Characters},
		PasswordRules: purchaseserverlogin.PasswordRules{Length: policy.Password.Length, Characters: policy.Password.Characters}}, nil
}
