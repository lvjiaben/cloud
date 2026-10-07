// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyCustomerRequest is the golang structure of table plugin_idc_compliance_privacy_customer_request for DAO operations like Where/Data.
type PrivacyCustomerRequest struct {
	g.Meta          `orm:"table:plugin_idc_compliance_privacy_customer_request, do:true"`
	Id              any        //
	TenantId        any        //
	RequestNo       any        //
	CustomerNo      any        //
	RequestTypeCode any        //
	StatusCode      any        //
	HoldReasonCode  any        //
	SubmittedAt     *time.Time //
	CompletedAt     *time.Time //
	Version         any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
