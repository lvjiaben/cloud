// This file provides the staff-only, permission-scoped server accessory operation boundary.

package serveroperation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PermissionServerAccessoryView = "idc:admin:resource:server-accessory:view"
	PermissionSnapshotCreate      = "idc:admin:resource:server-accessory:snapshot:create"
	PermissionSnapshotDelete      = "idc:admin:resource:server-accessory:snapshot:delete"
	PermissionSnapshotRestore     = "idc:admin:resource:server-accessory:snapshot:restore"
	PermissionBackupCreate        = "idc:admin:resource:server-accessory:backup:create"
	PermissionBackupDelete        = "idc:admin:resource:server-accessory:backup:delete"
	PermissionBackupRestore       = "idc:admin:resource:server-accessory:backup:restore"
	PermissionSecurityGroupAttach = "idc:admin:resource:server-accessory:security-group:attach"
	PermissionSecurityGroupDetach = "idc:admin:resource:server-accessory:security-group:detach"
	PermissionNATCreate           = "idc:admin:resource:server-accessory:nat:create"
	PermissionNATDelete           = "idc:admin:resource:server-accessory:nat:delete"
	PermissionVirtualMediaMount   = "idc:admin:resource:server-accessory:virtual-media:mount"
	PermissionVirtualMediaUnmount = "idc:admin:resource:server-accessory:virtual-media:unmount"
	PermissionTrafficAdd          = "idc:admin:resource:server-accessory:traffic:add"
)

var (
	CodeAccessoryManagementNotAccessible = bizerr.MustDefine(
		"IDC_SERVER_ACCESSORY_MANAGEMENT_NOT_ACCESSIBLE",
		"The server accessory operation is not accessible", gcode.CodeNotAuthorized,
	)
	CodeAccessoryManagementInvalid = bizerr.MustDefine(
		"IDC_SERVER_ACCESSORY_MANAGEMENT_INVALID",
		"The server accessory operation request is invalid", gcode.CodeInvalidParameter,
	)
	CodeAccessoryManagementUnavailable = bizerr.MustDefine(
		"IDC_SERVER_ACCESSORY_MANAGEMENT_UNAVAILABLE",
		"The server accessory operation is temporarily unavailable", gcode.CodeInternalError,
	)
)

type ManagementCommand struct {
	PackageNo      string
	PackageVersion int64
	ServiceNo      string
	Action         serveroperationresourcecap.Action
	Parameters     json.RawMessage
	IdempotencyKey string
	Confirmed      bool
}

type ManagementDetail struct {
	CustomerNo string
	Detail     *ServerDetail
}

type ManagementResult struct {
	OperationNo     string
	OrderNo         string
	InvoiceNo       string
	Action          serveroperationresourcecap.Action
	Status          string
	AmountMinor     int64
	CurrencyCode    string
	PaymentRequired bool
}

type ManagementService interface {
	// TrafficPackages exposes applicable packages only after staff view/add grants
	// and merchant data-scope checks; it never accepts a caller-supplied customer.
	TrafficPackages(context.Context, string, TrafficPackageSearch) (*TrafficPackagePage, error)
	Get(context.Context, string) (*ManagementDetail, error)
	Create(context.Context, ManagementCommand) (*ManagementResult, error)
}

type managementServiceImpl struct {
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	operations    Service
	directory     *directoryImpl
	parameters    serveroperationresourcecap.ParameterValidator
	pricing       AccessoryPricing
}

func NewManagementService(bizCtx bizctxcap.Service, principalReader principal.Reader,
	authorization authz.Service, operations Service, directory Directory,
	parameters serveroperationresourcecap.ParameterValidator, pricing AccessoryPricing) (ManagementService, error) {
	implementation, ok := directory.(*directoryImpl)
	if nilServerOperationDependency(bizCtx) || nilServerOperationDependency(principalReader) ||
		nilServerOperationDependency(authorization) || nilServerOperationDependency(operations) ||
		nilServerOperationDependency(parameters) || nilServerOperationDependency(pricing) || !ok || implementation == nil {
		return nil, ErrServiceUnavailable
	}
	return &managementServiceImpl{bizCtx: bizCtx, principal: principalReader,
		authorization: authorization, operations: operations, directory: implementation,
		parameters: parameters, pricing: pricing}, nil
}

