// This file defines the versioned, secret-free input required to bind one
// generic long-lived entitlement to an idc-resource-owned server mapping.

package resourcecap

import (
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const ServerEntitlementProjectionVersion = "v1"

var (
	serverEntitlementNumberPattern  = regexp.MustCompile(`^ent_[0-9a-f]{32}$`)
	serverOwnershipReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)
	serverOwnershipPluginPattern    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	serverOwnershipDigestPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ServerEntitlementBinding contains only frozen identifiers and digests. The
// caller remains responsible for proving the matching generic entitlement;
// idc-resource proves the matching succeeded Provider task and active mapping.
type ServerEntitlementBinding struct {
	Version                    string
	EntitlementNo              string
	SourceIntentNo             string
	CustomerNo                 string
	ProductNo                  string
	ProductVersionNo           string
	ProductNameZhCN            string
	ProductNameEnUS            string
	LocalResourceNo            string
	MappingNo                  string
	ProviderTaskNo             string
	InterfaceInstanceNo        string
	ProviderPluginID           string
	PluginReleaseVersion       string
	FulfillmentContractVersion string
	ServerContractVersion      string
	ReleaseDigest              string
	DeclarationDigest          string
}

// Validate rejects incomplete or non-server ownership evidence before any
// database lookup. Customer ownership is always an explicit frozen field.
func (binding ServerEntitlementBinding) Validate() error {
	if binding.Version != ServerEntitlementProjectionVersion ||
		!serverEntitlementNumberPattern.MatchString(binding.EntitlementNo) ||
		!validServerOwnershipReference(binding.LocalResourceNo, 64) ||
		!validServerOwnershipReference(binding.SourceIntentNo, 40) ||
		!validServerOwnershipReference(binding.CustomerNo, 40) ||
		!validServerOwnershipReference(binding.ProductNo, 40) ||
		!validServerOwnershipReference(binding.ProductVersionNo, 40) ||
		!validServerProductLabel(binding.ProductNameZhCN) ||
		!validServerProductLabel(binding.ProductNameEnUS) ||
		!validServerOwnershipReference(binding.MappingNo, 64) ||
		!validServerOwnershipReference(binding.ProviderTaskNo, 64) ||
		!validServerOwnershipReference(binding.InterfaceInstanceNo, 40) ||
		!serverOwnershipPluginPattern.MatchString(binding.ProviderPluginID) ||
		!validServerOwnershipVersion(binding.PluginReleaseVersion) ||
		!validServerOwnershipVersion(binding.FulfillmentContractVersion) ||
		binding.ServerContractVersion != ServerFulfillmentContractVersion ||
		!serverOwnershipDigestPattern.MatchString(binding.ReleaseDigest) ||
		!serverOwnershipDigestPattern.MatchString(binding.DeclarationDigest) {
		return gerror.New("resource server entitlement binding is invalid")
	}
	return nil
}

func validServerProductLabel(value string) bool {
	return value == strings.TrimSpace(value) && len(value) >= 1 && len(value) <= 256
}

func validServerOwnershipReference(value string, maximum int) bool {
	return len(value) <= maximum && serverOwnershipReferencePattern.MatchString(value)
}

func validServerOwnershipVersion(value string) bool {
	return value == strings.TrimSpace(value) && len(value) >= 1 && len(value) <= 32
}
