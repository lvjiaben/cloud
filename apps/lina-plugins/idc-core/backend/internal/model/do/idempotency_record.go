// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// IdempotencyRecord is the golang structure of table plugin_idc_core_idempotency_record for DAO operations like Where/Data.
type IdempotencyRecord struct {
	g.Meta               `orm:"table:plugin_idc_core_idempotency_record, do:true"`
	Id                   any        //
	TenantId             any        //
	Scope                any        // Stable command or endpoint idempotency namespace
	CallerScopeDigest    any        // Digest of the normalized customer, staff, channel, or system caller scope
	IdempotencyKeyDigest any        // SHA-256 digest of the caller idempotency key
	RequestFingerprint   any        // SHA-256 digest of canonical request semantics
	StatusCode           any        //
	LockOwner            any        //
	LockExpiresAt        *time.Time //
	ResourceType         any        //
	ResourceNo           any        //
	ResponseStatusCode   any        //
	ResponseSnapshot     any        // Redacted normalized response used for safe replay
	ExpiresAt            *time.Time // Earliest time at which retention cleanup may evaluate the record
	CompletedAt          *time.Time //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
