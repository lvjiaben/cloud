// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
)

type IAdminV1 interface {
	EnterpriseVerificationApprovalCreate(ctx context.Context, req *v1.EnterpriseVerificationApprovalCreateReq) (res *v1.EnterpriseVerificationApprovalCreateRes, err error)
	EnterpriseVerificationCaseGet(ctx context.Context, req *v1.EnterpriseVerificationCaseGetReq) (res *v1.EnterpriseVerificationCaseGetRes, err error)
	EnterpriseVerificationCaseList(ctx context.Context, req *v1.EnterpriseVerificationCaseListReq) (res *v1.EnterpriseVerificationCaseListRes, err error)
	EnterpriseVerificationMaterialAccessCreate(ctx context.Context, req *v1.EnterpriseVerificationMaterialAccessCreateReq) (res *v1.EnterpriseVerificationMaterialAccessCreateRes, err error)
	EnterpriseVerificationMaterialContentCreate(ctx context.Context, req *v1.EnterpriseVerificationMaterialContentCreateReq) (res *v1.EnterpriseVerificationMaterialContentCreateRes, err error)
	EnterpriseVerificationRejectionCreate(ctx context.Context, req *v1.EnterpriseVerificationRejectionCreateReq) (res *v1.EnterpriseVerificationRejectionCreateRes, err error)
	EnterpriseVerificationReviewStartCreate(ctx context.Context, req *v1.EnterpriseVerificationReviewStartCreateReq) (res *v1.EnterpriseVerificationReviewStartCreateRes, err error)
	PrivacyDeletionConfirmCreate(ctx context.Context, req *v1.PrivacyDeletionConfirmCreateReq) (res *v1.PrivacyDeletionConfirmCreateRes, err error)
	PrivacyDeletionList(ctx context.Context, req *v1.PrivacyDeletionListReq) (res *v1.PrivacyDeletionListRes, err error)
	PrivacyDeletionScanCreate(ctx context.Context, req *v1.PrivacyDeletionScanCreateReq) (res *v1.PrivacyDeletionScanCreateRes, err error)
	PrivacyRetentionHoldCreate(ctx context.Context, req *v1.PrivacyRetentionHoldCreateReq) (res *v1.PrivacyRetentionHoldCreateRes, err error)
	PrivacyRetentionHoldReleaseCreate(ctx context.Context, req *v1.PrivacyRetentionHoldReleaseCreateReq) (res *v1.PrivacyRetentionHoldReleaseCreateRes, err error)
	PrivacyRetentionList(ctx context.Context, req *v1.PrivacyRetentionListReq) (res *v1.PrivacyRetentionListRes, err error)
	PrivacyRetentionUpdate(ctx context.Context, req *v1.PrivacyRetentionUpdateReq) (res *v1.PrivacyRetentionUpdateRes, err error)
	ProviderConfigurationGet(ctx context.Context, req *v1.ProviderConfigurationGetReq) (res *v1.ProviderConfigurationGetRes, err error)
	ProviderConfigurationRecoveryCreate(ctx context.Context, req *v1.ProviderConfigurationRecoveryCreateReq) (res *v1.ProviderConfigurationRecoveryCreateRes, err error)
	ProviderConfigurationUpdate(ctx context.Context, req *v1.ProviderConfigurationUpdateReq) (res *v1.ProviderConfigurationUpdateRes, err error)
	ProviderConfigurationValidationCreate(ctx context.Context, req *v1.ProviderConfigurationValidationCreateReq) (res *v1.ProviderConfigurationValidationCreateRes, err error)
	RedLetterAllocationResolutionCreate(ctx context.Context, req *v1.RedLetterAllocationResolutionCreateReq) (res *v1.RedLetterAllocationResolutionCreateRes, err error)
	RedLetterManualCompletionCreate(ctx context.Context, req *v1.RedLetterManualCompletionCreateReq) (res *v1.RedLetterManualCompletionCreateRes, err error)
	RedLetterTaskGet(ctx context.Context, req *v1.RedLetterTaskGetReq) (res *v1.RedLetterTaskGetRes, err error)
	RedLetterTaskList(ctx context.Context, req *v1.RedLetterTaskListReq) (res *v1.RedLetterTaskListRes, err error)
	SupplierConfigurationGet(ctx context.Context, req *v1.SupplierConfigurationGetReq) (res *v1.SupplierConfigurationGetRes, err error)
	SupplierCredentialUpdate(ctx context.Context, req *v1.SupplierCredentialUpdateReq) (res *v1.SupplierCredentialUpdateRes, err error)
	SupplierSubjectBindingCreate(ctx context.Context, req *v1.SupplierSubjectBindingCreateReq) (res *v1.SupplierSubjectBindingCreateRes, err error)
	SupplierSubjectReopeningCreate(ctx context.Context, req *v1.SupplierSubjectReopeningCreateReq) (res *v1.SupplierSubjectReopeningCreateRes, err error)
	SupplierSubjectRevocationCreate(ctx context.Context, req *v1.SupplierSubjectRevocationCreateReq) (res *v1.SupplierSubjectRevocationCreateRes, err error)
	TaxInvoiceApprovalCreate(ctx context.Context, req *v1.TaxInvoiceApprovalCreateReq) (res *v1.TaxInvoiceApprovalCreateRes, err error)
	TaxInvoiceCancellationCreate(ctx context.Context, req *v1.TaxInvoiceCancellationCreateReq) (res *v1.TaxInvoiceCancellationCreateRes, err error)
	TaxInvoiceGet(ctx context.Context, req *v1.TaxInvoiceGetReq) (res *v1.TaxInvoiceGetRes, err error)
	TaxInvoiceList(ctx context.Context, req *v1.TaxInvoiceListReq) (res *v1.TaxInvoiceListRes, err error)
	TaxInvoiceRejectionCreate(ctx context.Context, req *v1.TaxInvoiceRejectionCreateReq) (res *v1.TaxInvoiceRejectionCreateRes, err error)
	TaxInvoiceSubmissionCreate(ctx context.Context, req *v1.TaxInvoiceSubmissionCreateReq) (res *v1.TaxInvoiceSubmissionCreateRes, err error)
}
