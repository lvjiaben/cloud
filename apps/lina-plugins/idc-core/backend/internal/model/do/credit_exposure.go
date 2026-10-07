// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditExposure is the golang structure of table plugin_idc_core_credit_exposure for DAO operations like Where/Data.
type CreditExposure struct {
	g.Meta                   `orm:"table:plugin_idc_core_credit_exposure, do:true"`
	Id                       any        //
	TenantId                 any        //
	ExposureNo               any        //
	CreditAccountNo          any        //
	CustomerNo               any        //
	CurrencyCode             any        //
	SourceTypeCode           any        //
	SourceReference          any        //
	OrderNo                  *string    //
	InvoiceNo                *string    //
	OriginalPrincipalMinor   any        //
	UnbilledPrincipalMinor   any        //
	BilledPrincipalMinor     any        //
	RepaidPrincipalMinor     any        //
	ReleasedPrincipalMinor   any        //
	WrittenOffPrincipalMinor any        //
	StatusCode               any        //
	SourceSnapshotDigest     any        //
	ContentDigest            any        //
	Version                  any        //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
