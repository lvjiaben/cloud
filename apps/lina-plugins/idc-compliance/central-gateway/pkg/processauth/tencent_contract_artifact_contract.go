// This file defines signed local-process routes for contract file archival.
package processauth

const (
	TencentContractFinalPDFsPath      = "/internal/v1/tencent-essbasic/contract-final-pdfs"
	TencentContractFinalPDFReadScope  = "tencent-essbasic.contract-final-pdf.read"
	TencentContractSignReportsPath    = "/internal/v1/tencent-essbasic/contract-sign-reports"
	TencentContractSignReportScope    = "tencent-essbasic.contract-sign-report.ensure"
	TencentArtifactStateHeader        = "X-Lina-Artifact-State"
	TencentArtifactKindHeader         = "X-Lina-Artifact-Kind"
	TencentArtifactSHA256Header       = "X-Lina-Artifact-SHA256"
	TencentArtifactObservedUnixHeader = "X-Lina-Supplier-Observed-Unix"
	TencentArtifactStateReady         = "ready"
	TencentArtifactStatePending       = "pending"
	TencentArtifactStateSkipped       = "skipped"
	TencentArtifactStateResultUnknown = "result_unknown"
	TencentArtifactKindFinalPDF       = "final_pdf"
	TencentArtifactKindSignReport     = "sign_report"
)

// TencentContractArtifactRequest identifies one protected completed flow.
type TencentContractArtifactRequest struct {
	RouteNo string `json:"routeNo"`
}

// TencentContractArtifactPendingResponse contains no provider URL or identifier.
type TencentContractArtifactPendingResponse struct {
	State         string `json:"state"`
	Code          string `json:"code"`
	NextCheckUnix int64  `json:"nextCheckUnix,omitempty"`
}
