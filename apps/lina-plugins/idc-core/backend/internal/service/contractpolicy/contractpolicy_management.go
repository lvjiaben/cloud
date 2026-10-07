package contractpolicy

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PermissionEnterpriseWrite = "idc:admin:customer:contract-policy:write"
	PermissionProductWrite    = "idc:admin:catalog:contract-policy:write"
	PermissionOrderWrite      = "idc:admin:commerce:contract-policy:write"
)

var (
	CodeManagementInvalid = bizerr.MustDefine(
		"IDC_CONTRACT_POLICY_INVALID",
		"The contract rule request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeManagementNotAccessible = bizerr.MustDefine(
		"IDC_CONTRACT_POLICY_NOT_ACCESSIBLE",
		"The contract rule is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeManagementConflict = bizerr.MustDefine(
		"IDC_CONTRACT_POLICY_CONFLICT",
		"The contract rule changed; reload it and try again",
		gcode.CodeInvalidOperation,
	)
	CodeManagementOptionalContractLocked = bizerr.MustDefine(
		"IDC_CONTRACT_POLICY_OPTIONAL_CONTRACT_LOCKED",
		"This order already has an optional contract and must remain a no-contract order",
		gcode.CodeInvalidOperation,
	)
	CodeManagementTemplateUnavailable = bizerr.MustDefine(
		"IDC_CONTRACT_POLICY_TEMPLATE_UNAVAILABLE",
		"The selected contract template is not enabled or does not match this setting",
		gcode.CodeInvalidOperation,
	)
	CodeManagementTemplateConflict = bizerr.MustDefine(
		"IDC_CONTRACT_POLICY_TEMPLATE_CONFLICT",
		"The products require different contract templates; choose one order template before payment",
		gcode.CodeInvalidOperation,
	)
	CodeManagementUnavailable = bizerr.MustDefine(
		"IDC_CONTRACT_POLICY_UNAVAILABLE",
		"Contract rule management is temporarily unavailable",
		gcode.CodeInternalError,
	)
)

type TemplateListInput struct {
	Level      contracttemplatecap.SelectionLevel
	TargetNo   string
	CustomerNo string
}

type RuleUpdateInput struct {
	TargetNo        string
	ExpectedVersion int64
	Rule            Rule
}

type RuleState struct {
	Rule    Rule
	Version int64
}

type OrderState struct {
	RuleState
	RequiresContract bool
	DecisionSource   DecisionSource
	Template         contracttemplatecap.Template
}

type ManagementService interface {
	ListTemplates(context.Context, TemplateListInput) ([]contracttemplatecap.Template, error)
	UpdateEnterpriseRule(context.Context, RuleUpdateInput) (*RuleState, error)
	UpdateProductRule(context.Context, RuleUpdateInput) (*RuleState, error)
	UpdateOrderRule(context.Context, RuleUpdateInput) (*OrderState, error)
}

type managementService struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	audit         idccap.BusinessAuditWriter
	templates     contracttemplatecap.Source
	resolver      Resolver
}

var _ ManagementService = (*managementService)(nil)

func NewManagementService(
	database gdb.DB,
	bizCtx bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
	audit idccap.BusinessAuditWriter,
	templates contracttemplatecap.Source,
) (ManagementService, error) {
	if database == nil || isNilManagementDependency(bizCtx) || isNilManagementDependency(principalReader) ||
		isNilManagementDependency(authorization) || isNilManagementDependency(audit) {
		return nil, gerror.New("contract policy management requires database, context, principal, authorization, and audit services")
	}
	return &managementService{
		database: database, bizCtx: bizCtx, principal: principalReader,
		authorization: authorization, audit: audit, templates: templates,
		resolver: NewResolver(templates),
	}, nil
}

func (s *managementService) ListTemplates(
	ctx context.Context,
	input TemplateListInput,
) ([]contracttemplatecap.Template, error) {
	selection, err := s.templateSelection(ctx, input)
	if err != nil {
		return nil, err
	}
	if s.templates == nil {
		return nil, bizerr.NewCode(CodeManagementTemplateUnavailable)
	}
	items, err := s.templates.ListAvailable(ctx, selection)
	if err != nil {
		return nil, mapManagementTemplateError(err)
	}
	return items, nil
}

