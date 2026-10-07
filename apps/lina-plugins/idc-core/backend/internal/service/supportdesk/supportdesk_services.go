// This file validates optional customer entitlement associations and batch-projects their current safe state.
package supportdesk

import (
	"context"
	"strings"
)

// SearchAccountEntitlementCandidates returns a bounded page of non-terminal long-term entitlements owned by the current trusted customer.
func (s *serviceImpl) SearchAccountEntitlementCandidates(ctx context.Context, keyword, associationReference, cursor string, limit int) (*EntitlementCandidatePage, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	keyword = strings.TrimSpace(keyword)
	associationReference = strings.TrimSpace(associationReference)
	if len([]byte(keyword)) > 128 {
		return nil, bizInvalid()
	}
	if associationReference != "" && !businessNoPattern.MatchString(associationReference) {
		return nil, bizInvalid()
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 50 {
		return nil, bizInvalid()
	}
	cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	where := strings.Builder{}
	where.WriteString(` WHERE e.tenant_id=0 AND e.customer_no=? AND e.status_code<>'terminated'`)
	args := []any{a.CustomerNo}
	if associationReference != "" {
		where.WriteString(` AND e.support_association_reference=?`)
		args = append(args, associationReference)
	}
	if cursorID > 0 {
		where.WriteString(` AND e.id<?`)
		args = append(args, cursorID)
	}
	if keyword != "" {
		where.WriteString(` AND (LOWER(i.product_name_zh_cn) LIKE LOWER(?) ESCAPE '\' OR LOWER(i.product_name_en_us) LIKE LOWER(?) ESCAPE '\')`)
		pattern := escapeLike(keyword) + "%"
		args = append(args, pattern, pattern)
	}
	args = append(args, limit+1)
	var rows []struct {
		ID int64 `orm:"id"`
		EntitlementCandidate
	}
	if err = s.database.GetScan(ctx, &rows, `SELECT e.id,e.entitlement_no,e.support_association_reference AS association_reference,i.product_name_zh_cn AS name_zh_cn,i.product_name_en_us AS name_en_us,e.status_code AS status,e.version FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.intent_no=e.source_intent_no`+where.String()+` ORDER BY e.id DESC LIMIT ?`, args...); err != nil {
		return nil, wrapUnavailable(err, "search customer entitlement candidates")
	}
	page := &EntitlementCandidatePage{Items: make([]EntitlementCandidate, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index >= limit {
			break
		}
		page.Items = append(page.Items, row.EntitlementCandidate)
	}
	if len(rows) > limit {
		page.NextCursor = encodeCursor(rows[limit-1].ID)
	}
	return page, nil
}

func (s *serviceImpl) resolveRelatedEntitlement(ctx context.Context, ticketType, customerNo, associationReference string) (*EntitlementCandidate, error) {
	associationReference = strings.TrimSpace(associationReference)
	if associationReference == "" {
		return nil, nil
	}
	if ticketType != "customer" || !businessNoPattern.MatchString(associationReference) || !businessNoPattern.MatchString(customerNo) {
		return nil, bizInvalid()
	}
	var rows []EntitlementCandidate
	if err := s.database.GetScan(ctx, &rows, `SELECT e.entitlement_no,e.support_association_reference AS association_reference,i.product_name_zh_cn AS name_zh_cn,i.product_name_en_us AS name_en_us,e.status_code AS status,e.version FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.intent_no=e.source_intent_no WHERE e.tenant_id=0 AND e.support_association_reference=? AND e.customer_no=? AND e.status_code<>'terminated' FOR SHARE OF e,i`, associationReference, customerNo); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizInvalid()
	}
	return &rows[0], nil
}

func (s *serviceImpl) loadCurrentEntitlementProjections(ctx context.Context, tickets []*Ticket) error {
	entitlementNos := make([]string, 0, len(tickets))
	seen := make(map[string]struct{}, len(tickets))
	for _, ticket := range tickets {
		if ticket == nil || ticket.RelatedEntitlementNo == "" {
			continue
		}
		if _, exists := seen[ticket.RelatedEntitlementNo]; exists {
			continue
		}
		seen[ticket.RelatedEntitlementNo] = struct{}{}
		entitlementNos = append(entitlementNos, ticket.RelatedEntitlementNo)
	}
	if len(entitlementNos) == 0 {
		return nil
	}
	args := make([]any, 0, len(entitlementNos))
	for _, entitlementNo := range entitlementNos {
		args = append(args, entitlementNo)
	}
	var rows []EntitlementCandidate
	if err := s.database.GetScan(ctx, &rows, `SELECT e.entitlement_no,e.support_association_reference AS association_reference,i.product_name_zh_cn AS name_zh_cn,i.product_name_en_us AS name_en_us,e.status_code AS status,e.version FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.intent_no=e.source_intent_no WHERE e.tenant_id=0 AND e.entitlement_no IN (`+sqlPlaceholders(len(entitlementNos))+`)`, args...); err != nil {
		return wrapUnavailable(err, "load current ticket entitlement projections")
	}
	current := make(map[string]EntitlementCandidate, len(rows))
	for _, row := range rows {
		current[row.EntitlementNo] = row
	}
	for _, ticket := range tickets {
		row, ok := current[ticket.RelatedEntitlementNo]
		if !ok {
			continue
		}
		ticket.CurrentEntitlementNameZhCN = row.NameZhCN
		ticket.CurrentEntitlementNameEnUS = row.NameEnUS
		ticket.CurrentEntitlementStatus = row.Status
		ticket.CurrentEntitlementVersion = row.Version
		ticket.CurrentEntitlementAvailable = true
		ticket.EntitlementAssociationRef = row.AssociationReference
	}
	return nil
}
