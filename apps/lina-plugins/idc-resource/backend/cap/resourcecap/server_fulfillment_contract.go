// This file publishes the server-specific product-fulfillment contract owned
// by idc-resource. Generic catalog and commerce code must treat the declared
// configuration as opaque plugin data.

package resourcecap

import (
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

const (
	// ServerFulfillmentContractVersion is the exact server release contract
	// currently accepted by the resource owner.
	ServerFulfillmentContractVersion = "v1"

	ServerConfigurationRegion          = "region"
	ServerConfigurationLine            = "line"
	ServerConfigurationCPU             = "cpu"
	ServerConfigurationMemory          = "memory"
	ServerConfigurationOperatingSystem = "operating_system"
	ServerConfigurationSystemDisk      = "system_disk"
	ServerConfigurationDataDisk        = "data_disk"
	ServerConfigurationBandwidth       = "bandwidth"
)

// ServerFulfillmentContract combines the generic product-fulfillment
// declaration with the server owner's capability and optional connector-owned
// frontend surfaces. Shared server pages remain owned by idc-resource. It is
// intentionally validated by idc-resource rather than interpreted by the
// generic product, cart, order, billing, or refund domains.
type ServerFulfillmentContract struct {
	Version          string
	Fulfillment      *capmodel.BusinessExtensionDeclaration
	Capabilities     *CapabilitySnapshot
	AdminSurfaces    []*capmodel.FrontendSurfaceDeclaration
	CustomerSurfaces []*capmodel.FrontendSurfaceDeclaration
}

// ServerConfigurationKeys returns the complete deterministic set of
// server-only configuration semantics a connector release must declare.
func ServerConfigurationKeys() []string {
	return []string{
		ServerConfigurationRegion,
		ServerConfigurationLine,
		ServerConfigurationCPU,
		ServerConfigurationMemory,
		ServerConfigurationOperatingSystem,
		ServerConfigurationSystemDisk,
		ServerConfigurationDataDisk,
		ServerConfigurationBandwidth,
	}
}

// ServerManagementCapabilityCodes returns the server-only lifecycle and
// follow-up actions that remain owned by the resource plugin. Unsupported
// actions still appear explicitly in the Provider capability matrix so the UI
// can hide or degrade them without guessing.
func ServerManagementCapabilityCodes() []CapabilityCode {
	return []CapabilityCode{
		CapabilityStart,
		CapabilityStop,
		CapabilityReboot,
		CapabilitySuspend,
		CapabilityResume,
		CapabilityRenew,
		CapabilityResetPassword,
		CapabilityConsole,
		CapabilityReinstall,
		CapabilityEnterRescue,
		CapabilityExitRescue,
		CapabilityResize,
		CapabilityChangeIP,
		CapabilityManageSnapshot,
		CapabilityBackup,
		CapabilitySecurityGroup,
		CapabilityNAT,
		CapabilityVirtualMedia,
		CapabilityTrafficPackage,
		CapabilityUpstreamTicket,
		CapabilityTerminate,
	}
}

// ValidateAt verifies one connector release is a complete server fulfillment
// implementation. The explicit owner clock keeps capability freshness
// deterministic across nodes and tests.
func (c ServerFulfillmentContract) ValidateAt(now time.Time) error {
	if c.Version != ServerFulfillmentContractVersion {
		return gerror.New("resource server fulfillment contract version is unsupported")
	}
	if err := validateServerFulfillmentDeclaration(c.Fulfillment); err != nil {
		return err
	}
	if c.Capabilities == nil {
		return gerror.New("resource server capability matrix is required")
	}
	if err := c.Capabilities.ValidateAt(now); err != nil {
		return gerror.Wrap(err, "validate resource server capability matrix")
	}
	if err := validateServerManagementCapabilities(c.Capabilities.Capabilities); err != nil {
		return err
	}
	if err := validateOptionalServerSurfaces(
		"administration",
		capmodel.FrontendSlotAdminResourceOperations,
		true,
		c.AdminSurfaces,
	); err != nil {
		return err
	}
	return validateOptionalServerSurfaces(
		"customer",
		capmodel.FrontendSlotCustomerProducts,
		false,
		c.CustomerSurfaces,
	)
}

func validateServerFulfillmentDeclaration(declaration *capmodel.BusinessExtensionDeclaration) error {
	if declaration == nil || declaration.Point != capmodel.BusinessExtensionProductFulfillment ||
		declaration.Version != productfulfillmentcap.ProviderMetadataVersion ||
		declaration.Role != capmodel.BusinessExtensionRoleProvider || declaration.Owner != "idc-core" {
		return gerror.New("resource server product-fulfillment declaration is invalid")
	}
	typed, err := productfulfillmentcap.ParseProviderDeclaration(
		declaration.ProviderMetadata,
		declaration.Methods,
	)
	if err != nil {
		return gerror.Wrap(err, "validate resource server product-fulfillment declaration")
	}
	if !typed.OngoingManagement {
		return gerror.New("resource server fulfillment must declare ongoing management")
	}
	return validateServerConfigurationSchemas(
		declaration.ProductConfiguration,
		declaration.CustomerConfiguration,
	)
}

// ValidateServerProductEvidence verifies that the product-facing half of one
// connector release is a complete server fulfillment declaration. Resource
// channel code uses it before freezing the separate product projection.
func ValidateServerProductEvidence(evidence *capmodel.BusinessExtensionProviderEvidence) error {
	if evidence == nil {
		return gerror.New("resource server product evidence is required")
	}
	return validateServerFulfillmentDeclaration(&capmodel.BusinessExtensionDeclaration{
		Point: evidence.Point, Version: evidence.ContractVersion,
		Role: capmodel.BusinessExtensionRoleProvider, Owner: "idc-core",
		Methods:               append([]string(nil), evidence.AuthorizedMethods...),
		ProductConfiguration:  evidence.ProductConfiguration,
		CustomerConfiguration: evidence.CustomerConfiguration,
		ProviderMetadata:      evidence.ProviderMetadata,
	})
}

func validateServerConfigurationSchemas(schemas ...*capmodel.ConfigurationSchemaDeclaration) error {
	required := make(map[string]bool, len(ServerConfigurationKeys()))
	for _, key := range ServerConfigurationKeys() {
		required[key] = false
	}
	for _, schema := range schemas {
		if schema == nil || schema.Version != ServerFulfillmentContractVersion {
			continue
		}
		for _, field := range schema.Fields {
			if field == nil {
				continue
			}
			if _, tracked := required[field.Key]; !tracked {
				continue
			}
			if !validServerConfigurationFieldType(field.Key, field.Type) {
				return gerror.New("resource server configuration field type is invalid")
			}
			required[field.Key] = true
		}
	}
	for _, key := range ServerConfigurationKeys() {
		if !required[key] {
			return gerror.New("resource server configuration declaration is incomplete")
		}
	}
	return nil
}

func validServerConfigurationFieldType(key string, fieldType string) bool {
	switch key {
	case ServerConfigurationRegion, ServerConfigurationLine, ServerConfigurationOperatingSystem:
		return fieldType == capmodel.ConfigurationFieldSingleSelect ||
			fieldType == capmodel.ConfigurationFieldTreeSelect
	case ServerConfigurationCPU, ServerConfigurationMemory, ServerConfigurationSystemDisk,
		ServerConfigurationDataDisk, ServerConfigurationBandwidth:
		return fieldType == capmodel.ConfigurationFieldInteger ||
			fieldType == capmodel.ConfigurationFieldDecimal ||
			fieldType == capmodel.ConfigurationFieldSingleSelect
	default:
		return false
	}
}

func validateServerManagementCapabilities(capabilities []CapabilityDescriptor) error {
	declared := make(map[CapabilityCode]struct{}, len(capabilities))
	for _, capability := range capabilities {
		declared[capability.Code] = struct{}{}
	}
	for _, code := range ServerManagementCapabilityCodes() {
		if _, ok := declared[code]; !ok {
			return gerror.New("resource server management capability declaration is incomplete")
		}
	}
	return nil
}

func validateOptionalServerSurfaces(
	host string,
	slot string,
	requirePermission bool,
	surfaces []*capmodel.FrontendSurfaceDeclaration,
) error {
	// A connector does not have to duplicate the shared server pages supplied
	// by idc-resource. If it contributes an independent page, every declaration
	// must still use the governed server slot and a complete route binding.
	for _, surface := range surfaces {
		if surface == nil || surface.Version != ServerFulfillmentContractVersion ||
			surface.Slot != slot || surface.Key == "" || surface.Title == "" ||
			surface.Route == "" || surface.Entry == "" {
			return gerror.Newf("resource server %s surface is invalid", host)
		}
		if requirePermission && surface.Permission == "" {
			return gerror.Newf("resource server %s surface permission is required", host)
		}
	}
	return nil
}
