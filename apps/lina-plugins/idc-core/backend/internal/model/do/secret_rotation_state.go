// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SecretRotationState is the golang structure of table plugin_idc_core_secret_rotation_state for DAO operations like Where/Data.
type SecretRotationState struct {
	g.Meta                 `orm:"table:plugin_idc_core_secret_rotation_state, do:true"`
	Id                     any        //
	TenantId               any        //
	OwnerPluginId          any        //
	CallerScope            any        //
	Epoch                  any        // Monotonic active-key verification generation
	ActiveKeyId            any        // Active key identifier certified by the current generation
	StatusCode             any        //
	WriteFence             any        // Monotonic payload mutation fence shared by all service instances
	VerificationCursorId   any        // Last immutable payload row authenticated in the current generation
	VerificationWriteFence any        // Write fence captured when the current full verification pass began
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
}
