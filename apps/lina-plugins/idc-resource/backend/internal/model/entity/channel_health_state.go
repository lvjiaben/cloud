// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ChannelHealthState is the golang structure for table channel_health_state.
type ChannelHealthState struct {
	Id                              int64      `json:"id"                              orm:"id"                                  description:""`
	TenantId                        int        `json:"tenantId"                        orm:"tenant_id"                           description:""`
	ChannelNo                       string     `json:"channelNo"                       orm:"channel_no"                          description:""`
	SampleCount                     int        `json:"sampleCount"                     orm:"sample_count"                        description:""`
	FailureCount                    int        `json:"failureCount"                    orm:"failure_count"                       description:""`
	UnknownCount                    int        `json:"unknownCount"                    orm:"unknown_count"                       description:""`
	ConsecutiveFailures             int        `json:"consecutiveFailures"             orm:"consecutive_failures"                description:""`
	CircuitOpen                     bool       `json:"circuitOpen"                     orm:"circuit_open"                        description:""`
	WriteProtectedUntil             *time.Time `json:"writeProtectedUntil"             orm:"write_protected_until"               description:"Database-clock protection deadline; read-only task and resource reconciliation remains allowed"`
	WindowStartedAt                 *time.Time `json:"windowStartedAt"                 orm:"window_started_at"                   description:""`
	LastSummaryCode                 string     `json:"lastSummaryCode"                 orm:"last_summary_code"                   description:"Stable non-sensitive machine code; raw upstream errors are forbidden"`
	LastObservedAt                  *time.Time `json:"lastObservedAt"                  orm:"last_observed_at"                    description:""`
	Version                         int64      `json:"version"                         orm:"version"                             description:""`
	CreatedAt                       *time.Time `json:"createdAt"                       orm:"created_at"                          description:""`
	UpdatedAt                       *time.Time `json:"updatedAt"                       orm:"updated_at"                          description:""`
	WindowSampleLimit               int        `json:"windowSampleLimit"               orm:"window_sample_limit"                 description:"Merchant policy: bounded write sample window from 5 through 100"`
	MinimumSamplesToProtect         int        `json:"minimumSamplesToProtect"         orm:"minimum_samples_to_protect"          description:"Merchant policy: minimum write observations before protection may open"`
	FailureLimit                    int        `json:"failureLimit"                    orm:"failure_limit"                       description:""`
	UnknownResultLimit              int        `json:"unknownResultLimit"              orm:"unknown_result_limit"                description:""`
	ConsecutiveFailureLimit         int        `json:"consecutiveFailureLimit"         orm:"consecutive_failure_limit"           description:""`
	ProtectionDurationSeconds       int        `json:"protectionDurationSeconds"       orm:"protection_duration_seconds"         description:"Merchant policy: automatic write protection duration from 60 through 86400 seconds"`
	PolicyVersion                   int64      `json:"policyVersion"                   orm:"policy_version"                      description:""`
	PolicyUpdatedBy                 int        `json:"policyUpdatedBy"                 orm:"policy_updated_by"                   description:""`
	PolicyUpdatedAt                 *time.Time `json:"policyUpdatedAt"                 orm:"policy_updated_at"                   description:"PostgreSQL-clock time of the latest audited merchant policy change"`
	LastAuthenticationHealthCode    string     `json:"lastAuthenticationHealthCode"    orm:"last_authentication_health_code"     description:""`
	LastAuthenticationSummaryCode   string     `json:"lastAuthenticationSummaryCode"   orm:"last_authentication_summary_code"    description:"Stable non-sensitive latest authentication summary; raw upstream errors are forbidden"`
	LastAuthenticationLatencyMs     int        `json:"lastAuthenticationLatencyMs"     orm:"last_authentication_latency_ms"      description:""`
	LastAuthenticationObservedAt    *time.Time `json:"lastAuthenticationObservedAt"    orm:"last_authentication_observed_at"     description:""`
	LastQueryHealthCode             string     `json:"lastQueryHealthCode"             orm:"last_query_health_code"              description:""`
	LastQuerySummaryCode            string     `json:"lastQuerySummaryCode"            orm:"last_query_summary_code"             description:"Stable non-sensitive latest query summary; raw upstream errors are forbidden"`
	LastQueryLatencyMs              int        `json:"lastQueryLatencyMs"              orm:"last_query_latency_ms"               description:""`
	LastQueryObservedAt             *time.Time `json:"lastQueryObservedAt"             orm:"last_query_observed_at"              description:""`
	LastWriteHealthCode             string     `json:"lastWriteHealthCode"             orm:"last_write_health_code"              description:""`
	LastWriteSummaryCode            string     `json:"lastWriteSummaryCode"            orm:"last_write_summary_code"             description:"Stable non-sensitive latest write summary; raw upstream errors are forbidden"`
	LastWriteLatencyMs              int        `json:"lastWriteLatencyMs"              orm:"last_write_latency_ms"               description:""`
	LastWriteObservedAt             *time.Time `json:"lastWriteObservedAt"             orm:"last_write_observed_at"              description:""`
	LastProviderCapacityHealthCode  string     `json:"lastProviderCapacityHealthCode"  orm:"last_provider_capacity_health_code"  description:""`
	LastProviderCapacitySummaryCode string     `json:"lastProviderCapacitySummaryCode" orm:"last_provider_capacity_summary_code" description:"Stable non-sensitive latest provider_capacity summary; raw upstream errors are forbidden"`
	LastProviderCapacityLatencyMs   int        `json:"lastProviderCapacityLatencyMs"   orm:"last_provider_capacity_latency_ms"   description:""`
	LastProviderCapacityObservedAt  *time.Time `json:"lastProviderCapacityObservedAt"  orm:"last_provider_capacity_observed_at"  description:""`
}
