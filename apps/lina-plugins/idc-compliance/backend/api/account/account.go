// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package account

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/account/v1"
)

type IAccountV1 interface {
	CustomerPrivacyConsentWithdrawCreate(ctx context.Context, req *v1.CustomerPrivacyConsentWithdrawCreateReq) (res *v1.CustomerPrivacyConsentWithdrawCreateRes, err error)
	CustomerPrivacyGet(ctx context.Context, req *v1.CustomerPrivacyGetReq) (res *v1.CustomerPrivacyGetRes, err error)
	CustomerRedLetterAuthoritativeStateUpdate(ctx context.Context, req *v1.CustomerRedLetterAuthoritativeStateUpdateReq) (res *v1.CustomerRedLetterAuthoritativeStateUpdateRes, err error)
	CustomerRedLetterTaskGet(ctx context.Context, req *v1.CustomerRedLetterTaskGetReq) (res *v1.CustomerRedLetterTaskGetRes, err error)
	CustomerTaxInvoiceCreate(ctx context.Context, req *v1.CustomerTaxInvoiceCreateReq) (res *v1.CustomerTaxInvoiceCreateRes, err error)
	CustomerTaxInvoiceList(ctx context.Context, req *v1.CustomerTaxInvoiceListReq) (res *v1.CustomerTaxInvoiceListRes, err error)
	CustomerTaxInvoiceOptionsGet(ctx context.Context, req *v1.CustomerTaxInvoiceOptionsGetReq) (res *v1.CustomerTaxInvoiceOptionsGetRes, err error)
	EnterpriseLegalChangeCreate(ctx context.Context, req *v1.EnterpriseLegalChangeCreateReq) (res *v1.EnterpriseLegalChangeCreateRes, err error)
	EnterpriseVerificationConsentCreate(ctx context.Context, req *v1.EnterpriseVerificationConsentCreateReq) (res *v1.EnterpriseVerificationConsentCreateRes, err error)
	EnterpriseVerificationCreate(ctx context.Context, req *v1.EnterpriseVerificationCreateReq) (res *v1.EnterpriseVerificationCreateRes, err error)
	EnterpriseVerificationGet(ctx context.Context, req *v1.EnterpriseVerificationGetReq) (res *v1.EnterpriseVerificationGetRes, err error)
	EnterpriseVerificationMaterialContentUpdate(ctx context.Context, req *v1.EnterpriseVerificationMaterialContentUpdateReq) (res *v1.EnterpriseVerificationMaterialContentUpdateRes, err error)
	EnterpriseVerificationMaterialCreate(ctx context.Context, req *v1.EnterpriseVerificationMaterialCreateReq) (res *v1.EnterpriseVerificationMaterialCreateRes, err error)
	EnterpriseVerificationSubmit(ctx context.Context, req *v1.EnterpriseVerificationSubmitReq) (res *v1.EnterpriseVerificationSubmitRes, err error)
	PersonalVerificationAuthoritativeStateUpdate(ctx context.Context, req *v1.PersonalVerificationAuthoritativeStateUpdateReq) (res *v1.PersonalVerificationAuthoritativeStateUpdateRes, err error)
	PersonalVerificationConsentCreate(ctx context.Context, req *v1.PersonalVerificationConsentCreateReq) (res *v1.PersonalVerificationConsentCreateRes, err error)
	PersonalVerificationCreate(ctx context.Context, req *v1.PersonalVerificationCreateReq) (res *v1.PersonalVerificationCreateRes, err error)
	PersonalVerificationGet(ctx context.Context, req *v1.PersonalVerificationGetReq) (res *v1.PersonalVerificationGetRes, err error)
}
