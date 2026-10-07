// Privacy retention API types expose versioned merchant-configured durations. No retention duration is hardcoded in these contracts.

package v1

type PrivacyRetentionPolicyItem struct {
	PolicyNo      string `json:"policyNo" dc:"Stable retention policy number" eg:"PRP_0123456789abcdef0123456789abcdef"`
	DataTypeCode  string `json:"dataTypeCode" dc:"Governed data type" eg:"personal_verification_material"`
	Version       int64  `json:"version" dc:"Policy version" eg:"1"`
	RetentionDays int    `json:"retentionDays" dc:"Merchant-configured positive number of days" eg:"365"`
	StatusCode    string `json:"statusCode" dc:"Active or superseded" eg:"active"`
	CreatedBy     int    `json:"createdBy" dc:"Creating administrator identifier" eg:"1"`
	CreatedAt     int64  `json:"createdAt" dc:"Creation time in Unix milliseconds" eg:"1787040000000"`
}
