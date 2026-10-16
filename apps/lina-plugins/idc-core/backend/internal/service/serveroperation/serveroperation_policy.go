package serveroperation

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PermissionServerOperationPolicyRead  = "idc:admin:resource:server-operation-policy:read"
	PermissionServerOperationPolicyWrite = "idc:admin:resource:server-operation-policy:write"
)

var (
	CodePolicyNotAccessible = bizerr.MustDefine("IDC_SERVER_OPERATION_POLICY_NOT_ACCESSIBLE",
		"The server operation security policy is not accessible", gcode.CodeNotAuthorized)
	CodePolicyInvalid = bizerr.MustDefine("IDC_SERVER_OPERATION_POLICY_INVALID",
		"The server operation security policy request is invalid", gcode.CodeInvalidParameter)
	CodePolicyConflict = bizerr.MustDefine("IDC_SERVER_OPERATION_POLICY_CONFLICT",
		"The server operation security policy changed; reload it and try again", gcode.CodeInvalidOperation)
	CodePolicyUnavailable = bizerr.MustDefine("IDC_SERVER_OPERATION_POLICY_UNAVAILABLE",
		"The server operation security policy is temporarily unavailable", gcode.CodeInternalError)
)

type PolicyState struct {
	RequireHighRiskReauth bool      `orm:"require_high_risk_reauth"`
	AllowCustomerTransfer bool      `orm:"allow_customer_transfer"`
	Version               int64     `orm:"version"`
	UpdatedAt             time.Time `orm:"updated_at"`
}

type PolicyManagement interface {
	GetPolicy(context.Context) (*PolicyState, error)
	UpdatePolicy(context.Context, bool, bool, int64) (*PolicyState, error)
}

type policyManagementImpl struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	audit         idccap.BusinessAuditWriter
}

func NewPolicyManagement(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader,
	authorization authz.Service, audit idccap.BusinessAuditWriter) (PolicyManagement, error) {
	dependencies := []any{database, bizCtx, principalReader, authorization, audit}
	for _, dependency := range dependencies {
		if nilServerOperationDependency(dependency) {
			return nil, gerror.New("server operation policy requires database, context, principal, authorization, and audit")
		}
	}
	return &policyManagementImpl{database: database, bizCtx: bizCtx, principal: principalReader,
		authorization: authorization, audit: audit}, nil
}

func (s *policyManagementImpl) GetPolicy(ctx context.Context) (*PolicyState, error) {
	if _, err := s.authorizePolicy(ctx, PermissionServerOperationPolicyRead); err != nil {
		return nil, err
	}
	var state *PolicyState
	if err := s.database.GetScan(ctx, &state, `SELECT require_high_risk_reauth,allow_customer_transfer,version,updated_at
FROM plugin_idc_core_server_operation_policy WHERE tenant_id = 0`); err != nil {
		return nil, bizerr.WrapCode(err, CodePolicyUnavailable)
	}
	if state == nil {
		return nil, bizerr.NewCode(CodePolicyUnavailable)
	}
	state.UpdatedAt = state.UpdatedAt.UTC()
	return state, nil
}

func (s *policyManagementImpl) UpdatePolicy(ctx context.Context, requireReauth, allowTransfer bool,
	expectedVersion int64) (*PolicyState, error) {
	if expectedVersion < 1 {
		return nil, bizerr.NewCode(CodePolicyInvalid)
	}
	if _, err := s.authorizePolicy(ctx, PermissionServerOperationPolicyWrite); err != nil {
		return nil, err
	}
	var state *PolicyState
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var current *PolicyState
		if loadErr := s.database.GetScan(ctx, &current, `SELECT require_high_risk_reauth,allow_customer_transfer,version,updated_at
FROM plugin_idc_core_server_operation_policy WHERE tenant_id = 0 FOR UPDATE`); loadErr != nil {
			return loadErr
		}
		if current == nil {
			return bizerr.NewCode(CodePolicyUnavailable)
		}
		if current.Version != expectedVersion {
			return bizerr.NewCode(CodePolicyConflict)
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_server_operation_policy
SET require_high_risk_reauth=?,allow_customer_transfer=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND version=?`, requireReauth, allowTransfer, now, expectedVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodePolicyConflict)
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "server.operation.policy.update",
			IdempotencyKey:   strconv.FormatInt(expectedVersion+1, 10), OperationCode: "server.operation.policy.update",
			TargetType: "server_operation_policy", TargetNo: "tenant-0", ResultCode: idccap.BusinessAuditResultSucceeded,
			ReasonCode: "staff_security_policy_update", RiskCode: "high",
			RequiredPermission: PermissionServerOperationPolicyWrite,
			BeforeSnapshot: map[string]any{"requireHighRiskReauth": current.RequireHighRiskReauth,
				"allowCustomerTransfer": current.AllowCustomerTransfer, "version": current.Version},
			AfterSnapshot: map[string]any{"requireHighRiskReauth": requireReauth,
				"allowCustomerTransfer": allowTransfer, "version": expectedVersion + 1},
		}); auditErr != nil {
			return auditErr
		}
		state = &PolicyState{RequireHighRiskReauth: requireReauth, AllowCustomerTransfer: allowTransfer,
			Version: expectedVersion + 1, UpdatedAt: now.UTC()}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodePolicyUnavailable)
	}
	return state, nil
}

func (s *policyManagementImpl) authorizePolicy(ctx context.Context, permission string) (int, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 ||
		current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported ||
		(current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodePolicyNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodePolicyNotAccessible)
	}
	keys := []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard)}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodePolicyNotAccessible)
	}
	platformAdmin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || (!grants[keys[0]] && !grants[keys[1]] && !platformAdmin) {
		return 0, bizerr.NewCode(CodePolicyNotAccessible)
	}
	return current.UserID, nil
}

func nilServerOperationDependency(value any) bool {
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
