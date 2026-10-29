// Customer consent API types expose only customer-owned decisions, without private storage references.

package v1

// CustomerPrivacyConsent is the latest decision for one consent category.
type CustomerPrivacyConsent struct {
	ConsentNo       string `json:"consentNo" dc:"Opaque consent decision number" eg:"PVC_0123456789abcdef0123456789abcdef"`
	ConsentTypeCode string `json:"consentTypeCode" dc:"Stable consent category" eg:"sensitive_personal_information"`
	ActionCode      string `json:"actionCode" dc:"Latest grant or withdrawal decision" eg:"granted"`
	DecidedAt       int64  `json:"decidedAt" dc:"Decision time as Unix milliseconds" eg:"1787047200000"`
}
