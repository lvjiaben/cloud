// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package customeraccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
)

type ICustomeraccountV1 interface {
	AvatarUpload(ctx context.Context, req *v1.AvatarUploadReq) (res *v1.AvatarUploadRes, err error)
	ProfileGet(ctx context.Context, req *v1.ProfileGetReq) (res *v1.ProfileGetRes, err error)
	ProfileUpdate(ctx context.Context, req *v1.ProfileUpdateReq) (res *v1.ProfileUpdateRes, err error)
	ContactList(ctx context.Context, req *v1.ContactListReq) (res *v1.ContactListRes, err error)
	ContactCreate(ctx context.Context, req *v1.ContactCreateReq) (res *v1.ContactCreateRes, err error)
	ContactUpdate(ctx context.Context, req *v1.ContactUpdateReq) (res *v1.ContactUpdateRes, err error)
	ContactArchive(ctx context.Context, req *v1.ContactArchiveReq) (res *v1.ContactArchiveRes, err error)
	AddressList(ctx context.Context, req *v1.AddressListReq) (res *v1.AddressListRes, err error)
	AddressCreate(ctx context.Context, req *v1.AddressCreateReq) (res *v1.AddressCreateRes, err error)
	AddressUpdate(ctx context.Context, req *v1.AddressUpdateReq) (res *v1.AddressUpdateRes, err error)
	AddressArchive(ctx context.Context, req *v1.AddressArchiveReq) (res *v1.AddressArchiveRes, err error)
	ClosureRequest(ctx context.Context, req *v1.ClosureRequestReq) (res *v1.ClosureRequestRes, err error)
	DataExportProof(ctx context.Context, req *v1.DataExportProofReq) (res *v1.DataExportProofRes, err error)
	DataExportRequest(ctx context.Context, req *v1.DataExportRequestReq) (res *v1.DataExportRequestRes, err error)
	DataExportStatus(ctx context.Context, req *v1.DataExportStatusReq) (res *v1.DataExportStatusRes, err error)
	DataExportTicket(ctx context.Context, req *v1.DataExportTicketReq) (res *v1.DataExportTicketRes, err error)
	DataExportOpen(ctx context.Context, req *v1.DataExportOpenReq) (res *v1.DataExportOpenRes, err error)
	SalesChannelOverviewGet(ctx context.Context, req *v1.SalesChannelOverviewGetReq) (res *v1.SalesChannelOverviewGetRes, err error)
	ReferralOverviewGet(ctx context.Context, req *v1.ReferralOverviewGetReq) (res *v1.ReferralOverviewGetRes, err error)
	ReferralLinkCreate(ctx context.Context, req *v1.ReferralLinkCreateReq) (res *v1.ReferralLinkCreateRes, err error)
	CommissionOverviewGet(ctx context.Context, req *v1.CommissionOverviewGetReq) (res *v1.CommissionOverviewGetRes, err error)
	CommissionSettlementsList(ctx context.Context, req *v1.CommissionSettlementsListReq) (res *v1.CommissionSettlementsListRes, err error)
}
