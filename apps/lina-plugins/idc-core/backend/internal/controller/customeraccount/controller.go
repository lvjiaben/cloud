package customeraccount

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	accountapi "lina-plugin-idc-core/backend/api/customeraccount"
	v1 "lina-plugin-idc-core/backend/api/customeraccount/v1"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customeraddress"
	"lina-plugin-idc-core/backend/internal/service/customeravatar"
	"lina-plugin-idc-core/backend/internal/service/customercontact"
	"lina-plugin-idc-core/backend/internal/service/customerexport"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

type ControllerV1 struct {
	avatars      customeravatar.Service
	customers    customer.Service
	contacts     customercontact.Service
	addresses    customeraddress.Service
	exports      customerexport.Delivery
	salesChannel saleschannel.CustomerService
}

func BindSalesChannel(controller *ControllerV1, service saleschannel.CustomerService) error {
	if controller == nil || service == nil || controller.salesChannel != nil {
		return gerror.New("customer sales channel service must be bound exactly once")
	}
	controller.salesChannel = service
	return nil
}

var _ accountapi.ICustomerAccountV1 = (*ControllerV1)(nil)

func NewV1(customers customer.Service, contacts customercontact.Service, addresses customeraddress.Service, exports customerexport.Delivery, avatars customeravatar.Service) (*ControllerV1, error) {
	if customers == nil || contacts == nil || addresses == nil || exports == nil || avatars == nil {
		return nil, gerror.New("customer account controller requires customer service")
	}
	return &ControllerV1{customers: customers, contacts: contacts, addresses: addresses, exports: exports, avatars: avatars}, nil
}

func (c *ControllerV1) ProfileGet(ctx context.Context, _ *v1.ProfileGetReq) (*v1.ProfileGetRes, error) {
	item, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ProfileGetRes{Item: profileItem(item)}, nil
}

