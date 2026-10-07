// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrincipalTask is the golang structure of table plugin_idc_resource_principal_task for DAO operations like Where/Data.
type PrincipalTask struct {
	g.Meta                       `orm:"table:plugin_idc_resource_principal_task, do:true"`
	Id                           any        //
	TenantId                     any        //
	TaskNo                       any        //
	CustomerNo                   any        //
	ChannelNo                    any        //
	UsernameSecretRef            any        // Channel-scoped secretcap handle; never a username or customer profile field
	PasswordSecretRef            any        // Channel-scoped secretcap handle; plaintext is never persisted in the resource schema
	StatusCode                   any        //
	ExpectedExternalPrincipalRef any        //
	ResultUnknown                any        // Hard replay fence: when true, only exact lookup or manual review is allowed
	AttemptCount                 any        // Count of write-eligible claims; unknown writes are never made claimable again
	ObservationAttemptCount      any        // Read-only exact lookup attempts; never authorizes another create
	AvailableAt                  *time.Time //
	LeaseTokenDigest             any        //
	LeaseExpiresAt               *time.Time //
	FailureCode                  any        //
	EvidenceDigest               any        //
	FinishedAt                   *time.Time // Confirmed mappings revoke both handles; manual review revokes the password and retains only its encrypted username handle for operator reconciliation
	Version                      any        //
	CreatedAt                    *time.Time //
	UpdatedAt                    *time.Time //
}
