// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerSubject is the golang structure of table plugin_idc_core_customer_subject for DAO operations like Where/Data.
type CustomerSubject struct {
	g.Meta                       `orm:"table:plugin_idc_core_customer_subject, do:true"`
	Id                           any        //
	TenantId                     any        //
	CustomerNo                   any        // Opaque public customer business number
	CustomerTypeCode             any        // Customer type code: individual or enterprise
	StatusCode                   any        // Customer lifecycle status code
	DisplayName                  any        // Customer display name without verified legal identity evidence
	CountryCode                  any        //
	PreferredLocale              any        // Preferred IETF locale tag
	Version                      any        //
	AnonymizedAt                 *time.Time // Time when approved account closure anonymization completed
	CreatedBy                    any        //
	UpdatedBy                    any        //
	CreatedAt                    *time.Time //
	UpdatedAt                    *time.Time //
	SearchName                   any        // Service-normalized display-name projection for indexed prefix search
	StatusReasonCode             any        // Stable reason code for the current account restriction or lifecycle state
	StatusReason                 any        // Bounded operator explanation presented through localized business copy
	StatusChangedBy              any        //
	StatusChangedAt              *time.Time //
	EnterpriseContractPolicyCode any        // Enterprise default: none or required; it affects only orders created after the setting changes
	EnterpriseContractTemplateNo any        //
	CustomerCode                 any        // Short monotonic customer number shown to staff and customers; starts at 10000 and is never reused
}
