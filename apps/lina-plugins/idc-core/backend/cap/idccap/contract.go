// Package idccap publishes the stable, owner-scoped base contract shared by
// IDC plugins. It contains contracts and descriptor metadata only; runtime
// implementations are supplied explicitly by the owner plugin.
package idccap

import (
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Version identifies one exact idccap protocol version.
type Version string

// Method identifies one versioned idccap method.
type Method string

const (
	// OwnerPluginID is the only plugin allowed to publish this capability.
	OwnerPluginID = "idc-core"
	// ServiceName is the owner-aware host-service key for this capability.
	ServiceName = "idc"
	// SourceContractPackage is the public Go contract imported by source plugins.
	SourceContractPackage = "lina-plugin-idc-core/backend/cap/idccap"
	// DynamicContractPackage is the reserved public bridge SDK package path.
	// Naming the boundary does not register or enable a runtime bridge.
	DynamicContractPackage = SourceContractPackage + "/bridge"
	// ContractVersion is the current exact IDC base protocol version.
	ContractVersion Version = "v1"
	// MethodMerchantContextCurrent returns the installation merchant context.
	MethodMerchantContextCurrent Method = "merchant.context.current"
)

// Dependency is a pure owner/service/version/method requirement that source
// composition or a dynamic hostServices bridge can validate before use.
type Dependency struct {
	// OwnerPluginID identifies the plugin that must own the capability.
	OwnerPluginID string `json:"ownerPluginId"`
	// Service identifies the owner-scoped capability service.
	Service string `json:"service"`
	// Version is the exact protocol version required by the consumer.
	Version Version `json:"version"`
	// Methods lists the concrete methods required by the consumer.
	Methods []Method `json:"methods"`
}

// NewDependency creates a validated dependency on this contract. When methods
// is empty, it requires every method in the current contract.
func NewDependency(methods ...Method) (Dependency, error) {
	if len(methods) == 0 {
		methods = SupportedMethods()
	}
	dependency := Dependency{
		OwnerPluginID: OwnerPluginID,
		Service:       ServiceName,
		Version:       ContractVersion,
		Methods:       append([]Method(nil), methods...),
	}
	if err := ValidateDependency(dependency); err != nil {
		return Dependency{}, err
	}
	return dependency, nil
}

// SupportedMethods returns a detached, deterministic method list for the
// current contract version.
func SupportedMethods() []Method {
	return []Method{MethodMerchantContextCurrent}
}

// RequireCompatibleVersion rejects any version that is not implemented by
// this package. Protocol versions are exact owner-aware identifiers, not
// plugin semantic-version ranges.
func RequireCompatibleVersion(version Version) error {
	if version != ContractVersion {
		return gerror.Newf(
			"idccap: incompatible contract version: required=%s available=%s",
			version,
			ContractVersion,
		)
	}
	return nil
}

// ValidateDependency fails closed unless owner, service, exact version, and
// every requested method match the capability published by this package.
func ValidateDependency(dependency Dependency) error {
	if dependency.OwnerPluginID != strings.TrimSpace(dependency.OwnerPluginID) ||
		dependency.OwnerPluginID != OwnerPluginID {
		return gerror.Newf(
			"idccap: incompatible owner plugin: required=%q available=%q",
			dependency.OwnerPluginID,
			OwnerPluginID,
		)
	}
	if dependency.Service != strings.TrimSpace(dependency.Service) ||
		dependency.Service != ServiceName {
		return gerror.Newf(
			"idccap: incompatible service: required=%q available=%q",
			dependency.Service,
			ServiceName,
		)
	}
	if err := RequireCompatibleVersion(dependency.Version); err != nil {
		return err
	}
	if len(dependency.Methods) == 0 {
		return gerror.New("idccap: dependency must require at least one method")
	}
	seen := make(map[Method]struct{}, len(dependency.Methods))
	for _, method := range dependency.Methods {
		if method != Method(strings.TrimSpace(string(method))) || !supportsMethod(method) {
			return gerror.Newf("idccap: unsupported dependency method: %q", method)
		}
		if _, exists := seen[method]; exists {
			return gerror.Newf("idccap: duplicate dependency method: %q", method)
		}
		seen[method] = struct{}{}
	}
	return nil
}

func supportsMethod(method Method) bool {
	return method == MethodMerchantContextCurrent
}
