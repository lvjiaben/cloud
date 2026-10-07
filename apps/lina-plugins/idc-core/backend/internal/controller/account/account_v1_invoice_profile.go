package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/invoiceprofile"
)

func (c *ControllerV1) InvoiceProfileList(ctx context.Context, req *v1.InvoiceProfileListReq) (*v1.InvoiceProfileListRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(invoiceprofile.CodeInvalid)
	}
	page, err := c.invoiceProfiles.ListCurrentCustomer(ctx, invoiceprofile.ListInput{Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.InvoiceProfile, 0, len(page.Items))
	for index := range page.Items {
		items = append(items, toAPIInvoiceProfile(&page.Items[index]))
	}
	return &v1.InvoiceProfileListRes{Items: items, NextCursor: page.NextCursor}, nil
}

func (c *ControllerV1) InvoiceProfileCreate(ctx context.Context, req *v1.InvoiceProfileCreateReq) (*v1.InvoiceProfileCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(invoiceprofile.CodeInvalid)
	}
	bankAccount := []byte(req.BankAccount)
	req.BankAccount = ""
	defer clear(bankAccount)
	item, err := c.invoiceProfiles.CreateCurrentCustomer(ctx, invoiceprofile.CreateInput{
		TitleTypeCode: req.TitleTypeCode, InvoiceTitle: req.InvoiceTitle,
		TaxRegistrationNo: req.TaxRegistrationNo, RegisteredAddress: req.RegisteredAddress,
		RegisteredPhone: req.RegisteredPhone, BankName: req.BankName, BankAccount: bankAccount,
		IsDefault: req.IsDefault, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.InvoiceProfileCreateRes{Item: toAPIInvoiceProfile(item)}, nil
}

func (c *ControllerV1) InvoiceProfileUpdate(ctx context.Context, req *v1.InvoiceProfileUpdateReq) (*v1.InvoiceProfileUpdateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(invoiceprofile.CodeInvalid)
	}
	bankAccount := []byte(req.BankAccount)
	req.BankAccount = ""
	defer clear(bankAccount)
	item, err := c.invoiceProfiles.UpdateCurrentCustomer(ctx, invoiceprofile.UpdateInput{
		ProfileNo: req.ProfileNo, ExpectedVersion: req.ExpectedVersion,
		TitleTypeCode: req.TitleTypeCode, InvoiceTitle: req.InvoiceTitle,
		TaxRegistrationNo: req.TaxRegistrationNo, RegisteredAddress: req.RegisteredAddress,
		RegisteredPhone: req.RegisteredPhone, BankName: req.BankName,
		BankAccountAction: req.BankAccountAction, BankAccount: bankAccount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.InvoiceProfileUpdateRes{Item: toAPIInvoiceProfile(item)}, nil
}

func (c *ControllerV1) InvoiceProfileDefaultSet(ctx context.Context, req *v1.InvoiceProfileDefaultSetReq) (*v1.InvoiceProfileDefaultSetRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(invoiceprofile.CodeInvalid)
	}
	item, err := c.invoiceProfiles.SetDefaultCurrentCustomer(ctx, invoiceprofile.VersionInput{
		ProfileNo: req.ProfileNo, ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.InvoiceProfileDefaultSetRes{Item: toAPIInvoiceProfile(item)}, nil
}

func (c *ControllerV1) InvoiceProfileArchive(ctx context.Context, req *v1.InvoiceProfileArchiveReq) (*v1.InvoiceProfileArchiveRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(invoiceprofile.CodeInvalid)
	}
	item, err := c.invoiceProfiles.ArchiveCurrentCustomer(ctx, invoiceprofile.VersionInput{
		ProfileNo: req.ProfileNo, ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.InvoiceProfileArchiveRes{Item: toAPIInvoiceProfile(item)}, nil
}

func toAPIInvoiceProfile(item *invoiceprofile.Profile) *v1.InvoiceProfile {
	if item == nil {
		return nil
	}
	return &v1.InvoiceProfile{
		ProfileNo: item.ProfileNo, TitleTypeCode: item.TitleTypeCode, InvoiceTitle: item.InvoiceTitle,
		TaxRegistrationNo: item.TaxRegistrationNo, RegisteredAddress: item.RegisteredAddress,
		RegisteredPhone: item.RegisteredPhone, BankName: item.BankName,
		BankAccountMasked: item.BankAccountMasked, BankAccountConfigured: item.BankAccountConfigured,
		IsDefault: item.IsDefault, StatusCode: item.StatusCode, Version: item.Version,
		CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}
