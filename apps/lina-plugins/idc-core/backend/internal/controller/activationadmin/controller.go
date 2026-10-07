package activationadmin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customeractivation"
)

type ControllerV1 struct{ service customeractivation.Service }

func NewV1(service customeractivation.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("customer activation admin controller requires service")
	}
	return &ControllerV1{service: service}, nil
}

func (c *ControllerV1) CustomerActivationInvitationCreate(ctx context.Context, req *v1.CustomerActivationInvitationCreateReq) (*v1.CustomerActivationInvitationCreateRes, error) {
	item, err := c.service.Create(ctx, customeractivation.CreateInput{CustomerType: req.CustomerType, DisplayName: req.DisplayName, CountryCode: req.CountryCode, PreferredLocale: req.PreferredLocale, Email: req.Email, PhoneCountryCode: req.PhoneCountryCode, Phone: req.Phone, BindExistingIdentity: req.BindExistingIdentity, Reason: req.Reason, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerActivationInvitationCreateRes{InvitationNo: item.InvitationNo, CustomerNo: item.CustomerNo, CustomerType: item.CustomerType, DisplayName: item.DisplayName, DestinationHint: item.DestinationHint, ExistingIdentityBound: item.ExistingIdentityBound, Status: item.Status, ExpiresAt: item.ExpiresAt.UnixMilli(), Reused: item.Reused}, nil
}

func (c *ControllerV1) CustomerActivationInvitationResend(ctx context.Context, req *v1.CustomerActivationInvitationResendReq) (*v1.CustomerActivationInvitationResendRes, error) {
	item, err := c.service.Resend(ctx, customeractivation.ResendInput{CustomerNo: req.CustomerNo, Reason: req.Reason, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerActivationInvitationResendRes{InvitationNo: item.InvitationNo, CustomerNo: item.CustomerNo, CustomerType: item.CustomerType, DisplayName: item.DisplayName, DestinationHint: item.DestinationHint, ExistingIdentityBound: item.ExistingIdentityBound, Status: item.Status, ExpiresAt: item.ExpiresAt.UnixMilli(), Reused: item.Reused}, nil
}

// CustomerDirectAccountCreate creates one active customer login without sending an invitation.
func (c *ControllerV1) CustomerDirectAccountCreate(ctx context.Context, req *v1.CustomerDirectAccountCreateReq) (*v1.CustomerDirectAccountCreateRes, error) {
	item, err := c.service.CreateDirect(ctx, customeractivation.DirectCreateInput{
		CustomerType: req.CustomerType, DisplayName: req.DisplayName, CountryCode: req.CountryCode,
		PreferredLocale: req.PreferredLocale, Email: req.Email, PhoneCountryCode: req.PhoneCountryCode,
		Phone: req.Phone, InitialPassword: req.InitialPassword, Reason: req.Reason, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerDirectAccountCreateRes{CustomerNo: item.CustomerNo, UserID: item.UserID, Status: item.Status, Reused: item.Reused}, nil
}

func (c *ControllerV1) CustomerPasswordReset(ctx context.Context, req *v1.CustomerPasswordResetReq) (*v1.CustomerPasswordResetRes, error) {
	err := c.service.ResetPassword(ctx, customeractivation.PasswordResetInput{CustomerNo: req.CustomerNo, NewPassword: req.NewPassword, Reason: req.Reason, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerPasswordResetRes{}, nil
}
