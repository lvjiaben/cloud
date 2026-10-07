// This file defines server-trusted personal-verification initiation.

package v1

import "github.com/gogf/gf/v2/frame/g"

// PersonalVerificationCreateReq starts one personal verification attempt.
type PersonalVerificationCreateReq struct {
	g.Meta    `path:"/account/compliance/personal-verifications" method:"post" tags:"IDC Compliance Account" summary:"Start personal verification" dc:"Start one Aliyun H5 FACE attempt for the authenticated personal customer. Customer number, tenant number, and return URL are derived exclusively by the server."`
	ConsentNo string `json:"consentNo" v:"required|max-length:40" dc:"Latest effective separate sensitive-information consent number" eg:"CNS_0123456789abcdef0123456789abcdef"`
	CertType  string `json:"certType" v:"required|max-length:32" dc:"Supported identity document type; the first release accepts only IDENTITY_CARD" eg:"IDENTITY_CARD"`
	CertName  string `json:"certName" v:"required|max-length:64" dc:"Legal name consumed transiently for encrypted snapshot creation and Aliyun initiation" eg:"Test User"`
	CertNo    string `json:"certNo" v:"required|max-length:64" dc:"Mainland China 18-digit resident identity number consumed transiently" eg:"110101200002290019"`
}

// PersonalVerificationCreateRes returns the fresh H5 continuation and safe state.
type PersonalVerificationCreateRes struct {
	Item *PersonalVerificationItem `json:"item" dc:"Fresh verification attempt and HTTPS H5 continuation" eg:"{}"`
}