func (s *managementService) templateSelection(
	ctx context.Context,
	input TemplateListInput,
) (contracttemplatecap.Selection, error) {
	input.TargetNo = strings.TrimSpace(input.TargetNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	permission, err := permissionForLevel(input.Level)
	if err != nil {
		return contracttemplatecap.Selection{}, bizerr.NewCode(CodeManagementInvalid)
	}
	if _, err = s.authorize(ctx, permission); err != nil {
		return contracttemplatecap.Selection{}, err
	}
	switch input.Level {
	case contracttemplatecap.SelectionLevelProduct:
		if input.TargetNo != "" || input.CustomerNo != "" {
			return contracttemplatecap.Selection{}, bizerr.NewCode(CodeManagementInvalid)
		}
		return contracttemplatecap.Selection{Level: input.Level}, nil
	case contracttemplatecap.SelectionLevelEnterprise:
		if input.CustomerNo != "" {
			return contracttemplatecap.Selection{}, bizerr.NewCode(CodeManagementInvalid)
		}
		customerType, loadErr := s.loadCustomerType(ctx, input.TargetNo)
		if loadErr != nil || customerType != contracttemplatecap.CustomerTypeEnterprise {
			return contracttemplatecap.Selection{}, bizerr.NewCode(CodeManagementNotAccessible)
		}
		return contracttemplatecap.Selection{Level: input.Level, CustomerType: customerType}, nil
	case contracttemplatecap.SelectionLevelOrder:
		if (input.TargetNo == "") == (input.CustomerNo == "") {
			return contracttemplatecap.Selection{}, bizerr.NewCode(CodeManagementInvalid)
		}
		var customerType contracttemplatecap.CustomerType
		var loadErr error
		if input.CustomerNo != "" {
			customerType, loadErr = s.loadCustomerType(ctx, input.CustomerNo)
		} else {
			customerType, loadErr = s.loadOrderCustomerType(ctx, input.TargetNo)
		}
		if loadErr != nil {
			return contracttemplatecap.Selection{}, loadErr
		}
		return contracttemplatecap.Selection{Level: input.Level, CustomerType: customerType}, nil
	default:
		return contracttemplatecap.Selection{}, bizerr.NewCode(CodeManagementInvalid)
	}
}

func (s *managementService) UpdateEnterpriseRule(
	ctx context.Context,
	input RuleUpdateInput,
) (*RuleState, error) {
	actor, err := s.authorize(ctx, PermissionEnterpriseWrite)
	if err != nil {
		return nil, err
	}
	input.TargetNo = strings.TrimSpace(input.TargetNo)
	rule, err := NormalizeRule(input.Rule, false)
	if err != nil || input.TargetNo == "" || input.ExpectedVersion < 1 {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	if err = s.validateSelectedTemplate(ctx, contracttemplatecap.Selection{
		Level:        contracttemplatecap.SelectionLevelEnterprise,
		CustomerType: contracttemplatecap.CustomerTypeEnterprise,
	}, rule); err != nil {
		return nil, err
	}
	return s.updateEnterpriseRule(ctx, actor, input, rule)
}

func (s *managementService) updateEnterpriseRule(
	ctx context.Context,
	actor int,
	input RuleUpdateInput,
	rule Rule,
) (*RuleState, error) {
	var state *RuleState
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var customer *entity.CustomerSubject
		if err := tx.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{
			TenantId: idccap.DefaultTenantID, CustomerNo: input.TargetNo,
		}).LockUpdate().Scan(&customer); err != nil {
			return gerror.Wrap(err, "lock enterprise contract policy customer")
		}
		if customer == nil || customer.CustomerTypeCode != string(contracttemplatecap.CustomerTypeEnterprise) ||
			customer.AnonymizedAt != nil {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if customer.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeManagementConflict)
		}
		now, err := managementDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		data := map[string]any{
			"enterprise_contract_policy_code": string(rule.Policy),
			"enterprise_contract_template_no": nullableTemplateNo(rule),
			"version":                         customer.Version + 1, "updated_by": actor, "updated_at": now,
		}
		result, err := tx.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{
			TenantId: idccap.DefaultTenantID, CustomerNo: customer.CustomerNo, Version: customer.Version,
		}).Data(data).Update()
		if err != nil {
			return gerror.Wrap(err, "update enterprise contract policy")
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "contract-policy.enterprise.update",
			IdempotencyKey:   customer.CustomerNo + ":" + strconv.FormatInt(customer.Version+1, 10),
			OperationCode:    "contract-policy.enterprise.update", TargetType: "customer",
			TargetNo: customer.CustomerNo, CustomerNo: customer.CustomerNo,
			ResultCode: "updated", ReasonCode: "staff_contract_policy_update",
			RiskCode: "medium", RequiredPermission: PermissionEnterpriseWrite,
			BeforeSnapshot: map[string]any{
				"policyCode": customer.EnterpriseContractPolicyCode,
				"templateNo": customer.EnterpriseContractTemplateNo,
				"version":    customer.Version,
			},
			AfterSnapshot: map[string]any{
				"policyCode": rule.Policy, "templateNo": rule.TemplateNo,
				"version": customer.Version + 1,
			},
		}); err != nil {
			return err
		}
		state = &RuleState{Rule: rule, Version: customer.Version + 1}
		return nil
	})
	if err != nil {
		return nil, mapManagementWriteError(err)
	}
	return state, nil
}

