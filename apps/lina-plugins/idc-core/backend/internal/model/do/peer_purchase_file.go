// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PeerPurchaseFile is the golang structure of table plugin_idc_core_peer_purchase_file for DAO operations like Where/Data.
type PeerPurchaseFile struct {
	g.Meta            `orm:"table:plugin_idc_core_peer_purchase_file, do:true"`
	Id                any        //
	TenantId          any        //
	IntentNo          any        //
	FieldKey          any        //
	FileSequence      any        //
	Sha256Hex         any        //
	UpstreamFileToken any        //
	CreatedAt         *time.Time //
}
