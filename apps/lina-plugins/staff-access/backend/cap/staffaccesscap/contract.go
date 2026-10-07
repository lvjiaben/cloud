// Package staffaccesscap defines reusable target-staff business eligibility.
package staffaccesscap

import "context"

const (
	OwnerPluginID         = "staff-access"
	ServiceName           = "staff-access"
	SourceContractPackage = "lina-plugin-staff-access/backend/cap/staffaccesscap"
	ContractVersion       = "v1"
	MaximumBatchSize      = 100
)

const (
	ScopeGlobal     = "global"
	ScopeDepartment = "department"
)

type CheckRequest struct {
	UserID         int64  `json:"userId"`
	CapabilityCode string `json:"capabilityCode"`
	ScopeType      string `json:"scopeType"`
	ScopeKey       string `json:"scopeKey,omitempty"`
}

type BatchCheckRequest struct {
	UserIDs        []int64 `json:"userIds"`
	CapabilityCode string  `json:"capabilityCode"`
	ScopeType      string  `json:"scopeType"`
	ScopeKey       string  `json:"scopeKey,omitempty"`
}

type Decision struct {
	UserID   int64  `json:"userId"`
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason"`
}

type Service interface {
	Check(context.Context, CheckRequest) (*Decision, error)
	BatchCheck(context.Context, BatchCheckRequest) ([]Decision, error)
}
