// Privacy assessment publication API requires explicit confirmation and a replay key. It publishes one draft without changing its historical content.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyAssessmentPublishCreateReq struct {
	g.Meta             `path:"/admin/compliance/privacy/assessments/publish" method:"post" tags:"IDC Compliance Privacy" summary:"Publish privacy impact assessment" dc:"Publish one complete draft after explicit administrator confirmation and immutable audit." permission:"idc:admin:compliance:privacy:configure"`
	AssessmentNo       string `json:"assessmentNo" v:"required|max-length:40" dc:"Draft assessment number" eg:"PIA_0123456789abcdef0123456789abcdef"`
	ExpectedRowVersion int64  `json:"expectedRowVersion" v:"required|min:1" dc:"Current draft row version" eg:"1"`
	Confirmed          bool   `json:"confirmed" v:"required" dc:"Explicit publication confirmation" eg:"true"`
	IdempotencyKey     string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Replay-safe browser request key" eg:"privacy-publish-0123456789abcdef"`
}

type PrivacyAssessmentPublishCreateRes struct {
	Item PrivacyAssessmentItem `json:"item" dc:"Published assessment" eg:"{}"`
}
