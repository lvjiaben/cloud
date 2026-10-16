// Package websitecap defines the declarative, code-free website template contract.
package websitecap

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-portal/backend/cap/templatecap"
)

// Stable website extension identifiers are shared by owners and providers.
const (
	Point           = "website-template"
	Version         = "v1"
	Owner           = "idc-portal"
	MetadataType    = "website-template-contract"
	Renderer        = "public-standard-v1"
	MinimalRenderer = "public-minimal-v1"
	MethodDescribe  = "capabilities.describe"
)

// ValidateDeclarations accepts bounded public configuration, never secrets or executable code.
func ValidateDeclarations(items []*capmodel.BusinessExtensionDeclaration) error {
	if len(items) != 1 || items[0] == nil {
		return gerror.New("website template declaration is incomplete")
	}
	d := items[0]
	if err := validateIdentity(d); err != nil {
		return err
	}
	if err := templatecap.Validate(d.ProviderMetadata); err != nil {
		return err
	}
	if err := validateRenderer(d.ProviderMetadata); err != nil {
		return err
	}
	if len(d.InterfaceConfiguration.Fields) > 30 {
		return gerror.New("website template has too many settings")
	}
	for _, f := range d.InterfaceConfiguration.Fields {
		if f == nil {
			return gerror.New("website template field is invalid")
		}
		switch f.Type {
		case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldInteger:
		default:
			return gerror.New("website template field type is unsupported")
		}
	}
	_, err := Pages(d.ProviderMetadata)
	return err
}

// validateIdentity requires one provider-only singleton contract.
func validateIdentity(d *capmodel.BusinessExtensionDeclaration) error {
	if d.Point != Point || d.Version != Version || d.Owner != Owner || d.Role != capmodel.BusinessExtensionRoleProvider {
		return gerror.New("website template identity is invalid")
	}
	if d.InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalitySingleton || d.InterfaceConfiguration == nil {
		return gerror.New("website template configuration is invalid")
	}
	if d.ProductConfiguration != nil || d.CustomerConfiguration != nil || len(d.Methods) != 1 || d.Methods[0] != MethodDescribe {
		return gerror.New("website template has unsupported capabilities")
	}
	return nil
}

// validateRenderer accepts only explicitly supported rendering contracts.
func validateRenderer(metadata *capmodel.BusinessExtensionProviderMetadata) error {
	if metadata == nil || metadata.Type != MetadataType || metadata.Version != Version {
		return gerror.New("website template metadata is invalid")
	}
	if metadata.Values["entry"] != nil {
		return nil
	}
	value, ok := metadata.Values["renderer"].(string)
	if !ok || (value != Renderer && value != MinimalRenderer) {
		return gerror.New("website template renderer is unsupported")
	}
	return nil
}
