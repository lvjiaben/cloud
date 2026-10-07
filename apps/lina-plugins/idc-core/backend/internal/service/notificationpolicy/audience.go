package notificationpolicy

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type currentAudienceReader struct {
	database  gdb.DB
	bizCtx    bizctxcap.Service
	principal principal.Reader
}

var _ notificationcap.CurrentAudience = (*currentAudienceReader)(nil)

func NewCurrentAudience(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader) (notificationcap.CurrentAudience, error) {
	if database == nil || bizCtx == nil || principalReader == nil {
		return nil, gerror.New("notification current audience requires database and current identity")
	}
	return &currentAudienceReader{database: database, bizCtx: bizCtx, principal: principalReader}, nil
}

func (r *currentAudienceReader) Current(ctx context.Context) (notificationcap.CurrentAudienceProjection, error) {
	current := r.bizCtx.Current(ctx)
	// Customer self-service identities intentionally have no staff data scope.
	// DataScopeUnsupported therefore cannot be used as a customer authorization
	// failure; direct identity, single-merchant scope and the bound active
	// customer audience remain the authoritative checks below.
	if !validCurrentCustomerContext(current) {
		return notificationcap.CurrentAudienceProjection{}, bizerr.WrapCode(
			gerror.New("current customer notification context is invalid"),
			CodePreferenceNotAccessible,
		)
	}
	audience, err := r.principal.Current(ctx)
	if err != nil {
		return notificationcap.CurrentAudienceProjection{}, bizerr.WrapCode(err, CodePreferenceNotAccessible)
	}
	if audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return notificationcap.CurrentAudienceProjection{}, bizerr.WrapCode(
			gerror.New("current customer notification audience binding is invalid"),
			CodePreferenceNotAccessible,
		)
	}
	if !notificationIdentifierPattern.MatchString(audience.CustomerNo) {
		return notificationcap.CurrentAudienceProjection{}, bizerr.WrapCode(
			gerror.New("current customer notification number is invalid"),
			CodePreferenceNotAccessible,
		)
	}
	var rows []struct {
		CustomerNo string `orm:"customer_no"`
		Version    int64  `orm:"version"`
	}
	if err = r.database.GetScan(ctx, &rows, `SELECT customer_no,version FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_no=? AND status_code='active' LIMIT 1`, audience.CustomerNo); err != nil {
		return notificationcap.CurrentAudienceProjection{}, bizerr.WrapCode(err, CodePreferenceUnavailable)
	}
	if len(rows) != 1 || rows[0].CustomerNo != audience.CustomerNo || rows[0].Version <= 0 {
		return notificationcap.CurrentAudienceProjection{}, bizerr.WrapCode(
			gerror.New("active customer notification subject is unavailable"),
			CodePreferenceNotAccessible,
		)
	}
	return notificationcap.CurrentAudienceProjection{CustomerNo: rows[0].CustomerNo, CustomerVersion: rows[0].Version}, nil
}

func validCurrentCustomerContext(current bizctxcap.CurrentContext) bool {
	directCustomer := current.ActingUserID == 0 && !current.IsImpersonation && !current.ActingAsTenant
	delegatedCustomer := current.IsImpersonation && current.ActingUserID > 0 &&
		current.ActingUserID != current.UserID && !current.ActingAsTenant
	return current.UserID > 0 && current.TenantID == 0 && (directCustomer || delegatedCustomer)
}