func (s *managementService) UpdateProductRule(
	ctx context.Context,
	input RuleUpdateInput,
) (*RuleState, error) {
	actor, err := s.authorize(ctx, PermissionProductWrite)
	if err != nil {
		return nil, err
	}
	input.TargetNo = strings.TrimSpace(input.TargetNo)
	rule, err := NormalizeRule(input.Rule, true)
	if err != nil || input.TargetNo == "" || input.ExpectedVersion < 1 {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	if err = s.validateSelectedTemplate(ctx, contracttemplatecap.Selection{
		Level: contracttemplatecap.SelectionLevelProduct,
	}, rule); err != nil {
		return nil, err
	}
	var state *RuleState
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var version *entity.CatalogProductVersion
		if loadErr := tx.Model(dao.CatalogProductVersion.Table()).Ctx(ctx).Where(do.CatalogProductVersion{
			TenantId: idccap.DefaultTenantID, VersionNo: input.TargetNo,
		}).LockUpdate().Scan(&version); loadErr != nil {
			return gerror.Wrap(loadErr, "lock product contract policy")
		}
		if version == nil {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if version.StatusCode != "draft" || version.ContractPolicyVersion != input.ExpectedVersion {
			return bizerr.NewCode(CodeManagementConflict)
		}
		data := map[string]any{
			"contract_policy_code":    string(rule.Policy),
			"contract_template_no":    nullableTemplateNo(rule),
			"requires_contract":       rule.Policy == PolicyRequired,
			"contract_policy_version": version.ContractPolicyVersion + 1,
		}
		result, updateErr := tx.Model(dao.CatalogProductVersion.Table()).Ctx(ctx).Where(do.CatalogProductVersion{
			TenantId: idccap.DefaultTenantID, VersionNo: version.VersionNo,
			ContractPolicyVersion: version.ContractPolicyVersion,
		}).Data(data).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "update product contract policy")
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if updateErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "contract-policy.product.update",
			IdempotencyKey:   version.VersionNo + ":" + strconv.FormatInt(version.ContractPolicyVersion+1, 10),
			OperationCode:    "contract-policy.product.update", TargetType: "catalog_product_version",
			TargetNo: version.VersionNo, ResultCode: "updated",
			ReasonCode: "staff_contract_policy_update", RiskCode: "medium",
			RequiredPermission: PermissionProductWrite,
			BeforeSnapshot: map[string]any{
				"policyCode": version.ContractPolicyCode, "templateNo": version.ContractTemplateNo,
				"version": version.ContractPolicyVersion,
			},
			AfterSnapshot: map[string]any{
				"policyCode": rule.Policy, "templateNo": rule.TemplateNo,
				"version": version.ContractPolicyVersion + 1,
			},
		}); updateErr != nil {
			return updateErr
		}
		state = &RuleState{Rule: rule, Version: version.ContractPolicyVersion + 1}
		return nil
	})
	if err != nil {
		return nil, mapManagementWriteError(err)
	}
	_ = actor
	return state, nil
}

