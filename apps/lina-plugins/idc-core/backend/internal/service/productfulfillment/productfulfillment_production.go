package productfulfillment

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/usercap"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type productionManagementAuthorizer struct {
	bizCtx    bizctxcap.Service
	principal principal.Reader
	authz     authz.Service
}

func NewManagementAuthorizer(bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service) (ManagementAuthorizer, error) {
	if bizCtx == nil || principalReader == nil || authorization == nil {
		return nil, gerror.New("product fulfillment authorizer requires bizctx, principal, and authz")
	}
	return &productionManagementAuthorizer{bizCtx: bizCtx, principal: principalReader, authz: authorization}, nil
}

func (a *productionManagementAuthorizer) authorize(ctx context.Context, permission string) (Authorization, error) {
	if a == nil || permissionForCatalog(permission) == "" {
		return Authorization{}, bizerr.NewCode(CodeNotAccessible)
	}
	current := a.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return Authorization{}, bizerr.NewCode(CodeNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return Authorization{}, bizerr.NewCode(CodeNotAccessible)
	}
	grants, err := a.authz.BatchHasPermissions(ctx, []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard)})
	if err != nil {
		return Authorization{}, bizerr.NewCode(CodeNotAccessible)
	}
	admin, err := a.authz.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || (!admin && !grants[authz.PermissionKey(permission)] && !grants[authz.PermissionKey(idccap.HostPermissionWildcard)]) {
		return Authorization{}, bizerr.NewCode(CodeNotAccessible)
	}
	scope := "platform_all"
	if current.DataScope == 2 {
		scope = "tenant_all"
	}
	return Authorization{ActorUserID: current.UserID, RequiredPermission: permission, DataScope: scope, AllCustomers: true}, nil
}

func permissionForCatalog(permission string) string {
	for _, candidate := range []string{PermissionTaskList, PermissionTaskQuery, PermissionReviewApprove, PermissionReviewReject, PermissionManualTakeover, PermissionManualConclude, PermissionPreparationRecover} {
		if permission == candidate {
			return candidate
		}
	}
	return ""
}

func (a *productionManagementAuthorizer) AuthorizeList(ctx context.Context) (Authorization, error) {
	return a.authorize(ctx, PermissionTaskList)
}
func (a *productionManagementAuthorizer) AuthorizeDetail(ctx context.Context, _ string) (Authorization, error) {
	return a.authorize(ctx, PermissionTaskQuery)
}
func (a *productionManagementAuthorizer) AuthorizeReviewApprove(ctx context.Context, _ string) (Authorization, error) {
	return a.authorize(ctx, PermissionReviewApprove)
}
func (a *productionManagementAuthorizer) AuthorizeReviewReject(ctx context.Context, _ string) (Authorization, error) {
	return a.authorize(ctx, PermissionReviewReject)
}
func (a *productionManagementAuthorizer) AuthorizeTakeover(ctx context.Context, _ string) (Authorization, error) {
	return a.authorize(ctx, PermissionManualTakeover)
}
func (a *productionManagementAuthorizer) AuthorizeManualConclude(ctx context.Context, _ string) (Authorization, error) {
	return a.authorize(ctx, PermissionManualConclude)
}

// AuthorizeRecovery keeps recovery separate from takeover and manual completion.
func (a *productionManagementAuthorizer) AuthorizeRecovery(ctx context.Context, _ string) (Authorization, error) {
	return a.authorize(ctx, PermissionPreparationRecover)
}

type productionActorDirectory struct{ users usercap.Service }

func NewActorDirectory(users usercap.Service) (ActorDirectory, error) {
	if users == nil {
		return nil, gerror.New("product fulfillment actor directory requires users")
	}
	return &productionActorDirectory{users: users}, nil
}

func (d *productionActorDirectory) DisplayNames(ctx context.Context, ids []int) (map[int]string, error) {
	if d == nil || d.users == nil || len(ids) == 0 || len(ids) > 100 {
		return nil, gerror.New("product fulfillment actor batch is invalid")
	}
	keys := make([]usercap.UserID, len(ids))
	for index, id := range ids {
		if id <= 0 {
			return nil, gerror.New("product fulfillment actor is invalid")
		}
		keys[index] = usercap.UserID(strconv.Itoa(id))
	}
	batch, err := d.users.BatchGet(ctx, keys)
	if err != nil || batch == nil || len(batch.Items) != len(keys) {
		return nil, gerror.New("product fulfillment actors are unavailable")
	}
	result := make(map[int]string, len(ids))
	for index, key := range keys {
		user := batch.Items[key]
		if user == nil {
			return nil, gerror.New("product fulfillment actor is unavailable")
		}
		name := strings.TrimSpace(user.Nickname)
		if name == "" {
			name = strings.TrimSpace(user.Username)
		}
		if name == "" {
			return nil, gerror.New("product fulfillment actor name is unavailable")
		}
		result[ids[index]] = name
	}
	return result, nil
}

type productionOwnerGateValidator struct {
	database interface {
		GetScan(context.Context, any, string, ...any) error
	}
}

func NewOwnerGateValidator(database interface {
	GetScan(context.Context, any, string, ...any) error
}) (OwnerGateValidator, error) {
	if database == nil {
		return nil, gerror.New("product fulfillment owner gate requires database")
	}
	return &productionOwnerGateValidator{database: database}, nil
}

func (v *productionOwnerGateValidator) EvaluateFulfillmentRelease(ctx context.Context, orderNo string, itemNos []string) (map[string]bool, error) {
	if v == nil || len(itemNos) == 0 || len(itemNos) > 100 {
		return nil, gerror.New("product fulfillment owner gate input is invalid")
	}
	var authority *struct {
		RiskDecision string `orm:"risk_decision_code"`

		ItemCount int
	}
	if err := v.database.GetScan(ctx, &authority, `SELECT orders.risk_decision_code,count(item.order_item_no)::int AS item_count FROM plugin_idc_core_commerce_order orders JOIN plugin_idc_core_commerce_order_item item ON item.tenant_id=orders.tenant_id AND item.order_no=orders.order_no AND item.order_item_no IN(SELECT jsonb_array_elements_text(?::jsonb)) WHERE orders.tenant_id=0 AND orders.order_no=? GROUP BY orders.risk_decision_code LIMIT 1`, mustJSON(itemNos), orderNo); err != nil || authority == nil || authority.ItemCount != len(itemNos) {
		return nil, gerror.New("product fulfillment owner gates are unavailable")
	}
	ready := authority.RiskDecision == "approved"
	result := make(map[string]bool, len(itemNos))
	for _, itemNo := range itemNos {
		result[itemNo] = ready
	}
	return result, nil
}

func mustJSON(values []string) string {
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(raw)
}
