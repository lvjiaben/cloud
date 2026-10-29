package billing

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	permissionBillingOverview                = "idc:admin:billing:overview"
	permissionBillingInvoiceList             = "idc:admin:billing:invoice:list"
	permissionBillingInvoiceQuery            = "idc:admin:billing:invoice:query"
	permissionBillingTransactionList         = "idc:admin:billing:transaction:list"
	permissionBillingTransactionQuery        = "idc:admin:billing:transaction:query"
	permissionBillingAdjustmentList          = "idc:admin:billing:adjustment:list"
	permissionBillingAdjustmentQuery         = "idc:admin:billing:adjustment:query"
	permissionBillingBalanceAdjustmentCreate = "idc:admin:billing:balance-adjustment:create"
	permissionBillingManualReceiptCreate     = "idc:admin:billing:manual-receipt:create"
	permissionBillingInvoiceWaiverCreate     = "idc:admin:billing:invoice-waiver:create"
	permissionBillingAdjustmentReview        = "idc:admin:billing:adjustment:review"
	permissionBillingAdjustmentPolicyWrite   = "idc:admin:billing:adjustment-policy:write"
	permissionBillingInvoiceResend           = "idc:admin:billing:invoice:resend"
	permissionBillingNoteList                = "idc:admin:billing:note:list"
	permissionBillingNoteCreate              = "idc:admin:billing:note:create"
	permissionBillingPersonalFilter          = "idc:admin:billing:filter:personal"
	permissionBillingExport                  = "idc:admin:billing:export"
	permissionBillingRefundList              = "idc:admin:billing:refund:list"
	permissionBillingRefundQuery             = "idc:admin:billing:refund:query"
	permissionBillingRefundReview            = "idc:admin:billing:refund:review"
	permissionBillingRefundPolicyWrite       = "idc:admin:billing:refund-policy:write"
)

// ManagementFinanceAuthorization is the smallest current staff scope needed
// by operations that must recheck both identity and representable data scope.
type ManagementFinanceAuthorization struct {
	UserID    int
	DataScope int
}

// ManagementFinanceAuthorizer keeps every staff financial action behind its
// own exact permission and a directly authenticated, representable data scope.
type ManagementFinanceAuthorizer interface {
	AuthorizeFinancialOverview(context.Context) (int, error)
	AuthorizeInvoiceList(context.Context) (int, error)
	AuthorizeInvoiceDetail(context.Context, string) (int, error)
	AuthorizeTransactionList(context.Context) (int, error)
	AuthorizeTransactionDetail(context.Context, string) (int, error)
	AuthorizeAdjustmentList(context.Context) (int, error)
	AuthorizeAdjustmentDetail(context.Context, string) (int, error)
	// AuthorizeBalanceAdjustmentCreate requires direct staff, default merchant,
	// representable data scope and the exact customer-balance adjustment permission.
	AuthorizeBalanceAdjustmentCreate(context.Context) (int, error)
	AuthorizeManualReceiptCreate(context.Context) (int, error)
	AuthorizeInvoiceWaiverCreate(context.Context) (int, error)
	AuthorizeAdjustmentReview(context.Context, string) (int, error)
	AuthorizeAdjustmentPolicyWrite(context.Context) (int, error)
}

// ManagementFinanceOperationsAuthorizer is separated from the established
// billing directory contract so adding convenience operations does not widen
// unrelated readers or command test doubles.
type ManagementFinanceOperationsAuthorizer interface {
	AuthorizeInvoiceResend(context.Context, string) (int, error)
	AuthorizeInternalNoteList(context.Context, string) (int, error)
	AuthorizeInternalNoteCreate(context.Context, string) (int, error)
	AuthorizePersonalFilter(context.Context) (int, error)
	AuthorizeFinancialExport(context.Context) (ManagementFinanceAuthorization, error)
}

