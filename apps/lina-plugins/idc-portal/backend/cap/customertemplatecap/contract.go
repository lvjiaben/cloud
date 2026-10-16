// Package customertemplatecap defines the declarative, code-free customer template contract.
package customertemplatecap

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-portal/backend/cap/templatecap"
)

// Stable website extension identifiers are shared by owners and providers.
const (
	Point           = "customer-template"
	Version         = "v1"
	Owner           = "idc-portal"
	MetadataType    = "customer-template-contract"
	Renderer        = "customer-standard-v1"
	MinimalRenderer = "customer-compact-v1"
	MethodDescribe  = "capabilities.describe"
)

// ValidateDeclarations accepts bounded public configuration, never secrets or executable code.
func ValidateDeclarations(items []*capmodel.BusinessExtensionDeclaration) error {
	if len(items) != 1 || items[0] == nil {
		return gerror.New("customer template declaration is incomplete")
	}
	d := items[0]
	if err := validateIdentity(d); err != nil {
		return err
	}
	if err := validateRenderer(d.ProviderMetadata); err != nil {
		return err
	}
	if len(d.InterfaceConfiguration.Fields) > 30 {
		return gerror.New("customer template has too many settings")
	}
	for _, f := range d.InterfaceConfiguration.Fields {
		if f == nil {
			return gerror.New("customer template field is invalid")
		}
		switch f.Type {
		case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldInteger:
		default:
			return gerror.New("customer template field type is unsupported")
		}
	}
	_, err := Pages(d.ProviderMetadata)
	return err
}

// validateIdentity requires one provider-only singleton contract.
func validateIdentity(d *capmodel.BusinessExtensionDeclaration) error {
	if d.Point != Point || d.Version != Version || d.Owner != Owner || d.Role != capmodel.BusinessExtensionRoleProvider {
		return gerror.New("customer template identity is invalid")
	}
	if d.InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalitySingleton || d.InterfaceConfiguration == nil {
		return gerror.New("customer template configuration is invalid")
	}
	if d.ProductConfiguration != nil || d.CustomerConfiguration != nil || len(d.Methods) != 1 || d.Methods[0] != MethodDescribe {
		return gerror.New("customer template has unsupported capabilities")
	}
	return nil
}

// validateRenderer accepts only explicitly supported rendering contracts.
func validateRenderer(metadata *capmodel.BusinessExtensionProviderMetadata) error {
	if err := templatecap.Validate(metadata); err != nil {
		return err
	}
	if metadata == nil || metadata.Type != MetadataType || metadata.Version != Version {
		return gerror.New("customer template metadata is invalid")
	}
	if metadata.Values["entry"] != nil {
		return nil
	}
	value, ok := metadata.Values["renderer"].(string)
	if !ok || (value != Renderer && value != MinimalRenderer && value != "static-app-v1") {
		return gerror.New("customer template renderer is unsupported")
	}
	return nil
}