func (s *managementService) UpdateOrderRule(
	ctx context.Context,
	input RuleUpdateInput,
) (*OrderState, error) {
	actor, err := s.authorize(ctx, PermissionOrderWrite)
	if err != nil {
		return nil, err
	}
	input.TargetNo = strings.TrimSpace(input.TargetNo)
	rule, err := NormalizeRule(input.Rule, true)
	if err != nil || input.TargetNo == "" || input.ExpectedVersion < 1 {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	var state *OrderState
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		contractLockKey, lockKeyErr := contracttemplatecap.OrderContractLockKey(input.TargetNo)
		if lockKeyErr != nil {
			return bizerr.NewCode(CodeManagementInvalid)
		}
		if _, lockErr := tx.GetValue(
			"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", contractLockKey,
		); lockErr != nil {
			return gerror.Wrap(lockErr, "lock order contract decision")
		}
		var order *entity.CommerceOrder
		if loadErr := tx.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{
			TenantId: idccap.DefaultTenantID, OrderNo: input.TargetNo,
		}).LockUpdate().Scan(&order); loadErr != nil {
			return gerror.Wrap(loadErr, "lock order contract policy")
		}
		if order == nil {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if order.Version != input.ExpectedVersion || order.PaidAt != nil ||
			(order.StatusCode != "pending_payment" && order.StatusCode != "payment_processing" &&
				order.StatusCode != "manual_review") {
			return bizerr.NewCode(CodeManagementConflict)
		}
		customerType, loadErr := loadLockedOrderCustomerType(ctx, tx, order.CustomerNo)
		if loadErr != nil {
			return loadErr
		}
		var items []entity.CommerceOrderItem
		if loadErr = tx.Model(dao.CommerceOrderItem.Table()).Ctx(ctx).Where(do.CommerceOrderItem{
			TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo,
		}).OrderAsc(dao.CommerceOrderItem.Columns().LineNumber).
			Limit(contracttemplatecap.MaximumTemplates + 1).Scan(&items); loadErr != nil {
			return gerror.Wrap(loadErr, "load order contract policy items")
		}
		if len(items) == 0 || len(items) > contracttemplatecap.MaximumTemplates {
			return bizerr.NewCode(CodeManagementUnavailable)
		}
		productRules := make([]Rule, 0, len(items))
		for _, item := range items {
			productRules = append(productRules, Rule{
				Policy: Policy(item.ContractPolicyCode), TemplateNo: item.ContractTemplateNo,
			})
		}
		decision, resolveErr := s.resolver.Resolve(ctx, ResolveInput{
			CustomerType: customerType,
			EnterpriseRule: Rule{
				Policy:     Policy(order.EnterpriseContractPolicyCode),
				TemplateNo: order.EnterpriseContractTemplateNo,
			},
			ProductRules: productRules, OrderOverride: rule,
		})
		if resolveErr != nil {
			return mapManagementResolveError(resolveErr)
		}
		if decision.RequiresContract {
			if mutableErr := s.ensureOrderCanRequireContract(ctx, order.OrderNo); mutableErr != nil {
				return mutableErr
			}
		}
		now, nowErr := managementDatabaseNow(ctx, tx)
		if nowErr != nil {
			return nowErr
		}
		data := map[string]any{
			"contract_override_policy_code": string(rule.Policy),
			"contract_override_template_no": nullableTemplateNo(rule),
			"contract_decision_source_code": string(decision.Source),
			"requires_contract":             decision.RequiresContract,
			"contract_template_no":          nil, "contract_template_version_no": nil,
			"contract_template_name": nil, "contract_language_code": nil,
			"contract_scope_code": nil, "contract_signing_order_code": nil,
			"version": order.Version + 1, "updated_at": now,
		}
		if decision.RequiresContract {
			data["contract_template_no"] = decision.Template.TemplateNo
			data["contract_template_version_no"] = decision.Template.VersionNo
			data["contract_template_name"] = decision.Template.Name
			data["contract_language_code"] = decision.Template.LanguageCode
			data["contract_scope_code"] = decision.Template.ScopeCode
			data["contract_signing_order_code"] = decision.Template.SigningOrderCode
		}
		result, updateErr := tx.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{
			TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo, Version: order.Version,
		}).Data(data).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "update order contract policy")
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if updateErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "contract-policy.order.update",
			IdempotencyKey:   order.OrderNo + ":" + strconv.FormatInt(order.Version+1, 10),
			OperationCode:    "contract-policy.order.update", TargetType: "commerce_order",
			TargetNo: order.OrderNo, CustomerNo: order.CustomerNo,
			ResultCode: "updated", ReasonCode: "staff_contract_policy_update",
			RiskCode: "high", RequiredPermission: PermissionOrderWrite,
			BeforeSnapshot: map[string]any{
				"policyCode":       order.ContractOverridePolicyCode,
				"templateNo":       order.ContractOverrideTemplateNo,
				"requiresContract": order.RequiresContract, "version": order.Version,
			},
			AfterSnapshot: map[string]any{
				"policyCode": rule.Policy, "templateNo": rule.TemplateNo,
				"requiresContract":   decision.RequiresContract,
				"resolvedTemplateNo": decision.Template.TemplateNo,
				"version":            order.Version + 1,
			},
		}); updateErr != nil {
			return updateErr
		}
		state = &OrderState{
			RuleState:        RuleState{Rule: rule, Version: order.Version + 1},
			RequiresContract: decision.RequiresContract,
			DecisionSource:   decision.Source, Template: decision.Template,
		}
		return nil
	})
	if err != nil {
		return nil, mapManagementWriteError(err)
	}
	_ = actor
	return state, nil
}