func (s *managementServiceImpl) Get(ctx context.Context, serviceNo string) (*ManagementDetail, error) {
	if !validReference(serviceNo, 40) {
		return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	actorID, grants, err := s.authorize(ctx, []string{PermissionServerAccessoryView})
	if err != nil {
		return nil, err
	}
	if actorID <= 0 || !grants[PermissionServerAccessoryView] {
		return nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	customerNo, err := s.customerForService(ctx, serviceNo)
	if err != nil {
		return nil, err
	}
	detail, err := s.detail(ctx, customerNo, serviceNo)
	if err != nil {
		return nil, err
	}
	now, err := databaseNow(ctx, s.directory.database)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	if detail.AccessoriesExpiresAt == nil || !detail.AccessoriesExpiresAt.After(now) {
		detail.AvailableActions = []serveroperationresourcecap.Action{}
	}
	permissions := make([]string, 0, len(detail.AvailableActions))
	for _, action := range detail.AvailableActions {
		if permission := staffAccessoryPermission(action); permission != "" {
			permissions = append(permissions, permission)
		}
	}
	if len(permissions) > 0 {
		_, actionGrants, authorizeErr := s.authorize(ctx, permissions)
		if authorizeErr != nil {
			return nil, authorizeErr
		}
		filtered := make([]serveroperationresourcecap.Action, 0, len(detail.AvailableActions))
		for _, action := range detail.AvailableActions {
			permission := staffAccessoryPermission(action)
			if permission != "" && actionGrants[permission] {
				filtered = append(filtered, action)
			}
		}
		detail.AvailableActions = filtered
	} else {
		detail.AvailableActions = []serveroperationresourcecap.Action{}
	}
	return &ManagementDetail{CustomerNo: customerNo, Detail: detail}, nil
}

func (s *managementServiceImpl) Create(ctx context.Context, command ManagementCommand) (*ManagementResult, error) {
	permission := staffAccessoryPermission(command.Action)
	if command.Action == serveroperationresourcecap.ActionTrafficAdd && command.PackageVersion < 1 {
		return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	if permission == "" || !command.Confirmed || !validReference(command.ServiceNo, 40) {
		return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	actorID, grants, err := s.authorize(ctx, []string{permission})
	if err != nil {
		return nil, err
	}
	if actorID <= 0 || !grants[permission] {
		return nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	customerNo, err := s.customerForService(ctx, command.ServiceNo)
	if err != nil {
		return nil, err
	}
	if command.Action == serveroperationresourcecap.ActionTrafficAdd {
		if replay, replayErr := s.replayTrafficPurchase(ctx, customerNo, command); replayErr != nil || replay != nil {
			return replay, replayErr
		}
	}
	detail, err := s.detail(ctx, customerNo, command.ServiceNo)
	if err != nil {
		return nil, err
	}
	now, err := databaseNow(ctx, s.directory.database)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	if detail.AccessoriesExpiresAt == nil || !detail.AccessoriesExpiresAt.After(now) ||
		!containsAccessoryAction(detail.AvailableActions, command.Action) {
		return nil, bizerr.NewCode(CodeAccessoryManagementUnavailable)
	}
	// Fixed traffic capacity is derived and capability-validated by pricing.
	// Other actions continue through the existing parameter boundary.
	if command.Action != serveroperationresourcecap.ActionTrafficAdd {
		if command.PackageNo != "" || command.PackageVersion != 0 {
			return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
		}
		if err = s.parameters.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{
			ServiceNo: command.ServiceNo, ChannelNo: detail.channelNo, PoolNo: detail.poolNo,
		}, command.Action, command.Parameters); err != nil {
			return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
		}
	}
	if isPricedAccessoryCreate(command.Action) {
		quoteKey, referenceErr := deterministicReference("saq_", struct {
			CustomerNo, ServiceNo, IdempotencyKey string
		}{customerNo, command.ServiceNo, command.IdempotencyKey})
		if referenceErr != nil {
			return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
		}
		quote, quoteErr := s.pricing.Preview(ctx, AccessoryQuoteInput{
			CustomerNo: customerNo, ServiceNo: command.ServiceNo, Action: command.Action, PackageNo: command.PackageNo,
			Parameters: command.Parameters, IdempotencyKey: quoteKey,
		})
		if quoteErr != nil {
			return nil, PublicError(quoteErr)
		}
		if command.Action == serveroperationresourcecap.ActionTrafficAdd && quote.PolicyVersion != command.PackageVersion {
			return nil, bizerr.NewCode(CodeAccessoryPriceConflict)
		}
		checkout, checkoutErr := s.pricing.CheckoutForStaff(ctx, customerNo, AccessoryCheckoutInput{
			QuoteNo: quote.QuoteNo, IdempotencyKey: command.IdempotencyKey,
		}, actorID, permission)
		if checkoutErr != nil {
			return nil, PublicError(checkoutErr)
		}
		return &ManagementResult{
			OperationNo: checkout.OperationNo, OrderNo: checkout.OrderNo, InvoiceNo: checkout.InvoiceNo,
			Action: command.Action, Status: checkout.Status, AmountMinor: checkout.AmountMinor,
			CurrencyCode: checkout.CurrencyCode, PaymentRequired: checkout.AmountMinor > 0,
		}, nil
	}
	operation, err := s.operations.Create(ctx, Command{
		CustomerNo: customerNo, ServiceNo: command.ServiceNo, Action: command.Action,
		Parameters: command.Parameters, IdempotencyKey: command.IdempotencyKey,
		ActorType: "staff", ActorReference: strconv.Itoa(actorID), RequiredPermission: permission,
	})
	if err != nil {
		return nil, PublicError(err)
	}
	return &ManagementResult{OperationNo: operation.OperationNo, Action: operation.Action, Status: operation.Status}, nil
}

// replayTrafficPurchase returns only an already committed matching request after
// staff permission and current ownership checks. It never re-quotes, releases
// stock, or re-executes a paid, cancelled or unknown operation.
func (s *managementServiceImpl) replayTrafficPurchase(ctx context.Context, customerNo string, command ManagementCommand) (*ManagementResult, error) {
	parameters := strings.TrimSpace(string(command.Parameters))
	if !validReference(command.PackageNo, 40) || (parameters != "" && parameters != "{}") ||
		len(command.IdempotencyKey) < 8 || len(command.IdempotencyKey) > 256 || strings.TrimSpace(command.IdempotencyKey) != command.IdempotencyKey {
		return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	digest, err := digestValue(struct{ CustomerNo, Key string }{customerNo, command.IdempotencyKey})
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	var row *struct {
		OperationNo, OrderNo, InvoiceNo, ServiceNo, ActionCode, StatusCode, CurrencyCode, PolicyNo string
		AmountMinor, PolicyVersion                                                                 int64
		FinancialSettlementRequired                                                                bool
	}
	err = s.directory.database.Model(dao.BusinessOperation.Table()+" operation").Ctx(ctx).
		InnerJoin(dao.ServerAccessoryQuote.Table()+" quote", "quote.tenant_id=operation.tenant_id AND quote.quote_no=operation.quote_no AND quote.customer_no=operation.customer_no AND quote.service_no=operation.service_no AND quote.currency_code=operation.currency_code").
		WherePrefix("operation", do.BusinessOperation{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
			OperationTypeCode: operationTypeForAction(command.Action), IdempotencyDigest: digest,
		}).
		Fields("operation.operation_no,operation.order_no,operation.invoice_no,operation.service_no,operation.status_code,operation.currency_code,operation.financial_settlement_required,quote.action_code,quote.policy_no,quote.policy_version,quote.total_amount_minor AS amount_minor").
		Scan(&row)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	if row == nil {
		return nil, nil
	}
	if row.ServiceNo != command.ServiceNo || row.ActionCode != string(command.Action) || row.PolicyNo != command.PackageNo || row.PolicyVersion != command.PackageVersion {
		return nil, bizerr.NewCode(CodeAccessoryPriceConflict)
	}
	return &ManagementResult{OperationNo: row.OperationNo, OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo,
		Action: command.Action, Status: row.StatusCode, AmountMinor: row.AmountMinor, CurrencyCode: row.CurrencyCode,
		PaymentRequired: row.FinancialSettlementRequired}, nil
}

func isPricedAccessoryCreate(action serveroperationresourcecap.Action) bool {
	return action == serveroperationresourcecap.ActionSnapshotCreate ||
		action == serveroperationresourcecap.ActionBackupCreate ||
		action == serveroperationresourcecap.ActionTrafficAdd
}

// TrafficPackages reuses the owner projection without impersonating a customer.
func (s *managementServiceImpl) TrafficPackages(ctx context.Context, serviceNo string, input TrafficPackageSearch) (*TrafficPackagePage, error) {
	if !validReference(serviceNo, 40) {
		return nil, bizerr.NewCode(CodeAccessoryManagementInvalid)
	}
	actorID, grants, err := s.authorize(ctx, []string{PermissionServerAccessoryView, PermissionTrafficAdd})
	if err != nil {
		return nil, err
	}
	if actorID <= 0 || !grants[PermissionServerAccessoryView] || !grants[PermissionTrafficAdd] {
		return nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	customerNo, err := s.customerForService(ctx, serviceNo)
	if err != nil {
		return nil, err
	}
	return s.pricing.ForServerTrafficPackages(ctx, customerNo, serviceNo, input)
}

func (s *managementServiceImpl) customerForService(ctx context.Context, serviceNo string) (string, error) {
	value, err := s.directory.database.GetValue(ctx, `SELECT customer_no
FROM plugin_idc_core_service_instance
WHERE tenant_id=0 AND service_no=?`, serviceNo)
	if err != nil {
		return "", bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	customerNo := value.String()
	if !validReference(customerNo, 40) {
		return "", bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	return customerNo, nil
}

func (s *managementServiceImpl) detail(ctx context.Context, customerNo, serviceNo string) (*ServerDetail, error) {
	var row *serverDirectoryRow
	if err := s.directory.database.GetScan(ctx, &row, serverDirectorySQL+` AND service.service_no=?`,
		customerNo, serviceNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryManagementUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	servers, err := s.directory.projectServers(ctx, []serverDirectoryRow{*row})
	if err != nil || len(servers) != 1 {
		return nil, bizerr.NewCode(CodeAccessoryManagementUnavailable)
	}
	projection, err := s.directory.accessories.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: serviceNo, ChannelNo: servers[0].channelNo, PoolNo: servers[0].poolNo,
	})
	if err != nil || projection == nil || projection.ServiceNo != serviceNo {
		return nil, bizerr.NewCode(CodeAccessoryManagementUnavailable)
	}
	detail := &ServerDetail{Server: servers[0], Accessories: append([]serveroperationresourcecap.AccessoryItem(nil), projection.Items...)}
	if !projection.ObservedAt.IsZero() {
		observed := projection.ObservedAt.UTC()
		detail.AccessoriesObservedAt = &observed
	}
	if !projection.ExpiresAt.IsZero() {
		expires := projection.ExpiresAt.UTC()
		detail.AccessoriesExpiresAt = &expires
	}
	return detail, nil
}

func (s *managementServiceImpl) authorize(ctx context.Context, permissions []string) (int, map[string]bool, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 ||
		current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported ||
		(current.DataScope != 1 && current.DataScope != 2) {
		return 0, nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return 0, nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
	}
	keys := make([]authz.PermissionKey, 0, len(permissions)+1)
	for _, permission := range permissions {
		if !idccap.IsCatalogedAdminPermission(permission) {
			return 0, nil, bizerr.NewCode(CodeAccessoryManagementNotAccessible)
		}
		keys = append(keys, authz.PermissionKey(permission))
	}
	keys = append(keys, authz.PermissionKey(idccap.HostPermissionWildcard))
	granted, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, nil, bizerr.WrapCode(err, CodeAccessoryManagementNotAccessible)
	}
	platformAdmin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return 0, nil, bizerr.WrapCode(err, CodeAccessoryManagementNotAccessible)
	}
	result := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		result[permission] = granted[authz.PermissionKey(permission)] ||
			granted[authz.PermissionKey(idccap.HostPermissionWildcard)] || platformAdmin
	}
	return current.UserID, result, nil
}

func staffAccessoryPermission(action serveroperationresourcecap.Action) string {
	switch action {
	case serveroperationresourcecap.ActionSnapshotCreate:
		return PermissionSnapshotCreate
	case serveroperationresourcecap.ActionSnapshotDelete:
		return PermissionSnapshotDelete
	case serveroperationresourcecap.ActionSnapshotRestore:
		return PermissionSnapshotRestore
	case serveroperationresourcecap.ActionBackupCreate:
		return PermissionBackupCreate
	case serveroperationresourcecap.ActionBackupDelete:
		return PermissionBackupDelete
	case serveroperationresourcecap.ActionBackupRestore:
		return PermissionBackupRestore
	case serveroperationresourcecap.ActionSecurityGroupAttach:
		return PermissionSecurityGroupAttach
	case serveroperationresourcecap.ActionSecurityGroupDetach:
		return PermissionSecurityGroupDetach
	case serveroperationresourcecap.ActionNATCreate:
		return PermissionNATCreate
	case serveroperationresourcecap.ActionNATDelete:
		return PermissionNATDelete
	case serveroperationresourcecap.ActionVirtualMediaMount:
		return PermissionVirtualMediaMount
	case serveroperationresourcecap.ActionVirtualMediaUnmount:
		return PermissionVirtualMediaUnmount
	case serveroperationresourcecap.ActionTrafficAdd:
		return PermissionTrafficAdd
	default:
		return ""
	}
}
