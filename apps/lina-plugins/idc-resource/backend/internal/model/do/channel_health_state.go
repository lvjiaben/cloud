// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ChannelHealthState is the golang structure of table plugin_idc_resource_channel_health_state for DAO operations like Where/Data.
type ChannelHealthState struct {
	g.Meta                          `orm:"table:plugin_idc_resource_channel_health_state, do:true"`
	Id                              any        //
	TenantId                        any        //
	ChannelNo                       any        //
	SampleCount                     any        //
	FailureCount                    any        //
	UnknownCount                    any        //
	ConsecutiveFailures             any        //
	CircuitOpen                     any        //
	WriteProtectedUntil             *time.Time // Database-clock protection deadline; read-only task and resource reconciliation remains allowed
	WindowStartedAt                 *time.Time //
	LastSummaryCode                 any        // Stable non-sensitive machine code; raw upstream errors are forbidden
	LastObservedAt                  *time.Time //
	Version                         any        //
	CreatedAt                       *time.Time //
	UpdatedAt                       *time.Time //
	WindowSampleLimit               any        // Merchant policy: bounded write sample window from 5 through 100
	MinimumSamplesToProtect         any        // Merchant policy: minimum write observations before protection may open
	FailureLimit                    any        //
	UnknownResultLimit              any        //
	ConsecutiveFailureLimit         any        //
	ProtectionDurationSeconds       any        // Merchant policy: automatic write protection duration from 60 through 86400 seconds
	PolicyVersion                   any        //
	PolicyUpdatedBy                 any        //
	PolicyUpdatedAt                 *time.Time // PostgreSQL-clock time of the latest audited merchant policy change
	LastAuthenticationHealthCode    any        //
	LastAuthenticationSummaryCode   any        // Stable non-sensitive latest authentication summary; raw upstream errors are forbidden
	LastAuthenticationLatencyMs     any        //
	LastAuthenticationObservedAt    *time.Time //
	LastQueryHealthCode             any        //
	LastQuerySummaryCode            any        // Stable non-sensitive latest query summary; raw upstream errors are forbidden
	LastQueryLatencyMs              any        //
	LastQueryObservedAt             *time.Time //
	LastWriteHealthCode             any        //
	LastWriteSummaryCode            any        // Stable non-sensitive latest write summary; raw upstream errors are forbidden
	LastWriteLatencyMs              any        //
	LastWriteObservedAt             *time.Time //
	LastProviderCapacityHealthCode  any        //
	LastProviderCapacitySummaryCode any        // Stable non-sensitive latest provider_capacity summary; raw upstream errors are forbidden
	LastProviderCapacityLatencyMs   any        //
	LastProviderCapacityObservedAt  *time.Time //
}