func (c *ControllerV1) ProfileUpdate(ctx context.Context, req *v1.ProfileUpdateReq) (*v1.ProfileUpdateRes, error) {
	current, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.customers.UpdateOwnBasics(ctx, customer.UpdateOwnBasicsInput{
		CustomerNo: current.Subject.CustomerNo, DisplayName: req.DisplayName,
		CountryCode: req.CountryCode, PreferredLocale: req.PreferredLocale,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ProfileUpdateRes{Item: profileItem(item)}, nil
}

func (c *ControllerV1) ContactList(ctx context.Context, _ *v1.ContactListReq) (*v1.ContactListRes, error) {
	items, err := c.contacts.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.ContactItem, 0, len(items))
	for index := range items {
		result = append(result, contactItem(&items[index]))
	}
	return &v1.ContactListRes{Items: result}, nil
}

func (c *ControllerV1) ContactCreate(ctx context.Context, req *v1.ContactCreateReq) (*v1.ContactCreateRes, error) {
	item, err := c.contacts.Save(ctx, contactSaveInput("", 0, req.ContactSaveFields))
	if err != nil {
		return nil, err
	}
	return &v1.ContactCreateRes{Item: contactItem(item)}, nil
}

func (c *ControllerV1) ContactUpdate(ctx context.Context, req *v1.ContactUpdateReq) (*v1.ContactUpdateRes, error) {
	item, err := c.contacts.Save(ctx, contactSaveInput(req.ContactNo, req.ExpectedVersion, req.ContactSaveFields))
	if err != nil {
		return nil, err
	}
	return &v1.ContactUpdateRes{Item: contactItem(item)}, nil
}

func (c *ControllerV1) ContactArchive(ctx context.Context, req *v1.ContactArchiveReq) (*v1.ContactArchiveRes, error) {
	item, err := c.contacts.Archive(ctx, customercontact.ArchiveInput{
		ContactNo: req.ContactNo, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ContactArchiveRes{Item: contactItem(item)}, nil
}

func contactSaveInput(contactNo string, expectedVersion int64, fields v1.ContactSaveFields) customercontact.SaveInput {
	return customercontact.SaveInput{ContactNo: contactNo, ExpectedVersion: expectedVersion,
		DisplayName: fields.DisplayName, Email: fields.Email, PhoneCountryCode: fields.PhoneCountryCode,
		Phone: fields.Phone, PreferredLocale: fields.PreferredLocale, IdempotencyKey: fields.IdempotencyKey}
}

func contactItem(item *customercontact.Contact) *v1.ContactItem {
	if item == nil {
		return nil
	}
	return &v1.ContactItem{ContactNo: item.ContactNo, DisplayName: item.DisplayName,
		Email: item.Email, EmailVerified: item.EmailVerified, PhoneCountryCode: item.PhoneCountryCode,
		Phone: item.Phone, PhoneVerified: item.PhoneVerified, PreferredLocale: item.PreferredLocale,
		Status: string(item.Status), Version: item.Version, CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli()}
}

func (c *ControllerV1) AddressList(ctx context.Context, _ *v1.AddressListReq) (*v1.AddressListRes, error) {
	items, err := c.addresses.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.AddressItem, 0, len(items))
	for index := range items {
		result = append(result, addressItem(&items[index]))
	}
	return &v1.AddressListRes{Items: result}, nil
}

func (c *ControllerV1) AddressCreate(ctx context.Context, req *v1.AddressCreateReq) (*v1.AddressCreateRes, error) {
	item, err := c.addresses.Save(ctx, addressSaveInput("", 0, req.AddressSaveFields))
	if err != nil {
		return nil, err
	}
	return &v1.AddressCreateRes{Item: addressItem(item)}, nil
}

func (c *ControllerV1) AddressUpdate(ctx context.Context, req *v1.AddressUpdateReq) (*v1.AddressUpdateRes, error) {
	item, err := c.addresses.Save(ctx, addressSaveInput(req.AddressNo, req.ExpectedVersion, req.AddressSaveFields))
	if err != nil {
		return nil, err
	}
	return &v1.AddressUpdateRes{Item: addressItem(item)}, nil
}

func (c *ControllerV1) AddressArchive(ctx context.Context, req *v1.AddressArchiveReq) (*v1.AddressArchiveRes, error) {
	item, err := c.addresses.Archive(ctx, customeraddress.ArchiveInput{AddressNo: req.AddressNo, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.AddressArchiveRes{Item: addressItem(item)}, nil
}

func addressSaveInput(addressNo string, expectedVersion int64, fields v1.AddressSaveFields) customeraddress.SaveInput {
	return customeraddress.SaveInput{AddressNo: addressNo, ExpectedVersion: expectedVersion, Type: customeraddress.Type(fields.Type),
		Label: fields.Label, RecipientName: fields.RecipientName, RecipientPhone: fields.RecipientPhone,
		CountryCode: fields.CountryCode, Province: fields.Province, City: fields.City, District: fields.District,
		AddressLine1: fields.AddressLine1, AddressLine2: fields.AddressLine2, PostalCode: fields.PostalCode,
		Default: fields.Default, IdempotencyKey: fields.IdempotencyKey}
}

func addressItem(item *customeraddress.Address) *v1.AddressItem {
	if item == nil {
		return nil
	}
	return &v1.AddressItem{AddressNo: item.AddressNo, Type: string(item.Type), Label: item.Label,
		RecipientName: item.RecipientName, RecipientPhone: item.RecipientPhone, CountryCode: item.CountryCode,
		Province: item.Province, City: item.City, District: item.District, AddressLine1: item.AddressLine1,
		AddressLine2: item.AddressLine2, PostalCode: item.PostalCode, Default: item.Default,
		Status: string(item.Status), Version: item.Version, CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli()}
}

func (c *ControllerV1) ClosureRequest(ctx context.Context, req *v1.ClosureRequestReq) (*v1.ClosureRequestRes, error) {
	current, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.customers.RequestOwnClosure(ctx, customer.RequestOwnClosureInput{
		CustomerNo: current.Subject.CustomerNo, ExpectedVersion: req.ExpectedVersion,
		Confirmed: req.Confirmed, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	findings := make([]*v1.ClosureFindingItem, 0, len(item.Findings))
	for _, finding := range item.Findings {
		findings = append(findings, &v1.ClosureFindingItem{Code: string(finding.Code), Count: finding.Count})
	}
	return &v1.ClosureRequestRes{Item: &v1.ClosureItem{RequestNo: item.RequestNo, Status: string(item.Status), Findings: findings, RequestedAt: item.RequestedAt.UnixMilli()}}, nil
}

func (c *ControllerV1) DataExportProof(ctx context.Context, req *v1.DataExportProofReq) (*v1.DataExportProofRes, error) {
	current, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	proof, err := c.customers.IssueOwnDataExportProof(ctx, customer.OwnDataExportIntent{
		CustomerNo: current.Subject.CustomerNo, Scope: customer.DataExportScopeOwnDeliverable,
		Format: customer.DataExportFormatJSONZip, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed,
	})
	if err != nil {
		return nil, err
	}
	return &v1.DataExportProofRes{ProofToken: proof.Token, ExpiresAt: proof.ExpiresAt.UnixMilli()}, nil
}

func (c *ControllerV1) DataExportRequest(ctx context.Context, req *v1.DataExportRequestReq) (*v1.DataExportRequestRes, error) {
	current, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.customers.RequestOwnDataExport(ctx, customer.RequestOwnDataExportInput{
		OwnDataExportIntent: customer.OwnDataExportIntent{CustomerNo: current.Subject.CustomerNo,
			Scope: customer.DataExportScopeOwnDeliverable, Format: customer.DataExportFormatJSONZip,
			ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed},
		ProofToken: req.ProofToken,
	})
	if err != nil {
		return nil, err
	}
	return &v1.DataExportRequestRes{RequestNo: item.RequestNo, TaskNo: item.TaskNo, RequestedAt: item.RequestedAt.UnixMilli()}, nil
}

func (c *ControllerV1) DataExportStatus(ctx context.Context, req *v1.DataExportStatusReq) (*v1.DataExportStatusRes, error) {
	item, err := c.exports.CurrentStatus(ctx, req.RequestNo)
	if err != nil {
		return nil, err
	}
	var expiresAt int64
	if item.ExpiresAt != nil {
		expiresAt = item.ExpiresAt.UnixMilli()
	}
	return &v1.DataExportStatusRes{RequestNo: item.RequestNo, Status: item.Status, RequestedAt: item.RequestedAt.UnixMilli(), ExpiresAt: expiresAt}, nil
}

func (c *ControllerV1) DataExportTicket(ctx context.Context, req *v1.DataExportTicketReq) (*v1.DataExportTicketRes, error) {
	item, err := c.exports.IssueTicket(ctx, req.RequestNo)
	if err != nil {
		return nil, err
	}
	return &v1.DataExportTicketRes{Token: item.Token, ExpiresAt: item.ExpiresAt.UnixMilli()}, nil
}

func (c *ControllerV1) DataExportOpen(ctx context.Context, req *v1.DataExportOpenReq) (res *v1.DataExportOpenRes, err error) {
	request := g.RequestFromCtx(ctx)
	file, err := c.exports.Open(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	if request == nil || file == nil || file.Body == nil {
		return nil, gerror.New("customer export stream is unavailable")
	}
	defer func() {
		if closeErr := file.Body.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	request.Response.Header().Set("Content-Type", file.MediaType)
	request.Response.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	safeName := strings.NewReplacer("\\", "_", "\"", "_", "\r", "_", "\n", "_").Replace(file.FileName)
	request.Response.Header().Set("Content-Disposition", "attachment; filename=\""+safeName+"\"")
	if _, err = io.Copy(request.Response.RawWriter(), file.Body); err != nil {
		return nil, err
	}
	request.ExitAll()
	return &v1.DataExportOpenRes{}, nil
}

func profileItem(profile *customer.Profile) *v1.ProfileItem {
	if profile == nil {
		return nil
	}
	item := &v1.ProfileItem{CustomerCode: profile.Subject.CustomerCode, Type: string(profile.Subject.Type),
		Status: string(profile.Subject.Status), DisplayName: profile.Subject.DisplayName,
		CountryCode: profile.Subject.CountryCode, PreferredLocale: profile.Subject.PreferredLocale,
		Version: profile.Subject.Version}
	if profile.Individual != nil {
		item.Verification = string(profile.Individual.IdentityStatus)
		item.LegalNameMasked = profile.Individual.LegalNameMasked
		item.IdentifierMasked = profile.Individual.IdentityIdentifierMasked
	}
	if profile.Enterprise != nil {
		item.Verification = string(profile.Enterprise.VerificationStatus)
		item.LegalNameMasked = profile.Enterprise.LegalName
		item.IdentifierMasked = profile.Enterprise.RegistrationIdentifierMasked
	}
	return item
}
