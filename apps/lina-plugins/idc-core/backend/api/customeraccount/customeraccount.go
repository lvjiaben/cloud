package customeraccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
)

type ICustomerAccountV1 interface {
	SalesChannelOverviewGet(context.Context, *v1.SalesChannelOverviewGetReq) (*v1.SalesChannelOverviewGetRes, error)
	ReferralOverviewGet(context.Context, *v1.ReferralOverviewGetReq) (*v1.ReferralOverviewGetRes, error)
	ReferralLinkCreate(context.Context, *v1.ReferralLinkCreateReq) (*v1.ReferralLinkCreateRes, error)
	CommissionOverviewGet(context.Context, *v1.CommissionOverviewGetReq) (*v1.CommissionOverviewGetRes, error)
	CommissionSettlementsList(context.Context, *v1.CommissionSettlementsListReq) (*v1.CommissionSettlementsListRes, error)
	CommissionSettlementRequestCreate(context.Context, *v1.CommissionSettlementRequestCreateReq) (*v1.CommissionSettlementRequestCreateRes, error)
	ProfileGet(context.Context, *v1.ProfileGetReq) (*v1.ProfileGetRes, error)
	ProfileUpdate(context.Context, *v1.ProfileUpdateReq) (*v1.ProfileUpdateRes, error)
	ContactList(context.Context, *v1.ContactListReq) (*v1.ContactListRes, error)
	ContactCreate(context.Context, *v1.ContactCreateReq) (*v1.ContactCreateRes, error)
	ContactUpdate(context.Context, *v1.ContactUpdateReq) (*v1.ContactUpdateRes, error)
	ContactArchive(context.Context, *v1.ContactArchiveReq) (*v1.ContactArchiveRes, error)
	AddressList(context.Context, *v1.AddressListReq) (*v1.AddressListRes, error)
	AddressCreate(context.Context, *v1.AddressCreateReq) (*v1.AddressCreateRes, error)
	AddressUpdate(context.Context, *v1.AddressUpdateReq) (*v1.AddressUpdateRes, error)
	AddressArchive(context.Context, *v1.AddressArchiveReq) (*v1.AddressArchiveRes, error)
	ClosureRequest(context.Context, *v1.ClosureRequestReq) (*v1.ClosureRequestRes, error)
	DataExportProof(context.Context, *v1.DataExportProofReq) (*v1.DataExportProofRes, error)
	DataExportRequest(context.Context, *v1.DataExportRequestReq) (*v1.DataExportRequestRes, error)
	DataExportStatus(context.Context, *v1.DataExportStatusReq) (*v1.DataExportStatusRes, error)
	DataExportTicket(context.Context, *v1.DataExportTicketReq) (*v1.DataExportTicketRes, error)
	DataExportOpen(context.Context, *v1.DataExportOpenReq) (*v1.DataExportOpenRes, error)
}
