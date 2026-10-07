package notificationpolicy

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/contenthub"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

type businessObjectAccess struct {
	database  gdb.DB
	marketing marketing.ManagementDirectoryReader
	support   supportdesk.Service
	content   contenthub.Service
}

var _ notificationcap.BusinessObjectAccess = (*businessObjectAccess)(nil)

func NewBusinessObjectAccess(
	database gdb.DB,
	marketingReader marketing.ManagementDirectoryReader,
	supportService supportdesk.Service,
	contentService contenthub.Service,
) (notificationcap.BusinessObjectAccess, error) {
	if database == nil || marketingReader == nil || supportService == nil || contentService == nil {
		return nil, gerror.New("notification business object access requires authoritative readers")
	}
	return &businessObjectAccess{database: database, marketing: marketingReader, support: supportService, content: contentService}, nil
}

func (s *businessObjectAccess) AuthorizeCurrent(
	ctx context.Context,
	request notificationcap.BusinessObjectRequest,
) (notificationcap.BusinessObjectProjection, error) {
	request.ObjectNo = strings.TrimSpace(request.ObjectNo)
	if !notificationIdentifierPattern.MatchString(request.ObjectNo) {
		return notificationcap.BusinessObjectProjection{}, gerror.New("notification business object is unavailable")
	}
	switch request.Type {
	case notificationcap.BusinessObjectBilling:
		return s.currentDatabaseVersion(ctx, request, "plugin_idc_core_billing_invoice", "invoice_no")
	case notificationcap.BusinessObjectService:
		return s.currentDatabaseVersion(ctx, request, "plugin_idc_core_service_entitlement", "entitlement_no")
	case notificationcap.BusinessObjectSupportTicket:
		ticket, err := s.support.DetailAccount(ctx, request.ObjectNo)
		if err != nil || ticket == nil || ticket.Ticket == nil {
			ticket, err = s.support.DetailStaff(ctx, request.ObjectNo)
		}
		if err != nil || ticket == nil || ticket.Ticket == nil || ticket.Ticket.Version < 1 {
			return notificationcap.BusinessObjectProjection{}, gerror.New("notification business object is unavailable")
		}
		return notificationcap.BusinessObjectProjection{Type: request.Type, ObjectNo: request.ObjectNo, CurrentVersion: ticket.Ticket.Version}, nil
	case notificationcap.BusinessObjectContentItem:
		item, err := s.content.DetailAccount(ctx, request.ObjectNo)
		if err != nil || item == nil {
			item, err = s.content.DetailAdmin(ctx, request.ObjectNo)
		}
		if err != nil || item == nil || item.Version < 1 {
			return notificationcap.BusinessObjectProjection{}, gerror.New("notification business object is unavailable")
		}
		return notificationcap.BusinessObjectProjection{Type: request.Type, ObjectNo: request.ObjectNo, CurrentVersion: item.Version}, nil
	case notificationcap.BusinessObjectMarketingCampaign:
		campaign, err := s.marketing.GetCampaign(ctx, request.ObjectNo)
		if err != nil || campaign == nil || campaign.Version < 1 {
			return notificationcap.BusinessObjectProjection{}, gerror.New("notification business object is unavailable")
		}
		return notificationcap.BusinessObjectProjection{
			Type: request.Type, ObjectNo: request.ObjectNo, CurrentVersion: campaign.Version,
		}, nil
	default:
		return notificationcap.BusinessObjectProjection{}, gerror.New("notification business object is unavailable")
	}
}

// currentDatabaseVersion exposes only the source-owned object number and
// current optimistic-lock version to the notify owner. It never exposes the
// invoice, service, customer, amount, or provider details.
func (s *businessObjectAccess) currentDatabaseVersion(
	ctx context.Context,
	request notificationcap.BusinessObjectRequest,
	table string,
	key string,
) (notificationcap.BusinessObjectProjection, error) {
	value, err := s.database.GetValue(ctx, "SELECT version FROM "+table+" WHERE tenant_id=0 AND "+key+"=?", request.ObjectNo)
	if err != nil || value.Int64() < 1 {
		return notificationcap.BusinessObjectProjection{}, gerror.New("notification business object is unavailable")
	}
	return notificationcap.BusinessObjectProjection{Type: request.Type, ObjectNo: request.ObjectNo, CurrentVersion: value.Int64()}, nil
}
