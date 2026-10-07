// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyAssessment is the golang structure for table privacy_assessment.
type PrivacyAssessment struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	AssessmentNo             string     `json:"assessmentNo"             orm:"assessment_no"              description:""`
	AssessmentTypeCode       string     `json:"assessmentTypeCode"       orm:"assessment_type_code"       description:""`
	Version                  int64      `json:"version"                  orm:"version"                    description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                description:""`
	Purpose                  string     `json:"purpose"                  orm:"purpose"                    description:""`
	Necessity                string     `json:"necessity"                orm:"necessity"                  description:""`
	Impact                   string     `json:"impact"                   orm:"impact"                     description:""`
	RecipientDescription     string     `json:"recipientDescription"     orm:"recipient_description"      description:""`
	StorageLocation          string     `json:"storageLocation"          orm:"storage_location"           description:""`
	RetentionDescription     string     `json:"retentionDescription"     orm:"retention_description"      description:""`
	RightsEntry              string     `json:"rightsEntry"              orm:"rights_entry"               description:""`
	MerchantRole             string     `json:"merchantRole"             orm:"merchant_role"              description:""`
	ThirdPartyRole           string     `json:"thirdPartyRole"           orm:"third_party_role"           description:""`
	ProcessingInstruction    string     `json:"processingInstruction"    orm:"processing_instruction"     description:""`
	SubcontractingBoundary   string     `json:"subcontractingBoundary"   orm:"subcontracting_boundary"    description:""`
	AuditEvidenceDescription string     `json:"auditEvidenceDescription" orm:"audit_evidence_description" description:""`
	CrossBorderTransfer      bool       `json:"crossBorderTransfer"      orm:"cross_border_transfer"      description:""`
	ConfirmedByUserId        int        `json:"confirmedByUserId"        orm:"confirmed_by_user_id"       description:""`
	ConfirmedAt              *time.Time `json:"confirmedAt"              orm:"confirmed_at"               description:""`
	SupersededAt             *time.Time `json:"supersededAt"             orm:"superseded_at"              description:""`
	CreatedByUserId          int        `json:"createdByUserId"          orm:"created_by_user_id"         description:""`
	UpdatedByUserId          int        `json:"updatedByUserId"          orm:"updated_by_user_id"         description:""`
	RowVersion               int64      `json:"rowVersion"               orm:"row_version"                description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                 description:""`
}