func (s *managementService) ensureOrderCanRequireContract(
	ctx context.Context,
	orderNo string,
) error {
	if s == nil || s.templates == nil {
		return bizerr.NewCode(CodeManagementTemplateUnavailable)
	}
	contractExists, err := s.templates.OrderContractExists(ctx, orderNo)
	if err != nil {
		return mapManagementTemplateError(err)
	}
	if contractExists {
		return bizerr.NewCode(CodeManagementOptionalContractLocked)
	}
	return nil
}

func (s *managementService) validateSelectedTemplate(
	ctx context.Context,
	selection contracttemplatecap.Selection,
	rule Rule,
) error {
	if rule.Policy != PolicyRequired {
		return nil
	}
	if s.templates == nil {
		return bizerr.NewCode(CodeManagementTemplateUnavailable)
	}
	resolved, err := s.templates.ResolveAvailable(ctx, selection, []string{rule.TemplateNo})
	if err != nil || len(resolved) != 1 || resolved[0].TemplateNo != rule.TemplateNo {
		if err == nil {
			err = contracttemplatecap.ErrTemplateUnavailable
		}
		return mapManagementTemplateError(err)
	}
	return nil
}

func (s *managementService) authorize(ctx context.Context, permission string) (int, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive ||
		audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeManagementNotAccessible)
	}
	platformAdmin, err := s.authorization.IsPlatformAdmin(
		ctx, authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil || (!grants[keys[0]] && !grants[keys[1]] && !platformAdmin) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return current.UserID, nil
}

