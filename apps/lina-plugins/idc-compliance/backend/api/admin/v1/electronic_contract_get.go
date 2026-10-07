// This file defines bounded contract lifecycle and file-evidence detail.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractGetReq reads one exact contract after data-scope authorization.
type ElectronicContractGetReq struct {
	g.Meta     `path:"/admin/compliance/electronic-contracts/{contractNo}" method:"get" tags:"IDC Compliance Electronic Contract" summary:"Get electronic contract evidence" dc:"Return one bounded secret-safe contract detail with local flows, append-only events, immutable file metadata, artifact collection states, and replacement relations. Private object references, supplier identifiers, signing links, and contract bodies are never returned." permission:"idc:admin:compliance:contract:query"`
	ContractNo string `json:"contractNo" in:"path" v:"required|max-length:64" dc:"Exact installation-local opaque contract number inside the caller's evaluated data scope" eg:"CTR_0123456789ABCDEF"`
}

// ElectronicContractGetRes returns one contract and bounded evidence groups.
type ElectronicContractGetRes struct {
	Item             ElectronicContractListItem          `json:"item" dc:"Contract summary and current lifecycle state" eg:"{}"`
	Flows            []ElectronicContractFlowItem        `json:"flows" dc:"At most 50 signing, reissue, or termination flows in newest-first order" eg:"[]"`
	Events           []ElectronicContractEventItem       `json:"events" dc:"At most 200 append-only supplier-authoritative timeline items in newest-first order" eg:"[]"`
	Files            []ElectronicContractFileItem        `json:"files" dc:"At most 50 immutable local artifact metadata rows without private storage references" eg:"[]"`
	ArchiveJobs      []ElectronicContractArchiveJobItem  `json:"archiveJobs" dc:"At most 50 managed artifact collection states" eg:"[]"`
	Replacements     []ElectronicContractReplacementItem `json:"replacements" dc:"At most 20 immutable old-to-new contract relations" eg:"[]"`
	HistoryTruncated bool                                `json:"historyTruncated" dc:"Whether older bounded history exists and is omitted from this interactive detail" eg:"false"`
}

// ElectronicContractFlowItem is one local provider operation without external identifiers.
type ElectronicContractFlowItem struct {
	FlowNo         string `json:"flowNo" dc:"Installation-local opaque flow number" eg:"CFW_0123456789ABCDEF0123456789ABCDEF"`
	SequenceNo     int    `json:"sequenceNo" dc:"Monotonic flow sequence inside the contract" eg:"2"`
	PurposeCode    string `json:"purposeCode" dc:"Flow purpose: initial, reissue, or termination" eg:"reissue"`
	StatusCode     string `json:"statusCode" dc:"Local guarded flow state" eg:"signing"`
	ProviderStatus string `json:"providerStatus" dc:"Safe supplier status code when observed; empty before observation" eg:"PART"`
	FailureCode    string `json:"failureCode" dc:"Stable safe failure reason; empty without a known failure" eg:""`
	ExpiresAt      int64  `json:"expiresAt" dc:"Signing deadline as Unix timestamp in milliseconds; zero when not applicable" eg:"1787644800000"`
	SubmittedAt    int64  `json:"submittedAt" dc:"Supplier submission time as Unix timestamp in milliseconds; zero before submission" eg:"1787040000000"`
	CompletedAt    int64  `json:"completedAt" dc:"Flow completion time as Unix timestamp in milliseconds; zero while open" eg:"1787043600000"`
	CreatedAt      int64  `json:"createdAt" dc:"Local flow creation time as Unix timestamp in milliseconds" eg:"1787040000000"`
}

// ElectronicContractEventItem is one append-only lifecycle observation.
type ElectronicContractEventItem struct {
	FlowNo             string `json:"flowNo" dc:"Installation-local flow number" eg:"CFW_0123456789ABCDEF0123456789ABCDEF"`
	SourceKind         string `json:"sourceKind" dc:"Observation source such as callback or active_query" eg:"callback"`
	ProviderStatus     string `json:"providerStatus" dc:"Exact validated supplier status code" eg:"ALL"`
	ProjectedStatus    string `json:"projectedStatus" dc:"Monotonic local contract state produced by this event" eg:"completed"`
	DecisionCode       string `json:"decisionCode" dc:"Stable convergence decision code" eg:"advanced"`
	ReasonCode         string `json:"reasonCode" dc:"Stable safe event reason" eg:"supplier_completed"`
	SupplierObservedAt int64  `json:"supplierObservedAt" dc:"Supplier-authoritative observation time as Unix timestamp in milliseconds" eg:"1787043600000"`
	ObservedAt         int64  `json:"observedAt" dc:"Authenticated gateway observation time as Unix timestamp in milliseconds" eg:"1787043600000"`
	CreatedAt          int64  `json:"createdAt" dc:"Append-only local event creation time as Unix timestamp in milliseconds" eg:"1787043600000"`
}