// ManagementRefundAuthorizer keeps refund reading, approval, and policy
// configuration behind separate exact permissions.
type ManagementRefundAuthorizer interface {
	AuthorizeRefundList(context.Context) (int, error)
	AuthorizeRefundDetail(context.Context, string) (int, error)
	AuthorizeRefundReview(context.Context, string) (int, error)
	AuthorizeRefundPolicyWrite(context.Context) (int, error)
}

// ManagementFinanceRuntimeAuthorizer is the complete local staff-finance
// authorization boundary used while assembling the plugin runtime. Readers,
// commands, and convenience operations can still depend on the narrower
// interfaces above.
type ManagementFinanceRuntimeAuthorizer interface {
	ManagementFinanceAuthorizer
	ManagementFinanceOperationsAuthorizer
	ManagementRefundAuthorizer
}

type managementFinanceAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

var _ ManagementFinanceAuthorizer = (*managementFinanceAuthorizerImpl)(nil)

func NewManagementFinanceAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (ManagementFinanceRuntimeAuthorizer, error) {
	if isNilFinanceDirectoryDependency(bizCtxSvc) ||
		isNilFinanceDirectoryDependency(principalReader) ||
		isNilFinanceDirectoryDependency(authorization) {
		return nil, gerror.New("management financial authorizer requires bizctx, principal, and authz services")
	}
	return &managementFinanceAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}, nil
}

func (a *managementFinanceAuthorizerImpl) AuthorizeFinancialOverview(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingOverview)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeInvoiceList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingInvoiceList)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeInvoiceDetail(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingInvoiceQuery, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeTransactionList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingTransactionList)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeTransactionDetail(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingTransactionQuery, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeAdjustmentList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingAdjustmentList)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeAdjustmentDetail(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingAdjustmentQuery, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeManualReceiptCreate(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingManualReceiptCreate)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeInvoiceWaiverCreate(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingInvoiceWaiverCreate)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeAdjustmentReview(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingAdjustmentReview, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeAdjustmentPolicyWrite(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingAdjustmentPolicyWrite)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeInvoiceResend(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingInvoiceResend, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeInternalNoteList(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingNoteList, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeInternalNoteCreate(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingNoteCreate, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizePersonalFilter(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingPersonalFilter)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeFinancialExport(ctx context.Context) (ManagementFinanceAuthorization, error) {
	userID, err := a.authorize(ctx, permissionBillingExport)
	if err != nil {
		return ManagementFinanceAuthorization{}, err
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID != userID || (current.DataScope != 1 && current.DataScope != 2) {
		return ManagementFinanceAuthorization{}, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return ManagementFinanceAuthorization{UserID: userID, DataScope: current.DataScope}, nil
}
func (a *managementFinanceAuthorizerImpl) AuthorizeRefundList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingRefundList)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeRefundDetail(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingRefundQuery, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeRefundReview(ctx context.Context, value string) (int, error) {
	return a.authorizeReference(ctx, permissionBillingRefundReview, value)
}
func (a *managementFinanceAuthorizerImpl) AuthorizeRefundPolicyWrite(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingRefundPolicyWrite)
}

func (a *managementFinanceAuthorizerImpl) authorizeReference(
	ctx context.Context,
	permission string,
	value string,
) (int, error) {
	if !validBusinessReference(value, 40) {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return a.authorize(ctx, permission)
}

func (a *managementFinanceAuthorizerImpl) authorize(ctx context.Context, permission string) (int, error) {
	if a == nil || !idccap.IsCatalogedAdminPermission(permission) {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive ||
		audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeDirectoryNotAccessible)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(
		ctx, authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeDirectoryNotAccessible)
	}
	if !grants[authz.PermissionKey(permission)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && !platformAdmin {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return current.UserID, nil
}

// AuthorizeBalanceAdjustmentCreate keeps gift and debit actions independent of receipt permissions.
func (a *managementFinanceAuthorizerImpl) AuthorizeBalanceAdjustmentCreate(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionBillingBalanceAdjustmentCreate)
}