func (s *managementService) loadCustomerType(
	ctx context.Context,
	customerNo string,
) (contracttemplatecap.CustomerType, error) {
	if customerNo == "" {
		return "", bizerr.NewCode(CodeManagementInvalid)
	}
	var row struct {
		CustomerTypeCode string `orm:"customer_type_code"`
	}
	if err := s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).
		Fields(dao.CustomerSubject.Columns().CustomerTypeCode).
		Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID, CustomerNo: customerNo}).
		Scan(&row); err != nil {
		return "", bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	customerType := contracttemplatecap.CustomerType(row.CustomerTypeCode)
	if contracttemplatecap.ValidateCustomerType(customerType) != nil {
		return "", bizerr.NewCode(CodeManagementNotAccessible)
	}
	return customerType, nil
}

func (s *managementService) loadOrderCustomerType(
	ctx context.Context,
	orderNo string,
) (contracttemplatecap.CustomerType, error) {
	if orderNo == "" {
		return "", bizerr.NewCode(CodeManagementInvalid)
	}
	var row struct {
		CustomerTypeCode string `orm:"customer_type_code"`
	}
	if err := s.database.GetScan(ctx, &row, `
SELECT c.customer_type_code
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_customer_subject c
  ON c.tenant_id = o.tenant_id AND c.customer_no = o.customer_no
WHERE o.tenant_id = ? AND o.order_no = ?`, idccap.DefaultTenantID, orderNo); err != nil {
		return "", bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	customerType := contracttemplatecap.CustomerType(row.CustomerTypeCode)
	if contracttemplatecap.ValidateCustomerType(customerType) != nil {
		return "", bizerr.NewCode(CodeManagementNotAccessible)
	}
	return customerType, nil
}

func loadLockedOrderCustomerType(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
) (contracttemplatecap.CustomerType, error) {
	var customer *entity.CustomerSubject
	if err := tx.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
	}).LockUpdate().Scan(&customer); err != nil {
		return "", gerror.Wrap(err, "lock order contract policy customer")
	}
	if customer == nil {
		return "", bizerr.NewCode(CodeManagementNotAccessible)
	}
	customerType := contracttemplatecap.CustomerType(customer.CustomerTypeCode)
	if contracttemplatecap.ValidateCustomerType(customerType) != nil {
		return "", bizerr.NewCode(CodeManagementNotAccessible)
	}
	return customerType, nil
}

func permissionForLevel(level contracttemplatecap.SelectionLevel) (string, error) {
	switch level {
	case contracttemplatecap.SelectionLevelProduct:
		return PermissionProductWrite, nil
	case contracttemplatecap.SelectionLevelEnterprise:
		return PermissionEnterpriseWrite, nil
	case contracttemplatecap.SelectionLevelOrder:
		return PermissionOrderWrite, nil
	default:
		return "", ErrInvalidRule
	}
}

func nullableTemplateNo(rule Rule) any {
	if rule.TemplateNo == "" {
		return nil
	}
	return rule.TemplateNo
}

func managementDatabaseNow(ctx context.Context, tx gdb.TX) (time.Time, error) {
	value, err := tx.Ctx(ctx).GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "load contract policy database time")
	}
	return value.Time(), nil
}

func mapManagementResolveError(err error) error {
	if errors.Is(err, ErrTemplateConflict) {
		return bizerr.WrapCode(err, CodeManagementTemplateConflict)
	}
	if errors.Is(err, contracttemplatecap.ErrSourceUnavailable) ||
		errors.Is(err, contracttemplatecap.ErrTemplateUnavailable) {
		return bizerr.WrapCode(err, CodeManagementTemplateUnavailable)
	}
	return bizerr.WrapCode(err, CodeManagementInvalid)
}

func mapManagementTemplateError(err error) error {
	if errors.Is(err, contracttemplatecap.ErrSourceUnavailable) ||
		errors.Is(err, contracttemplatecap.ErrTemplateUnavailable) {
		return bizerr.WrapCode(err, CodeManagementTemplateUnavailable)
	}
	return bizerr.WrapCode(err, CodeManagementUnavailable)
}

func mapManagementWriteError(err error) error {
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeManagementUnavailable)
}

func isNilManagementDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
