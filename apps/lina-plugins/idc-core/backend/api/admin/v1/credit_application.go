// This file defines credit application directory and review management contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ManagementCreditApplicationListReq lists bounded credit applications for staff.
type ManagementCreditApplicationListReq struct {
	Page         int `json:"page" v:"min:0" dc:"Optional one-based page number; omit or zero for cursor paging; cannot combine with cursor" eg:"1"`
	g.Meta       `path:"/admin/billing/credit/applications" method:"get" tags:"IDC Credit Management" summary:"List credit applications" dc:"Return one bounded page and exact filtered total after exact application-list permission." permission:"idc:admin:billing:credit-application:list"`
	CustomerNo   string `json:"customerNo" v:"max-length:40" dc:"Optional exact customer number" eg:"CUS_01J2ABCDEF"`
	StatusCode   string `json:"statusCode" dc:"Optional exact application status: pending_review, approved, rejected, cancelled, or expired" eg:"pending_review"`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum applications returned in this page" eg:"20"`
}

// ManagementCreditApplicationListRes returns one bounded application page.
type ManagementCreditApplicationListRes struct {
	Total        int                            `json:"total" dc:"Exact authorized record count matching the current filters before pagination" eg:"123"`
	Applications []*ManagementCreditApplication `json:"applications" dc:"Bounded authorized credit applications" eg:"[]"`
	NextCursor   string                         `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

// ManagementCreditApplicationDetailReq requests one application after exact authorization.
type ManagementCreditApplicationDetailReq struct {
	g.Meta        `path:"/admin/billing/credit/applications/{applicationNo}" method:"get" tags:"IDC Credit Management" summary:"Get credit application detail" dc:"Return one application and its immutable review summaries after exact detail permission." permission:"idc:admin:billing:credit-application:query"`
	ApplicationNo string `json:"applicationNo" in:"path" v:"required|max-length:40" dc:"Stable credit application number" eg:"CAP_01J2ABCDEF"`
}

// ManagementCreditApplicationDetailRes returns one authorized application aggregate.
type ManagementCreditApplicationDetailRes struct {
	Detail   *ManagementCreditApplicationDetail `json:"detail" dc:"Authorized credit application aggregate" eg:"{}"`
	Evidence []*ManagementCreditEvidence        `json:"evidence" dc:"At most ten private evidence file summaries" eg:"[]"`
}

type ManagementCreditEvidence struct {
	UploadNo  string `json:"uploadNo"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
}
type ManagementCreditEvidenceOpenReq struct {
	g.Meta        `path:"/admin/billing/credit/applications/{applicationNo}/evidence/{uploadNo}" method:"get" tags:"IDC Credit Management" summary:"Open authorized private credit evidence" permission:"idc:admin:billing:credit-application:query"`
	ApplicationNo string `json:"applicationNo" in:"path" v:"required|max-length:40"`
	UploadNo      string `json:"uploadNo" in:"path" v:"required|max-length:40"`
}
type ManagementCreditEvidenceOpenRes struct{}

// ManagementCreditApplicationReviewIntent is the exact review preview shared by proof and execution.
type ManagementCreditApplicationReviewIntent struct {
	RequestNo      string `json:"requestNo" v:"required|min-length:8|max-length:128" dc:"Stable request number shared by proof issuance and execution" eg:"credit-review-01J2"`
	DecisionCode   string `json:"decisionCode" v:"required|in:approved,rejected" dc:"Independent reviewer decision: approved or rejected" eg:"approved"`
	CommentDigest  string `json:"commentDigest" v:"required|regex:^[0-9a-f]{64}$" dc:"Lowercase SHA-256 digest of immutable review comment" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	EvidenceDigest string `json:"evidenceDigest" v:"required|regex:^[0-9a-f]{64}$" dc:"Lowercase SHA-256 digest of immutable independent review evidence" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}

// ManagementCreditApplicationReviewProofReq requests a proof bound to one review decision.
type ManagementCreditApplicationReviewProofReq struct {
	g.Meta        `path:"/admin/billing/credit/applications/{applicationNo}/review-proofs" method:"post" tags:"IDC Credit Management" summary:"Issue a credit application review proof" dc:"Authorize the exact independent decision and issue a short-lived proof only after a recent full login." permission:"idc:admin:billing:credit-application:review"`
	ApplicationNo string `json:"applicationNo" in:"path" v:"required|max-length:40" dc:"Stable pending credit application number" eg:"CAP_01J2ABCDEF"`
	ManagementCreditApplicationReviewIntent
}

// ManagementCreditApplicationReviewProofRes returns one short-lived review proof.
type ManagementCreditApplicationReviewProofRes struct {
	Proof *FinancialManagementActionProof `json:"proof" dc:"Short-lived single-use review proof" eg:"{}"`
}

// ManagementCreditApplicationReviewReq consumes an exact proof for one independent review.
type ManagementCreditApplicationReviewReq struct {
	g.Meta        `path:"/admin/billing/credit/applications/{applicationNo}/reviews" method:"post" tags:"IDC Credit Management" summary:"Review a credit application" dc:"Re-authorize and consume the exact-preview proof before appending an independent decision; account activation remains atomic." permission:"idc:admin:billing:credit-application:review"`
	ApplicationNo string `json:"applicationNo" in:"path" v:"required|max-length:40" dc:"Stable pending credit application number" eg:"CAP_01J2ABCDEF"`
	ManagementCreditApplicationReviewIntent
	ProofToken string `json:"proofToken" v:"required|max-length:4096" dc:"Opaque proof issued for this exact review preview" eg:"opaque-proof"`
}

// ManagementCreditApplicationReviewRes returns the committed application lifecycle result.
type ManagementCreditApplicationReviewRes struct {
	Application *ManagementCreditApplication `json:"application" dc:"Committed credit application review result" eg:"{}"`
}
