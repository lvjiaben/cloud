// Package renewalnotice delivers accessory-renewal attention to the actual
// signed-in staff member, after current permission and merchant-scope checks.
package renewalnotice

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

// Permission is the existing domain grant for all accessory-renewal work items.
const Permission = "idc:admin:resource:accessory-renewal:handle"

// Item is a safe, local work-item projection; it contains no resource secrets.
type Item struct {
	WorkItemNo string
	InvoiceNo  string
	CustomerNo string
	SubjectNo  string
	ReasonCode string
	CreatedAt  time.Time
}

// Page contains the exact filtered total and one bounded page.
type Page struct {
	Items []Item
	Total int
}

// Synced reports a durable event intent, never a claim that delivery succeeded.
type Synced struct {
	Total   int
	EventNo string
}

// Service owns current-staff work visibility and durable notification sync.
type Service interface {
	// List reads one bounded page after current permission, audience and supported
	// data-scope checks. It rejects delegated/customer identities and never writes.
	List(ctx context.Context, page, pageSize int) (*Page, error)
	// Sync creates one idempotent summary intent for the current open-work set and
	// actual staff user; it accepts only supported locales and no recipient ID.
	// Empty work produces no event. EventNo is enqueue evidence, not inbox delivery.
	Sync(ctx context.Context, locale string) (*Synced, error)
}

type serviceImpl struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	events        notificationcap.EventSink
	inbox         inboxcap.CurrentSyncService
}

var _ Service = (*serviceImpl)(nil)

// New requires the shared database, current identity, authorization and one
// core-bound outbox sink; none are recreated on the request path.
func New(database gdb.DB, bizCtx bizctxcap.Service, audience principal.Reader,
	authorization authz.Service, events notificationcap.EventSink, inbox inboxcap.CurrentSyncService) (Service, error) {
	if database == nil || bizCtx == nil || audience == nil || authorization == nil || events == nil || inbox == nil {
		return nil, gerror.New("renewal notifications require database, current staff authority, event sink and inbox sync")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, principal: audience, authorization: authorization, events: events, inbox: inbox}, nil
}
