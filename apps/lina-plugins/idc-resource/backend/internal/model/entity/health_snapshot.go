// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// HealthSnapshot is the golang structure for table health_snapshot.
type HealthSnapshot struct {
	Id                         int64      `json:"id"                         orm:"id"                            description:""`
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                     description:""`
	SnapshotNo                 string     `json:"snapshotNo"                 orm:"snapshot_no"                   description:""`
	ChannelNo                  string     `json:"channelNo"                  orm:"channel_no"                    description:""`
	HealthCode                 string     `json:"healthCode"                 orm:"health_code"                   description:""`
	LatencyMs                  int        `json:"latencyMs"                  orm:"latency_ms"                    description:""`
	ConsecutiveFailures        int        `json:"consecutiveFailures"        orm:"consecutive_failures"          description:""`
	CircuitOpen                bool       `json:"circuitOpen"                orm:"circuit_open"                  description:""`
	CapabilityObservedAt       *time.Time `json:"capabilityObservedAt"       orm:"capability_observed_at"        description:""`
	ProviderCapacityObservedAt *time.Time `json:"providerCapacityObservedAt" orm:"provider_capacity_observed_at" description:""`
	SummaryCode                string     `json:"summaryCode"                orm:"summary_code"                  description:"Stable non-sensitive machine code; raw upstream errors are forbidden"`
	ObservedAt                 *time.Time `json:"observedAt"                 orm:"observed_at"                   description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                    description:""`
	OperationKind              string     `json:"operationKind"              orm:"operation_kind"                description:""`
}
