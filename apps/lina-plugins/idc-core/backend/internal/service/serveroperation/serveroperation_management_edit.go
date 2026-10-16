// This file exposes staff product edits through the existing owner workflows and exact permission checks.

package serveroperation

import (
	"context"
	"lina-core/pkg/bizerr"
	"strconv"
)

// staffEditCustomer authorizes the direct employee and resolves the immutable server owner.
func (s *managementServiceImpl) staffEditCustomer(ctx context.Context, serviceNo, permission string) (string, int, error) {
	if !validReference(serviceNo, 40) {
		return "", 0, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	actor, grants, err := s.authorize(ctx, []string{permission})
	if err != nil {
		return "", 0, err
	}
	if actor <= 0 || !grants[permission] {
		return "", 0, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	customer, err := s.customerForService(ctx, serviceNo)
	return customer, actor, err
}

// Metadata returns display information only after staff scope and viewing permission checks.
func (s *managementServiceImpl) Metadata(ctx context.Context, serviceNo string) (*CustomerMetadata, error) {
	customer, _, err := s.staffEditCustomer(ctx, serviceNo, PermissionServerAccessoryView)
	if err != nil {
		return nil, err
	}
	var row *CustomerMetadata
	err = s.directory.database.GetScan(ctx, &row, `SELECT customer_alias AS alias,customer_remark AS remark,version FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND service_no=? AND customer_no=?`, serviceNo, customer)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	return row, nil
}

// UpdateMetadata applies a versioned note edit without changing orders or paid amounts.
func (s *managementServiceImpl) UpdateMetadata(ctx context.Context, serviceNo, alias, remark string, version int64) (*CustomerMetadata, error) {
	customer, _, err := s.staffEditCustomer(ctx, serviceNo, PermissionServerEdit)
	if err != nil {
		return nil, err
	}
	result, err := s.operations.UpdateCustomerMetadata(ctx, CustomerMetadataCommand{CustomerNo: customer, ServiceNo: serviceNo, Alias: alias, Remark: remark, ExpectedVersion: version})
	return result, PublicError(err)
}

// staffResizeCustomer uses the same direct-staff scope boundary as paid accessory checkout.
func (s *accessoryPricingImpl) staffResizeCustomer(ctx context.Context, serviceNo string) (string, int, error) {
	if !validReference(serviceNo, 40) {
		return "", 0, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	actor, grants, err := authorizeManagement(ctx, s.bizCtx, s.principal, s.authorization, []string{PermissionServerEdit})
	if err != nil {
		return "", 0, err
	}
	if actor <= 0 || !grants[PermissionServerEdit] {
		return "", 0, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	value, err := s.database.GetValue(ctx, `SELECT customer_no FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND service_no=?`, serviceNo)
	if err != nil {
		return "", 0, bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	customer := value.String()
	if !validReference(customer, 40) {
		return "", 0, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	return customer, actor, nil
}

// ResizeOptionsForStaff reads the existing configuration workflow for an authorized employee.
func (s *accessoryPricingImpl) ResizeOptionsForStaff(ctx context.Context, serviceNo string) (*ResizeOptions, error) {
	customer, _, err := s.staffResizeCustomer(ctx, serviceNo)
	if err != nil {
		return nil, err
	}
	value, err := s.resizeOptions(ctx, customer, serviceNo)
	return value, PublicError(err)
}

// PreviewResizeForStaff freezes the existing price calculation for the selected server.
func (s *accessoryPricingImpl) PreviewResizeForStaff(ctx context.Context, input ResizeQuoteInput) (*ResizeQuote, error) {
	customer, _, err := s.staffResizeCustomer(ctx, input.ServiceNo)
	if err != nil {
		return nil, err
	}
	value, err := s.previewResize(ctx, customer, input)
	return value, PublicError(err)
}

// CheckoutResizeForStaff requires a quote belonging to both the customer and target server.
func (s *accessoryPricingImpl) CheckoutResizeForStaff(ctx context.Context, serviceNo string, input ResizeCheckoutInput) (*ResizeCheckout, error) {
	customer, actor, err := s.staffResizeCustomer(ctx, serviceNo)
	if err != nil {
		return nil, err
	}
	value, err := s.database.GetValue(ctx, `SELECT service_no FROM plugin_idc_core_server_resize_quote WHERE tenant_id=0 AND customer_no=? AND quote_no=?`, customer, input.QuoteNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	if value.String() != serviceNo {
		return nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	result, err := s.checkoutResize(ctx, customer, input, "staff", strconv.Itoa(actor))
	return result, PublicError(err)
}
