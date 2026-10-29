// This file defines the owner-governed managed payment-method declaration.
package paymentcap

import (
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const (
	DynamicProviderPoint           = "payment-method"
	DynamicProviderContractVersion = "v1"
	DynamicProviderMetadataType    = "payment-method-contract"
	DynamicProviderMethodDescribe  = "capabilities.describe"
	OwnerPluginID                  = "idc-payment"
)

type DynamicProviderDescriptor struct {
	ProviderCode   ProviderCode
	EndpointOrigin string
	Modes          []Mode
	Currencies     []string
}

func DynamicProviderMethods() []string {
	return []string{DynamicProviderMethodDescribe}
}

func ValidateDynamicProviderDeclarations(declarations []*capmodel.BusinessExtensionDeclaration) error {
	if len(declarations) != 1 || declarations[0] == nil {
		return gerror.New("payment method declaration is incomplete")
	}
	declaration := declarations[0]
	if declaration.Point != DynamicProviderPoint || declaration.Version != DynamicProviderContractVersion ||
		declaration.Role != capmodel.BusinessExtensionRoleProvider || declaration.Owner != OwnerPluginID ||
		declaration.InterfaceConfiguration == nil ||
		declaration.InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalityMultiple ||
		declaration.ProductConfiguration != nil || declaration.CustomerConfiguration != nil {
		return gerror.New("payment method declaration is invalid")
	}
	want, got := DynamicProviderMethods(), append([]string(nil), declaration.Methods...)
	sort.Strings(want)
	sort.Strings(got)
	if len(want) != len(got) {
		return gerror.New("payment method declaration methods are incomplete")
	}
	for index := range want {
		if want[index] != got[index] {
			return gerror.New("payment method declaration methods are incomplete")
		}
	}
	_, err := DecodeDynamicProviderMetadata(declaration.ProviderMetadata)
	return err
}

func DecodeDynamicProviderMetadata(metadata *capmodel.BusinessExtensionProviderMetadata) (DynamicProviderDescriptor, error) {
	if metadata == nil || metadata.Type != DynamicProviderMetadataType ||
		metadata.Version != DynamicProviderContractVersion || metadata.Values == nil {
		return DynamicProviderDescriptor{}, gerror.New("payment method metadata is invalid")
	}
	providerText, ok := metadata.Values["providerCode"].(string)
	provider := ProviderCode(strings.TrimSpace(providerText))
	if !ok || !knownProvider(provider) {
		return DynamicProviderDescriptor{}, gerror.New("payment method metadata provider is invalid")
	}
	endpoint, ok := metadata.Values["endpointOrigin"].(string)
	endpoint = strings.TrimSpace(endpoint)
	if !ok || endpoint == "" || len(endpoint) > 2048 {
		return DynamicProviderDescriptor{}, gerror.New("payment method metadata endpoint is invalid")
	}
	modes, err := metadataModes(metadata.Values["modes"], provider)
	if err != nil {
		return DynamicProviderDescriptor{}, err
	}
	currencies, err := metadataCurrencies(metadata.Values["currencies"])
	if err != nil {
		return DynamicProviderDescriptor{}, err
	}
	return DynamicProviderDescriptor{ProviderCode: provider, EndpointOrigin: endpoint, Modes: modes, Currencies: currencies}, nil
}

func knownProvider(provider ProviderCode) bool {
	switch provider {
	case ProviderAlipay, ProviderWechatPay, ProviderEPay, ProviderUnionPay:
		return true
	default:
		return false
	}
}

func metadataModes(raw any, provider ProviderCode) ([]Mode, error) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 || len(values) > 16 {
		return nil, gerror.New("payment method metadata modes are invalid")
	}
	allowed := map[ProviderCode]map[Mode]bool{
		ProviderAlipay:    {ModeAlipayPage: true, ModeAlipayWAP: true},
		ProviderWechatPay: {ModeWechatH5: true, ModeWechatNative: true, ModeWechatJSAPI: true},
		ProviderEPay:      {ModeEPayCashier: true, ModeEPayAlipay: true, ModeEPayWechat: true},
		ProviderUnionPay:  {ModeUnionPayWeb: true, ModeUnionPayWAP: true},
	}
	result := make([]Mode, 0, len(values))
	seen := map[Mode]bool{}
	for _, rawValue := range values {
		text, ok := rawValue.(string)
		mode := Mode(strings.TrimSpace(text))
		if !ok || !allowed[provider][mode] || seen[mode] {
			return nil, gerror.New("payment method metadata modes are invalid")
		}
		seen[mode] = true
		result = append(result, mode)
	}
	return result, nil
}

func metadataCurrencies(raw any) ([]string, error) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 || len(values) > 16 {
		return nil, gerror.New("payment method metadata currencies are invalid")
	}
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, rawValue := range values {
		currency, ok := rawValue.(string)
		currency = strings.TrimSpace(currency)
		if !ok || !currencyPattern.MatchString(currency) || seen[currency] {
			return nil, gerror.New("payment method metadata currencies are invalid")
		}
		seen[currency] = true
		result = append(result, currency)
	}
	return result, nil
}
