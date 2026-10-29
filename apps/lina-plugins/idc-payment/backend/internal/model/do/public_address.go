// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PublicAddress is the golang structure of table plugin_idc_payment_public_address for DAO operations like Where/Data.
type PublicAddress struct {
	g.Meta               `orm:"table:plugin_idc_payment_public_address, do:true"`
	TenantId             any        //
	NotificationOrigin   any        //
	CustomerReturnOrigin any        //
	Version              any        //
	UpdatedBy            any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
