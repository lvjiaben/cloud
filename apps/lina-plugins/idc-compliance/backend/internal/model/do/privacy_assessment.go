// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyAssessment is the golang structure of table plugin_idc_compliance_privacy_assessment for DAO operations like Where/Data.
type PrivacyAssessment struct {
	g.Meta                   `orm:"table:plugin_idc_compliance_privacy_assessment, do:true"`
	Id                       any        //
	TenantId                 any        //
	AssessmentNo             any        //
	AssessmentTypeCode       any        //
	Version                  any        //
	StatusCode               any        //
	Purpose                  any        //
	Necessity                any        //
	Impact                   any        //
	RecipientDescription     any        //
	StorageLocation          any        //
	RetentionDescription     any        //
	RightsEntry              any        //
	MerchantRole             any        //
	ThirdPartyRole           any        //
	ProcessingInstruction    any        //
	SubcontractingBoundary   any        //
	AuditEvidenceDescription any        //
	CrossBorderTransfer      any        //
	ConfirmedByUserId        any        //
	ConfirmedAt              *time.Time //
	SupersededAt             *time.Time //
	CreatedByUserId          any        //
	UpdatedByUserId          any        //
	RowVersion               any        //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
