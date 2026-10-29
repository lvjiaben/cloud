// This file validates bounded inputs and trusted authorization evidence.

package productautomation

import (
	"encoding/hex"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

var (
	// ErrInvalidSelectionToken is the only codec error classification that may
	// be exposed as invalid caller input; every other codec failure is unavailable.
	ErrInvalidSelectionToken = gerror.New("invalid product automation provider selection token")
	businessNoPattern        = regexp.MustCompile(`^(pfi|ver)_[0-9a-f]{32}$`)
	pluginIDPattern          = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	methodPattern            = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	digestPattern            = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func normalizePage(input PageInput) (PageInput, error) {
	if input.Page == 0 {
		input.Page = 1
	}
	if input.PageSize == 0 {
		input.PageSize = defaultPageSize
	}
	if input.Page < 1 || input.PageSize < 1 || input.PageSize > maximumPageSize {
		return PageInput{}, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	input.NamePrefix = strings.TrimSpace(input.NamePrefix)
	if len([]rune(input.NamePrefix)) > 128 ||
		len(input.ProviderSelectionToken) > 4096 ||
		input.ProviderSelectionToken != strings.TrimSpace(input.ProviderSelectionToken) ||
		(input.NewBusinessState != "" && input.NewBusinessState != InterfaceStateAccepting && input.NewBusinessState != InterfaceStateStopped) {
		return PageInput{}, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return input, nil
}

func escapeLikePrefix(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value) + "%"
}

func requireAuthorization(value Authorization) error {
	if value.ActorUserID <= 0 || strings.TrimSpace(value.RequiredPermission) == "" || value.RequiredPermission != strings.TrimSpace(value.RequiredPermission) {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return nil
}

func validateDisplayName(value string) error {
	if value == "" || value != strings.TrimSpace(value) || len([]rune(value)) > 128 {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return nil
}

func validateBusinessNo(value, prefix string) error {
	if !businessNoPattern.MatchString(value) || !strings.HasPrefix(value, prefix+"_") {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return nil
}

func newInterfaceNo(random io.Reader) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", gerror.Wrap(err, "generate automation interface number")
	}
	return "pfi_" + hex.EncodeToString(value), nil
}

func validProviderEvidence(value *capmodel.BusinessExtensionProviderEvidence) bool {
	if !validFrozenEvidence(value) {
		return false
	}
	_, err := productfulfillmentcap.ParseProviderDeclaration(value.ProviderMetadata, value.AuthorizedMethods)
	return err == nil
}

func validFrozenEvidence(value *capmodel.BusinessExtensionProviderEvidence) bool {
	if value == nil || !pluginIDPattern.MatchString(value.PluginID) || len(value.PluginID) > 64 || strings.TrimSpace(value.DisplayName) == "" ||
		value.ReleaseVersion != strings.TrimSpace(value.ReleaseVersion) || value.ReleaseVersion == "" || len(value.ReleaseVersion) > 32 ||
		value.Point != capmodel.BusinessExtensionProductFulfillment ||
		value.ContractVersion != strings.TrimSpace(value.ContractVersion) || value.ContractVersion == "" || len(value.ContractVersion) > 32 ||
		value.AuthorizedMethods == nil || !digestPattern.MatchString(value.ReleaseDigest) ||
		!digestPattern.MatchString(value.DeclarationDigest) {
		return false
	}
	previous := ""
	for _, method := range value.AuthorizedMethods {
		if !methodPattern.MatchString(method) || method <= previous {
			return false
		}
		previous = method
	}
	if value.InterfaceConfiguration != nil {
		if value.InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalityMultiple {
			return false
		}
		if _, err := validateConfigurationSchema(value.InterfaceConfiguration, configurationSurfaceInterface, value.AuthorizedMethods); err != nil {
			return false
		}
	} else if value.InterfaceConfigurationCardinality != "" {
		return false
	}
	if value.ProductConfiguration != nil {
		if _, err := validateConfigurationSchema(value.ProductConfiguration, configurationSurfaceProduct, value.AuthorizedMethods); err != nil {
			return false
		}
	}
	if value.CustomerConfiguration != nil {
		if _, err := validateConfigurationSchema(value.CustomerConfiguration, configurationSurfaceCustomer, value.AuthorizedMethods); err != nil {
			return false
		}
	}
	return true
}

func sameEvidence(left, right *capmodel.BusinessExtensionProviderEvidence) bool {
	if !validFrozenEvidence(left) || !validFrozenEvidence(right) || left.PluginID != right.PluginID ||
		left.ReleaseVersion != right.ReleaseVersion || left.Point != right.Point || left.ContractVersion != right.ContractVersion ||
		left.ReleaseDigest != right.ReleaseDigest || left.DeclarationDigest != right.DeclarationDigest || len(left.AuthorizedMethods) != len(right.AuthorizedMethods) ||
		!sameOptionalConfiguration(left.InterfaceConfiguration, right.InterfaceConfiguration) ||
		!sameOptionalConfiguration(left.ProductConfiguration, right.ProductConfiguration) ||
		!sameOptionalConfiguration(left.CustomerConfiguration, right.CustomerConfiguration) {
		return false
	}
	for index := range left.AuthorizedMethods {
		if left.AuthorizedMethods[index] != right.AuthorizedMethods[index] {
			return false
		}
	}
	return true
}

func sameProviderContract(frozen, exact *capmodel.BusinessExtensionProviderEvidence) bool {
	// Editing and execution follow the current provider. Old package/declaration
	// digests never decide whether an existing interface remains usable.
	return frozen != nil && validProviderEvidence(exact) && frozen.PluginID == exact.PluginID && frozen.Point == exact.Point && frozen.ContractVersion == exact.ContractVersion
}

func sameOptionalConfiguration(left, right *capmodel.ConfigurationSchemaDeclaration) bool {
	return left == nil || right == nil || reflect.DeepEqual(left, right)
}

// sameCompatibleProductConfiguration retains frozen choices after an adapter-
// proven optional extension; it is never used for credentials or interface data.
func sameCompatibleProductConfiguration(stored, current *capmodel.ConfigurationSchemaDeclaration) bool {
	return sameExactConfiguration(stored, current) || productfulfillmentcap.IsAdditiveConfigurationSchema(stored, current)
}
