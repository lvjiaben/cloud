// This file defines one bounded Provider task evidence projection.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ProviderTaskDetailReq struct {
	ServiceNo string `json:"serviceNo" in:"query" v:"max-length:40" dc:"Optional exact product scope; a task outside this product is rejected" eg:"SVC_0123456789abcdef"`
	ChannelNo string `json:"channelNo" in:"query" v:"max-length:64" dc:"Optional exact interface scope; a task outside this interface is rejected" eg:"channel_primary"`
	g.Meta    `path:"/admin/resource/tasks/{taskNo}" method:"get" tags:"IDC Resource Tasks" summary:"Query Provider task evidence" dc:"Return one local task, latest normalized upstream binding, at most 50 immutable redacted evidence records, and any immutable takeover assignment. This endpoint never calls a Provider or exposes write parameters, credentials, secret bindings, or raw payloads." permission:"idc:admin:resource:task:query"`
	TaskNo    string `json:"taskNo" in:"path" v:"required|max-length:64" dc:"Exact opaque local Provider task number" eg:"task_0123456789abcdef"`
}

type ProviderTaskDetailRes struct {
	Item *ProviderTaskDetail `json:"item" dc:"Bounded non-secret task reconciliation detail" eg:"{}"`
}

type ProviderTaskUpstream struct {
	ExternalTaskRef     string `json:"externalTaskRef" dc:"Bounded normalized upstream task reference" eg:"900100"`
	ExternalResourceRef string `json:"externalResourceRef" dc:"Bounded normalized upstream resource reference" eg:"600100"`
	StatusCode          string `json:"statusCode" dc:"Latest normalized upstream task status" eg:"running"`
	Progress            int    `json:"progress" dc:"Latest normalized progress from 0 through 100" eg:"40"`
	ObservedAt          int64  `json:"observedAt" dc:"Latest database-recorded upstream observation Unix timestamp in milliseconds" eg:"1786464000000"`
}

type ProviderTaskEvidence struct {
	EvidenceNo   string `json:"evidenceNo" dc:"Opaque immutable evidence number selectable for takeover" eg:"evidence_0123456789abcdef0123456789abcdef"`
	SourceKind   string `json:"sourceKind" dc:"Stable owner evidence source kind" eg:"provider_task"`
	EvidenceKind string `json:"evidenceKind" dc:"Stable evidence kind" eg:"task_observation"`
	Reference    string `json:"reference" dc:"Bounded normalized task or resource reference; never a secret" eg:"900100"`
	Summary      string `json:"summary" dc:"Credential-scanned bounded evidence summary; never a raw response" eg:"task.running"`
	ObservedAt   int64  `json:"observedAt" dc:"Evidence observation Unix timestamp in milliseconds" eg:"1786464000000"`
}

type ProviderTaskResidualResource struct {
	ResourceKind string `json:"resourceKind" dc:"Normalized residual resource type; never a Provider payload" eg:"volume"`
	ResourceRef  string `json:"resourceRef" dc:"Opaque bounded residual resource number" eg:"volume_600100"`
	Status       string `json:"status" dc:"Safe reclamation status: present, release_pending, release_failed, or unknown" eg:"release_failed"`
	ReasonCode   string `json:"reasonCode" dc:"Stable normalized reclamation reason code" eg:"provider_rejected"`
}

type ProviderTaskDetail struct {
	Task              *ProviderTaskItem               `json:"task" dc:"Current local task projection" eg:"{}"`
	Upstream          *ProviderTaskUpstream           `json:"upstream" dc:"Latest normalized upstream task binding; null when unavailable" eg:"{}"`
	Evidence          []*ProviderTaskEvidence         `json:"evidence" dc:"At most 50 newest immutable redacted evidence records" eg:"[]"`
	ResidualResources []*ProviderTaskResidualResource `json:"residualResources" dc:"At most 16 normalized non-secret resources still requiring reclamation" eg:"[]"`
}
