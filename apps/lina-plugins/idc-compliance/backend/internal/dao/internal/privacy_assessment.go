// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyAssessmentDao is the data access object for the table plugin_idc_compliance_privacy_assessment.
type PrivacyAssessmentDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  PrivacyAssessmentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// PrivacyAssessmentColumns defines and stores column names for the table plugin_idc_compliance_privacy_assessment.
type PrivacyAssessmentColumns struct {
	Id                       string //
	TenantId                 string //
	AssessmentNo             string //
	AssessmentTypeCode       string //
	Version                  string //
	StatusCode               string //
	Purpose                  string //
	Necessity                string //
	Impact                   string //
	RecipientDescription     string //
	StorageLocation          string //
	RetentionDescription     string //
	RightsEntry              string //
	MerchantRole             string //
	ThirdPartyRole           string //
	ProcessingInstruction    string //
	SubcontractingBoundary   string //
	AuditEvidenceDescription string //
	CrossBorderTransfer      string //
	ConfirmedByUserId        string //
	ConfirmedAt              string //
	SupersededAt             string //
	CreatedByUserId          string //
	UpdatedByUserId          string //
	RowVersion               string //
	CreatedAt                string //
	UpdatedAt                string //
}

// privacyAssessmentColumns holds the columns for the table plugin_idc_compliance_privacy_assessment.
var privacyAssessmentColumns = PrivacyAssessmentColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	AssessmentNo:             "assessment_no",
	AssessmentTypeCode:       "assessment_type_code",
	Version:                  "version",
	StatusCode:               "status_code",
	Purpose:                  "purpose",
	Necessity:                "necessity",
	Impact:                   "impact",
	RecipientDescription:     "recipient_description",
	StorageLocation:          "storage_location",
	RetentionDescription:     "retention_description",
	RightsEntry:              "rights_entry",
	MerchantRole:             "merchant_role",
	ThirdPartyRole:           "third_party_role",
	ProcessingInstruction:    "processing_instruction",
	SubcontractingBoundary:   "subcontracting_boundary",
	AuditEvidenceDescription: "audit_evidence_description",
	CrossBorderTransfer:      "cross_border_transfer",
	ConfirmedByUserId:        "confirmed_by_user_id",
	ConfirmedAt:              "confirmed_at",
	SupersededAt:             "superseded_at",
	CreatedByUserId:          "created_by_user_id",
	UpdatedByUserId:          "updated_by_user_id",
	RowVersion:               "row_version",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewPrivacyAssessmentDao creates and returns a new DAO object for table data access.
func NewPrivacyAssessmentDao(handlers ...gdb.ModelHandler) *PrivacyAssessmentDao {
	return &PrivacyAssessmentDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_assessment",
		columns:  privacyAssessmentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacyAssessmentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacyAssessmentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacyAssessmentDao) Columns() PrivacyAssessmentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacyAssessmentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacyAssessmentDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *PrivacyAssessmentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