// ElectronicContractFileItem is immutable artifact metadata without its storage handle.
type ElectronicContractFileItem struct {
	FileNo             string `json:"fileNo" dc:"Installation-local opaque file number" eg:"CFI_0123456789ABCDEF"`
	FlowNo             string `json:"flowNo" dc:"Owning installation-local flow number; empty for contract-level evidence" eg:"CFW_0123456789ABCDEF0123456789ABCDEF"`
	FileTypeCode       string `json:"fileTypeCode" dc:"Artifact type such as final_pdf, sign_report, evidence_certificate, event_timeline, termination_pdf, or blockchain_metadata" eg:"final_pdf"`
	ContentSha256      string `json:"contentSha256" dc:"Lowercase SHA-256 digest used to verify immutable content" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	MediaType          string `json:"mediaType" dc:"Validated file media type" eg:"application/pdf"`
	ByteSize           int64  `json:"byteSize" dc:"Stored artifact byte size" eg:"245760"`
	StatusCode         string `json:"statusCode" dc:"Local file state: pending, archived, failed, or legal_hold" eg:"archived"`
	Immutable          bool   `json:"immutable" dc:"Whether database rules reject content and ownership mutation" eg:"true"`
	ArchivedAt         int64  `json:"archivedAt" dc:"Successful local archive time as Unix timestamp in milliseconds; zero until archived" eg:"1787043600000"`
	RetentionUntil     int64  `json:"retentionUntil" dc:"Retention deadline as Unix timestamp in milliseconds; zero without a scheduled deadline" eg:"1944810000000"`
	LegalHold          bool   `json:"legalHold" dc:"Whether normal retention disposal is suspended" eg:"false"`
	SupplierObservedAt int64  `json:"supplierObservedAt" dc:"Supplier-authoritative artifact time as Unix timestamp in milliseconds; zero when not supplied" eg:"1787043500000"`
	CreatedAt          int64  `json:"createdAt" dc:"Local metadata creation time as Unix timestamp in milliseconds" eg:"1787043600000"`
}

// ElectronicContractArchiveJobItem is one artifact collection state without leases or routes.
type ElectronicContractArchiveJobItem struct {
	JobNo              string `json:"jobNo" dc:"Installation-local opaque archive job number" eg:"caj_0123456789abcdef0123456789abcdef"`
	FlowNo             string `json:"flowNo" dc:"Owning installation-local flow number" eg:"CFW_0123456789ABCDEF0123456789ABCDEF"`
	ArtifactTypeCode   string `json:"artifactTypeCode" dc:"Requested artifact: final_pdf, sign_report, termination_pdf, or blockchain_metadata" eg:"sign_report"`
	StatusCode         string `json:"statusCode" dc:"Job state: queued, processing, pending, archived, skipped, or result_unknown" eg:"skipped"`
	AttemptCount       int    `json:"attemptCount" dc:"Bounded number of managed collection attempts" eg:"1"`
	LastReasonCode     string `json:"lastReasonCode" dc:"Stable reason for the current state, including closed capability reasons" eg:"report_capability_unavailable"`
	SupplierObservedAt int64  `json:"supplierObservedAt" dc:"Supplier-authoritative artifact time as Unix timestamp in milliseconds; zero when unavailable" eg:"1787043500000"`
	CompletedAt        int64  `json:"completedAt" dc:"Terminal job time as Unix timestamp in milliseconds; zero while open" eg:"1787043600000"`
	NextAttemptAt      int64  `json:"nextAttemptAt" dc:"Next managed retry time as Unix timestamp in milliseconds; zero when none is scheduled" eg:"1787047200000"`
	CreatedAt          int64  `json:"createdAt" dc:"Job creation time as Unix timestamp in milliseconds" eg:"1787040000000"`
	UpdatedAt          int64  `json:"updatedAt" dc:"Job update time as Unix timestamp in milliseconds" eg:"1787043600000"`
}

// ElectronicContractReplacementItem is one immutable contract-version relation.
type ElectronicContractReplacementItem struct {
	ReplacementNo string `json:"replacementNo" dc:"Installation-local opaque replacement relation number" eg:"CRP_0123456789ABCDEF"`
	OldContractNo string `json:"oldContractNo" dc:"Preserved replaced contract number" eg:"CTR_0123456789ABCDEF"`
	NewContractNo string `json:"newContractNo" dc:"New immutable replacement contract number" eg:"CTR_1123456789ABCDEF"`
	CreatedAt     int64  `json:"createdAt" dc:"Relation creation time as Unix timestamp in milliseconds" eg:"1787040000000"`
}
