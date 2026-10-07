// This file defines the management command that submits a reviewed contract reissue.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractReissueCreateReq separately submits one reviewed reissue preview.
type ElectronicContractReissueCreateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contracts/{contractNo}/reissues" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Reissue an electronic contract" dc:"Create one fresh seven-day Tencent signing flow for an unchanged contract or a separately reviewed replacement preview." permission:"idc:admin:compliance:contract:reissue"`
	ContractNo      string `json:"contractNo" in:"path" v:"required|max-length:64" dc:"Exact unchanged contract or reviewed replacement contract" eg:"CTR_0123456789ABCDEF"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Contract version shown with the reviewed preview" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|length:16,128" dc:"Retry key bound to this exact reissue submission" eg:"contract-reissue-20260817-0001"`
}

// ElectronicContractReissueCreateRes returns no Tencent identifier.
type ElectronicContractReissueCreateRes struct {
	Item *ElectronicContractSigningSubmissionItem `json:"item" dc:"Safe installation-local reissue result" eg:"{}"`
}
