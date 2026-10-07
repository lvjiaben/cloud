// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditApplication is the golang structure of table plugin_idc_core_credit_application for DAO operations like Where/Data.
type CreditApplication struct {
	g.Meta                `orm:"table:plugin_idc_core_credit_application, do:true"`
	Id                    any        //
	TenantId              any        //
	ApplicationNo         any        //
	CustomerNo            any        //
	CurrencyCode          any        //
	PolicyNo              any        //
	PolicyVersion         any        //
	RequestedLimitMinor   any        //
	RequestedValidFrom    *time.Time //
	RequestedValidUntil   *time.Time //
	RequiredApprovalCount any        //
	RiskEvidenceReference any        //
	RiskEvidenceDigest    any        //
	RequestDigest         any        //
	StatusCode            any        //
	RequestActorTypeCode  any        //
	RequestedBy           any        //
	DecidedAt             *time.Time //
	CancelledAt           *time.Time //
	Version               any        //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
}
