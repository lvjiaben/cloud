// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerDataExportRequest is the golang structure of table plugin_idc_core_customer_data_export_request for DAO operations like Where/Data.
type CustomerDataExportRequest struct {
	g.Meta               `orm:"table:plugin_idc_core_customer_data_export_request, do:true"`
	Id                   any        //
	TenantId             any        //
	RequestNo            any        //
	CustomerNo           any        //
	ConsentNo            any        //
	TaskNo               any        // Application-validated asynchronous task reference without a cross-domain database foreign key
	ScopeCode            any        //
	FormatCode           any        //
	SubjectVersion       any        //
	IdempotencyKeyDigest any        //
	RequestFingerprint   any        //
	ReauthIssuedAt       *time.Time //
	RequestedByUserId    any        //
	RequestedAt          *time.Time //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
