// Package resourcecap publishes stable resource-operation contracts for IDC plugins.
package resourcecap

import (
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// ContractVersion is the current Provider SPI contract version.
	ContractVersion = "v1"
	// ReadContractVersion is the current read-only resource projection contract version.
	ReadContractVersion = ContractVersion

	// maxIdentifierLength bounds plugin and Provider IDs.
	maxIdentifierLength = 64
	// maxContractVersionLength bounds parsing before splitting components.
	maxContractVersionLength = 32
)

// ProviderKey is the complete declaration identity for one Provider factory.
// ContractVersion remains part of the key for future explicitly tested owner
// ranges. The current registrar accepts only the exact ContractVersion value.
type ProviderKey struct {
	// PluginID identifies the source plugin that implements the Provider.
	PluginID string
	// ProviderID identifies the Provider implementation within that plugin.
	ProviderID string
	// ContractVersion identifies the resourcecap contract implemented by the factory.
	ContractVersion string
}

// Validate rejects keys that cannot safely participate in startup composition.
func (k ProviderKey) Validate() error {
	if err := ValidatePluginID(k.PluginID); err != nil {
		return err
	}
	if err := ValidateProviderID(k.ProviderID); err != nil {
		return err
	}
	if _, err := ParseContractVersion(k.ContractVersion); err != nil {
		return err
	}
	return nil
}

// ParsedContractVersion is the numeric representation of a validated
// resourcecap contract version.
type ParsedContractVersion struct {
	// Major is the compatibility boundary. Different major versions cannot be composed.
	Major uint32
	// Minor identifies backward-compatible contract additions.
	Minor uint32
	// Patch identifies backward-compatible contract corrections.
	Patch uint32
}

// CompatibleWith reports whether two parsed versions are exactly compatible
// under the owner's current support policy. Minor or patch compatibility must
// be added explicitly with contract tests before this policy is broadened.
func (v ParsedContractVersion) CompatibleWith(other ParsedContractVersion) bool {
	return v.Major > 0 && v == other
}

// String returns the only canonical text form for this numeric contract
// version. Zero-valued trailing components are omitted so one semantic version
// can never create multiple declaration keys.
func (v ParsedContractVersion) String() string {
	if v.Major == 0 {
		return ""
	}
	if v.Patch > 0 {
		return "v" + strconv.FormatUint(uint64(v.Major), 10) + "." +
			strconv.FormatUint(uint64(v.Minor), 10) + "." +
			strconv.FormatUint(uint64(v.Patch), 10)
	}
	if v.Minor > 0 {
		return "v" + strconv.FormatUint(uint64(v.Major), 10) + "." +
			strconv.FormatUint(uint64(v.Minor), 10)
	}
	return "v" + strconv.FormatUint(uint64(v.Major), 10)
}

// ParseContractVersion parses vMAJOR, vMAJOR.MINOR, or vMAJOR.MINOR.PATCH.
// Numeric components must be canonical unsigned decimals and MAJOR must be positive.
func ParseContractVersion(value string) (ParsedContractVersion, error) {
	if len(value) > maxContractVersionLength || containsCredentialMaterial(value) {
		return ParsedContractVersion{}, gerror.New("resource contract version is invalid")
	}
	if len(value) < 2 || value[0] != 'v' {
		return ParsedContractVersion{}, gerror.New("resource contract version must start with v")
	}
	parts := strings.Split(value[1:], ".")
	if len(parts) < 1 || len(parts) > 3 {
		return ParsedContractVersion{}, gerror.New("resource contract version must contain one to three numeric components")
	}

	components := [3]uint32{}
	for index, part := range parts {
		component, err := parseVersionComponent(part)
		if err != nil {
			return ParsedContractVersion{}, err
		}
		components[index] = component
	}
	if components[0] == 0 {
		return ParsedContractVersion{}, gerror.New("resource contract version must use a positive major version")
	}
	parsed := ParsedContractVersion{
		Major: components[0],
		Minor: components[1],
		Patch: components[2],
	}
	if parsed.String() != value {
		return ParsedContractVersion{}, gerror.New("resource contract version is not canonical")
	}
	return parsed, nil
}

// ValidatePluginID validates the Provider source-plugin identifier against the
// host's runtime-safe kebab-case boundary.
func ValidatePluginID(value string) error {
	return validateIdentifier("plugin", value)
}

// ValidateProviderID validates the stable identifier of one Provider implementation.
func ValidateProviderID(value string) error {
	return validateIdentifier("provider", value)
}

// validateIdentifier validates one stable kebab-case plugin or Provider ID.
func validateIdentifier(kind string, value string) error {
	if value == "" {
		return gerror.Newf("resource %s ID is required", kind)
	}
	if len(value) > maxIdentifierLength {
		return gerror.Newf("resource %s ID exceeds %d bytes", kind, maxIdentifierLength)
	}
	previousHyphen := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '-' {
			if index == 0 || index == len(value)-1 || previousHyphen {
				return invalidIdentifierError(kind)
			}
			previousHyphen = true
			continue
		}
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return invalidIdentifierError(kind)
		}
		previousHyphen = false
	}
	return nil
}

// invalidIdentifierError returns a bounded identifier error without echoing
// the untrusted identifier.
func invalidIdentifierError(kind string) error {
	return gerror.Newf("resource %s ID must use kebab-case lowercase letters and digits", kind)
}

// parseVersionComponent parses one canonical unsigned decimal component
// without reflecting untrusted input in an error.
func parseVersionComponent(component string) (uint32, error) {
	if component == "" || (len(component) > 1 && component[0] == '0') {
		return 0, gerror.New("resource contract version contains a non-canonical component")
	}
	for index := 0; index < len(component); index++ {
		if component[index] < '0' || component[index] > '9' {
			return 0, gerror.New("resource contract version contains a non-numeric component")
		}
	}
	value, err := strconv.ParseUint(component, 10, 32)
	if err != nil {
		return 0, gerror.New("resource contract version contains an out-of-range component")
	}
	return uint32(value), nil
}
