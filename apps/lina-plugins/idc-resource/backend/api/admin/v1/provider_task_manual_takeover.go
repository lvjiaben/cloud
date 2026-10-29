// This file defines evidence-backed Provider task manual takeover.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ProviderTaskManualTakeoverReq struct {
	ServiceNo       string `json:"serviceNo" v:"max-length:40" dc:"Optional exact product scope; a task outside this product is rejected" eg:"SVC_0123456789abcdef"`
	ChannelNo       string `json:"channelNo" v:"max-length:64" dc:"Optional exact interface scope; a task outside this interface is rejected" eg:"channel_primary"`
	g.Meta          `path:"/admin/resource/tasks/{taskNo}/manual-takeover" method:"post" tags:"IDC Resource Tasks" summary:"Take over a Provider task" dc:"Close automatic reconciliation for an eligible task and atomically append an evidence-backed staff assignment plus business audit. This endpoint performs no Provider call, does not decide success or failure, and cannot retry or force a task result." permission:"idc:admin:resource:task:manual-takeover"`
	TaskNo          string   `json:"taskNo" in:"path" v:"required|max-length:64" dc:"Exact opaque local Provider task number" eg:"task_0123456789abcdef"`
	ExpectedVersion int64    `json:"expectedVersion" v:"required|min:1" dc:"Task version last read by the staff user" eg:"3"`
	ReasonCode      string   `json:"reasonCode" v:"required|max-length:64" dc:"One constrained reason: provider_result_unresolved, provider_task_stalled, resource_state_conflict, external_coordination_required, or compensation_required" eg:"provider_result_unresolved"`
	EvidenceNos     []string `json:"evidenceNos" v:"required" dc:"One through sixteen immutable evidence numbers already returned for this exact task" eg:"[\"evidence_0123456789abcdef0123456789abcdef\"]"`
}

type ProviderTaskManualTakeoverRes struct {
	Item *ProviderTaskDetail `json:"item" dc:"Committed task detail in the closed manual_takeover state" eg:"{}"`
}
