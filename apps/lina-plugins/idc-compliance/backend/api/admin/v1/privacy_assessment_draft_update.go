// Privacy assessment draft API accepts a complete safe-text draft and optimistic version. It never accepts cross-border processing or raw personal data.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyAssessmentDraftUpdateReq struct {
	g.Meta `path:"/admin/compliance/privacy/assessments/draft" method:"put" tags:"IDC Compliance Privacy" summary:"Save privacy impact assessment draft" dc:"Create or update one complete assessment draft. Cross-border processing is never accepted." permission:"idc:admin:compliance:privacy:configure"`
	PrivacyAssessmentDraftPayload
}

type PrivacyAssessmentDraftUpdateRes struct {
	Item PrivacyAssessmentItem `json:"item" dc:"Saved draft" eg:"{}"`
}
