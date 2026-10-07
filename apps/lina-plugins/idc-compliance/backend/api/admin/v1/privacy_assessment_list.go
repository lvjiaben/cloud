// Privacy assessment list API returns a bounded installation-local history. It exposes governance text only and never private references.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyAssessmentListReq struct {
	g.Meta             `path:"/admin/compliance/privacy/assessments" method:"get" tags:"IDC Compliance Privacy" summary:"List privacy impact assessments" dc:"Return a bounded installation-local assessment history without personal data or private references." permission:"idc:admin:compliance:privacy:query"`
	AssessmentTypeCode string `json:"assessmentTypeCode" v:"max-length:40" dc:"Optional exact assessment type" eg:"personal_verification"`
	PageSize           int    `json:"pageSize" v:"min:0|max:100" dc:"Bounded history size; zero selects 60" eg:"60"`
}

type PrivacyAssessmentListRes struct {
	Items []PrivacyAssessmentItem `json:"items" dc:"Assessment history" eg:"[]"`
}
