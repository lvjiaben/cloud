// This file defines transport-neutral business extension and frontend surface
// declarations shared by plugin manifests, lifecycle snapshots, and host APIs.

package capmodel

import "context"

// Business extension point identifiers published by built-in business owners.
// These identifiers describe where a plugin can be selected; they do not imply
// a runtime type, distribution model, or product type.
const (
	BusinessExtensionProductFulfillment = "product-fulfillment"
	BusinessExtensionSMSChannel         = "sms-channel"
	BusinessExtensionPaymentMethod      = "payment-method"
	BusinessExtensionWebsiteTemplate    = "website-template"
)

// Business extension declaration roles.
const (
	BusinessExtensionRoleOwner    = "owner"
	BusinessExtensionRoleProvider = "provider"
)

// Interface configuration cardinality values declare whether administrators
// may create multiple named interface instances or one global settings record.
const (
	InterfaceConfigurationCardinalityMultiple  = "multiple"
	InterfaceConfigurationCardinalitySingleton = "singleton"
)

// BusinessExtensionProviderMetadata carries one owner-defined, versioned,
// non-executable provider declaration. The host freezes and digests the values
// but delegates their business meaning to the extension owner.
type BusinessExtensionProviderMetadata struct {
	Type    string         `json:"type" yaml:"type"`
	Version string         `json:"version" yaml:"version"`
	Values  map[string]any `json:"values" yaml:"values"`
}

// Governed administration-console surface slots. A plugin selects one of
// these host-published business locations; it cannot create a top-level menu.
const (
	FrontendSlotAdminCatalogProducts       = "admin.catalog-products"
	FrontendSlotAdminResourceOperations    = "admin.resource-operations"
	FrontendSlotAdminOrdersFinance         = "admin.orders-finance"
	FrontendSlotAdminNotificationMarketing = "admin.notification-marketing"
	FrontendSlotAdminComplianceGovernance  = "admin.compliance-governance"
	FrontendSlotAdminSupportContent        = "admin.support-content"
	FrontendSlotAdminExtensionManagement   = "admin.extension-management"
)

// Governed signed-in customer-center surface slots. These names match the
// customer center's stable second-level business groups.
const (
	FrontendSlotCustomerProducts   = "customer.products"
	FrontendSlotCustomerFinance    = "customer.finance"
	FrontendSlotCustomerAccount    = "customer.account"
	FrontendSlotCustomerSecurity   = "customer.security"
	FrontendSlotCustomerCompliance = "customer.compliance"
	FrontendSlotCustomerSupport    = "customer.support"
)

// BusinessExtensionDeclaration declares one versioned business extension
// point published by an owner or implemented by a provider plugin.
type BusinessExtensionDeclaration struct {
	// Point is the stable business extension point identifier.
	Point string `json:"point" yaml:"point"`
	// Version is the extension contract version, for example v1.
	Version string `json:"version" yaml:"version"`
	// Role distinguishes the business owner from an extension provider.
	Role string `json:"role" yaml:"role"`
	// Owner is the plugin ID that publishes the extension contract.
	Owner string `json:"owner" yaml:"owner"`
	// Methods is the bounded set of contract methods requested or published.
	Methods []string `json:"methods,omitempty" yaml:"methods,omitempty"`
	// InterfaceConfiguration declares provider credentials and connection settings.
	InterfaceConfiguration *ConfigurationSchemaDeclaration `json:"interfaceConfiguration,omitempty" yaml:"interface_configuration,omitempty"`
	// InterfaceConfigurationCardinality controls how many interface settings
	// records the owning business page may create. It does not constrain the
	// plugin's own business data or CRUD resources.
	InterfaceConfigurationCardinality string `json:"interfaceConfigurationCardinality,omitempty" yaml:"interface_configuration_cardinality,omitempty"`
	// ProductConfiguration declares merchant-authored product-version settings.
	ProductConfiguration *ConfigurationSchemaDeclaration `json:"productConfiguration,omitempty" yaml:"product_configuration,omitempty"`
	// CustomerConfiguration declares fields collected during customer purchase.
	CustomerConfiguration *ConfigurationSchemaDeclaration `json:"customerConfiguration,omitempty" yaml:"customer_configuration,omitempty"`
	// ProviderMetadata is optional host-opaque metadata validated by the extension owner.
	ProviderMetadata *BusinessExtensionProviderMetadata `json:"providerMetadata,omitempty" yaml:"provider_metadata,omitempty"`
}

// BusinessExtensionProviderEvidence is the minimum immutable proof needed to
// bind or later verify one owner-authorized provider release. It includes only
// safe configuration declarations and excludes private manifests, values, and secrets.
type BusinessExtensionProviderEvidence struct {
	PluginID                          string                             `json:"pluginId"`
	DisplayName                       string                             `json:"displayName"`
	ReleaseVersion                    string                             `json:"releaseVersion"`
	Point                             string                             `json:"point"`
	ContractVersion                   string                             `json:"contractVersion"`
	AuthorizedMethods                 []string                           `json:"authorizedMethods"`
	ReleaseDigest                     string                             `json:"releaseDigest"`
	DeclarationDigest                 string                             `json:"declarationDigest"`
	InterfaceConfiguration            *ConfigurationSchemaDeclaration    `json:"interfaceConfiguration,omitempty"`
	InterfaceConfigurationCardinality string                             `json:"interfaceConfigurationCardinality,omitempty"`
	ProductConfiguration              *ConfigurationSchemaDeclaration    `json:"productConfiguration,omitempty"`
	CustomerConfiguration             *ConfigurationSchemaDeclaration    `json:"customerConfiguration,omitempty"`
	ProviderMetadata                  *BusinessExtensionProviderMetadata `json:"providerMetadata,omitempty"`
}

// BusinessExtensionAdmissionAction commits an owner's binding and audit rows
// while the host still holds the provider lifecycle lock.
type BusinessExtensionAdmissionAction func(
	ctx context.Context,
	evidence *BusinessExtensionProviderEvidence,
) error

// FrontendSurfaceDeclaration describes one governed plugin page surface. Admin
// and customer surfaces are stored in separate manifest fields so identities,
// permissions, and mount locations cannot be mixed accidentally.
type FrontendSurfaceDeclaration struct {
	// Key is stable and unique within the matching admin or customer surface list.
	Key string `json:"key" yaml:"key"`
	// Version is the surface/bridge contract version, for example v1.
	Version string `json:"version" yaml:"version"`
	// Title is the i18n-aware page or menu title declared by the plugin.
	Title string `json:"title" yaml:"title"`
	// Icon is an optional host-supported icon identifier.
	Icon string `json:"icon,omitempty" yaml:"icon,omitempty"`
	// Permission is the host permission required to expose the surface.
	Permission string `json:"permission,omitempty" yaml:"permission,omitempty"`
	// Slot is the host-published mount location.
	Slot string `json:"slot" yaml:"slot"`
	// Route is the plugin-owned route fragment under the governed host namespace.
	Route string `json:"route" yaml:"route"`
	// Entry is the plugin-relative frontend entry asset.
	Entry string `json:"entry" yaml:"entry"`
	// Status is the plugin-defined business availability state shown by the host.
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
	// BridgeCapabilities lists versioned host page-bridge capabilities requested
	// by this surface. Runtime authorization remains method-scoped.
	BridgeCapabilities []string `json:"bridgeCapabilities,omitempty" yaml:"bridge_capabilities,omitempty"`
}
