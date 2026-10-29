// Package entitlementaccess owns the generic current-customer service gate.
package entitlementaccess

import (
	"context"
	"regexp"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

var entitlementPattern = regexp.MustCompile(`^ent_[0-9a-f]{32}$`)

type Service struct {
	customerauthcap.Reader
	database     gdb.DB
	confirmation reauth.SessionConfirmationService
}

var _ customerauthcap.EntitlementAccess = (*Service)(nil)
var _ customerauthcap.QueuedEntitlementAccess = (*Service)(nil)
var _ customerauthcap.SessionConfirmationAccess = (*Service)(nil)

func New(database gdb.DB, customers customerauthcap.Reader) (*Service, error) {
	if database == nil || customers == nil {
		return nil, gerror.New("customer entitlement dependencies are required")
	}
	return &Service{Reader: customers, database: database}, nil
}

func (s *Service) ReadCurrentEntitlement(ctx context.Context, number string) (*customerauthcap.Entitlement, error) {
	return s.read(ctx, number, false)
}

func (s *Service) WithCurrentEntitlement(ctx context.Context, number string, action func(context.Context, *customerauthcap.Entitlement) error) error {
	if action == nil || s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("customer entitlement admission is unavailable")
	}
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		item, err := s.read(ctx, number, true)
		if err != nil || !item.CanOperate {
			return gerror.New("customer entitlement is unavailable")
		}
		return action(ctx, item)
	})
}

func (s *Service) read(ctx context.Context, number string, lock bool) (*customerauthcap.Entitlement, error) {
	if s == nil || s.database == nil || s.Reader == nil || ctx == nil || ctx.Err() != nil || !entitlementPattern.MatchString(number) {
		return nil, gerror.New("customer entitlement is unavailable")
	}
	subject, err := s.CurrentCustomer(ctx)
	if err != nil || subject == nil || subject.CustomerNo == "" {
		return nil, gerror.New("customer entitlement is unavailable")
	}
	return s.readOwned(ctx, number, subject.CustomerNo, lock)
}

func (s *Service) readOwned(ctx context.Context, number, customer string, lock bool) (*customerauthcap.Entitlement, error) {
	query := `SELECT e.entitlement_no,e.customer_no,e.order_no,e.status_code AS status,
i.product_name_zh_cn,i.product_name_en_us,e.period_unit_code AS period_unit,e.period_count,
e.started_at,e.expires_at,e.version,
(e.status_code='active' AND (e.expires_at IS NULL OR e.expires_at>CURRENT_TIMESTAMP)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock l
 WHERE l.tenant_id=e.tenant_id AND l.entitlement_no=e.entitlement_no AND l.released_at IS NULL)) AS can_operate
FROM plugin_idc_core_service_entitlement e
JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.intent_no=e.source_intent_no
WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.entitlement_owner_id='idc-resource'`
	args := []any{number}
	if customer != "" {
		query += ` AND e.customer_no=?`
		args = append(args, customer)
	}
	if lock {
		query += ` FOR UPDATE OF e`
	}
	var item *customerauthcap.Entitlement
	if err := s.database.GetScan(ctx, &item, query, args...); err != nil || item == nil {
		return nil, gerror.New("customer entitlement is unavailable")
	}
	return item, nil
}

// WithQueuedEntitlement locks the current core fact without requiring a browser
// principal. Only a task frozen after customer admission may supply its version.
func (s *Service) WithQueuedEntitlement(ctx context.Context, number string, expectedVersion int64, action func(context.Context, *customerauthcap.Entitlement) error) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || !entitlementPattern.MatchString(number) || expectedVersion < 1 || action == nil {
		return gerror.New("queued customer entitlement is unavailable")
	}
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		item, err := s.readOwned(ctx, number, "", true)
		if err != nil || !item.CanOperate || item.Version != expectedVersion {
			return gerror.New("queued customer entitlement is unavailable")
		}
		return action(ctx, item)
	})
}
