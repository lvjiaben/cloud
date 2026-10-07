// Package secretcap publishes the stable, owner-scoped source-plugin secret
// handle and accessor contract. Encryption, persistence, authorization, and
// key rotation remain private owner implementations. Dynamic plaintext access
// is deliberately unavailable until a confidential transport is published.
package secretcap

import (
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// maximumDependencyOwnerPluginIDBytes bounds owner validation before comparison.
	maximumDependencyOwnerPluginIDBytes = 128
	// maximumDependencyServiceBytes bounds service validation before comparison.
	maximumDependencyServiceBytes = 64
	// maximumDependencyVersionBytes bounds version validation before comparison.
	maximumDependencyVersionBytes = 32
	// maximumDependencyMethodCount bounds method iteration and duplicate tracking.
	maximumDependencyMethodCount = 32
	// maximumDependencyMethodBytes bounds each method before normalization or comparison.
	maximumDependencyMethodBytes = 128
)

// Version identifies one exact secretcap protocol version.
type Version string

// Method identifies one versioned secretcap method.
type Method string

const (
	// OwnerPluginID is the only plugin allowed to publish this capability.
	OwnerPluginID = "idc-core"
	// ServiceName is the owner-aware host-service key for this capability.
	ServiceName = "secret"
	// SourceContractPackage is the public Go contract imported by source plugins.
	SourceContractPackage = "lina-plugin-idc-core/backend/cap/secretcap"
	// DynamicContractPackage reserves the future public bridge SDK package path.
	// No dynamic descriptor is published until a confidential, zeroizable
	// response transport has been designed and independently reviewed.
	DynamicContractPackage = SourceContractPackage + "/bridge"
	// ContractVersion is the current exact secret-access protocol version.
	ContractVersion Version = "v1"
	// MethodSecretAccess scopes a confidential access operation to one handle.
	MethodSecretAccess Method = "secret.access"
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
		Methods:       methods,
	}
	if err := ValidateDependency(dependency); err != nil {
		return Dependency{}, err
	}
	dependency.Methods = append([]Method(nil), methods...)
	return dependency, nil
}

// SupportedMethods returns a detached, deterministic method list for the
// current contract version.
func SupportedMethods() []Method {
	return []Method{MethodSecretAccess}
}

// RequireCompatibleVersion rejects any version that is not implemented by
// this package. Protocol versions are exact owner-aware identifiers, not
// plugin semantic-version ranges.
func RequireCompatibleVersion(version Version) error {
	if len(version) > maximumDependencyVersionBytes || version != ContractVersion {
		return gerror.New("secretcap: incompatible contract version")
	}
	return nil
}

// ValidateDependency fails closed unless owner, service, exact version, and
// every requested method match the capability published by this package.
func ValidateDependency(dependency Dependency) error {
	if err := validateDependencyBounds(dependency); err != nil {
		return err
	}
	if dependency.OwnerPluginID != strings.TrimSpace(dependency.OwnerPluginID) ||
		dependency.OwnerPluginID != OwnerPluginID {
		return gerror.New("secretcap: incompatible owner plugin")
	}
	if dependency.Service != strings.TrimSpace(dependency.Service) ||
		dependency.Service != ServiceName {
		return gerror.New("secretcap: incompatible service")
	}
	if err := RequireCompatibleVersion(dependency.Version); err != nil {
		return err
	}
	if len(dependency.Methods) == 0 {
		return gerror.New("secretcap: dependency must require at least one method")
	}
	seen := make(map[Method]struct{}, len(dependency.Methods))
	for _, method := range dependency.Methods {
		if method != Method(strings.TrimSpace(string(method))) || method != MethodSecretAccess {
			return gerror.New("secretcap: unsupported dependency method")
		}
		if _, exists := seen[method]; exists {
			return gerror.New("secretcap: duplicate dependency method")
		}
		seen[method] = struct{}{}
	}
	return nil
}

// validateDependencyBounds rejects oversized dependency fields before any
// normalization, compatibility comparison, allocation, or method iteration.
func validateDependencyBounds(dependency Dependency) error {
	if len(dependency.OwnerPluginID) > maximumDependencyOwnerPluginIDBytes {
		return gerror.New("secretcap: dependency owner plugin is too long")
	}
	if len(dependency.Service) > maximumDependencyServiceBytes {
		return gerror.New("secretcap: dependency service is too long")
	}
	if len(dependency.Version) > maximumDependencyVersionBytes {
		return gerror.New("secretcap: dependency version is too long")
	}
	if len(dependency.Methods) > maximumDependencyMethodCount {
		return gerror.New("secretcap: dependency requires too many methods")
	}
	for _, method := range dependency.Methods {
		if len(method) > maximumDependencyMethodBytes {
			return gerror.New("secretcap: dependency method is too long")
		}
	}
	return nil
}
