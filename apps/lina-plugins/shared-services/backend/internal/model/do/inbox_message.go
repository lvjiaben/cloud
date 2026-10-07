// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// InboxMessage is the golang structure of table plugin_shared_services_inbox_message for DAO operations like Where/Data.
type InboxMessage struct {
	g.Meta          `orm:"table:plugin_shared_services_inbox_message, do:true"`
	Id              any        //
	TenantId        any        //
	MessageNo       any        //
	RecipientUserId any        //
	CallerPluginId  any        //
	SourceType      any        //
	SourceNo        any        //
	IdempotencyKey  any        //
	ContentDigest   any        //
	Title           any        //
	Body            any        //
	Category        any        //
	ActionUrl       any        //
	Status          any        //
	ReadAt          any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
