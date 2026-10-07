// This file defines the shared secret-free Tencent onboarding response projection.

package v1

// ElectronicContractOnboardingItem is the secret-free management projection.
type ElectronicContractOnboardingItem struct {
	Merchant          ElectronicContractMerchantProfile `json:"merchant" dc:"Read-only merchant profile already held by this installation"`
	State             string                            `json:"state" dc:"Onboarding state: not_configured, ready, enterprise_pending, operator_pending, active, or temporarily_unavailable" eg:"ready"`
	ReasonCode        string                            `json:"reasonCode" dc:"Stable user-facing reason code, or empty" eg:"tencent_app_not_configured"`
	ConsoleURL        string                            `json:"consoleUrl,omitempty" dc:"Official one-use Tencent HTTPS URL returned only by link creation"`
	OrganizationReady bool                              `json:"organizationReady" dc:"Whether Tencent reports the enterprise activated"`
	OperatorReady     bool                              `json:"operatorReady" dc:"Whether Tencent reports the onboarding operator verified"`
	BindingNo         string                            `json:"bindingNo,omitempty" dc:"Opaque installation-local subject binding number"`
	SubjectStatus     string                            `json:"subjectStatus" dc:"Local supplier subject projection" eg:"unbound"`
	Capabilities      []ElectronicContractCapability    `json:"capabilities" dc:"Safe seal and download capability checks"`
}

// ElectronicContractMerchantProfile is the existing read-only local merchant identity.
type ElectronicContractMerchantProfile struct {
	LegalName   string `json:"legalName" dc:"Existing merchant legal enterprise name" eg:"Example IDC Co., Ltd."`
	CountryCode string `json:"countryCode" dc:"Existing ISO 3166-1 alpha-2 merchant region" eg:"CN"`
	MerchantNo  string `json:"merchantNo" dc:"Existing local merchant business number" eg:"merchant-local"`
}

// ElectronicContractCapability is one independently checked Tencent ability.
type ElectronicContractCapability struct {
	Code       string `json:"code" dc:"Capability code" eg:"flow.download"`
	State      string `json:"state" dc:"Capability state" eg:"available"`
	ReasonCode string `json:"reasonCode,omitempty" dc:"Stable closed-state reason" eg:"permission_denied"`
	CheckedAt  int64  `json:"checkedAt,omitempty" dc:"Check time as Unix timestamp in milliseconds" eg:"1786464000000"`
}
