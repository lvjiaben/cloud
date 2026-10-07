package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractSigningSubmissionCreateReq explicitly submits one frozen preview.
type ElectronicContractSigningSubmissionCreateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contracts/{contractNo}/signing-submissions" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Submit an electronic contract for signing" dc:"Create or safely recover one Tencent signing flow only after the immutable preview has been created and separately reviewed." permission:"idc:admin:compliance:contract:create"`
	ContractNo      string `json:"contractNo" in:"path" v:"required|max-length:64" dc:"Exact immutable contract number" eg:"CTR_0123456789ABCDEF"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Contract version shown with the immutable preview" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|length:16,128" dc:"Retry key bound to this exact signing submission" eg:"contract-submit-20260816-0001"`
}

// ElectronicContractSigningSubmissionCreateRes returns no Tencent identifier.
type ElectronicContractSigningSubmissionCreateRes struct {
	Item *ElectronicContractSigningSubmissionItem `json:"item" dc:"Safe installation-local signing submission result" eg:"{}"`
}

// ElectronicContractSigningSubmissionItem is safe for the management UI.
type ElectronicContractSigningSubmissionItem struct {
	ContractNo string `json:"contractNo" dc:"Immutable contract number" eg:"CTR_0123456789ABCDEF"`
	FlowNo     string `json:"flowNo" dc:"Installation-local contract flow number" eg:"CFW_0123456789ABCDEF0123456789ABCDEF"`
	StatusCode string `json:"statusCode" dc:"signing or result_unknown" eg:"signing"`
	Version    int64  `json:"version" dc:"Current contract version" eg:"2"`
	Reused     bool   `json:"reused" dc:"Whether a prior identical submission was safely recovered" eg:"false"`
}
