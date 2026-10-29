// This file reads persisted commercial history independently of live resource
// health. Core reauthorizes the current entitlement owner and financial scope.
package customeroperations

import (
	"context"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// CommercialHistoryQuery never accepts customer or entitlement identity.
type CommercialHistoryQuery struct {
	ServerNo, OperationNo, Cursor string
	Limit                         int
}

// AccessoryBillingQuery selects local verified object bindings only.
type AccessoryBillingQuery struct {
	ServerNo, ObjectNo, Kind, Cursor string
	Limit                            int
}

// CommercialOperations returns a bounded current-owner page, without observing
// or operating a supplier, even if the channel or parent is unavailable.
func (s *NativeCommercial) CommercialOperations(ctx context.Context, query CommercialHistoryQuery) (*servercommercialcap.OperationPage, error) {
	reader, ok := s.owner.(servercommercialcap.CommercialReadAccess)
	if !ok {
		return nil, ErrUnavailable
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Cursor) > 256 || len(query.OperationNo) > 40 {
		return nil, ErrInvalid
	}
	owned, err := s.management.commercialReadIdentity(ctx, query.ServerNo)
	if err != nil {
		return nil, err
	}
	return reader.ListOperations(ctx, servercommercialcap.OperationListRequest{EntitlementNo: owned.EntitlementNo, CustomerNo: owned.CustomerNo, OperationNo: query.OperationNo, Cursor: query.Cursor, Limit: query.Limit})
}

// AccessoryBilling reads real bound object billing and never invents a period
// or treats the latest catalog price as an existing invoice amount.
func (s *NativeCommercial) AccessoryBilling(ctx context.Context, query AccessoryBillingQuery) (*servercommercialcap.AccessoryBillingPage, error) {
	reader, ok := s.owner.(servercommercialcap.CommercialReadAccess)
	if !ok {
		return nil, ErrUnavailable
	}
	kind := serveroperationresourcecap.AccessoryKind(query.Kind)
	if query.Limit < 1 || query.Limit > 100 || len(query.Cursor) > 256 || len(query.ObjectNo) > 128 || (kind != "" && kind != serveroperationresourcecap.AccessoryKindSnapshot && kind != serveroperationresourcecap.AccessoryKindBackup) {
		return nil, ErrInvalid
	}
	owned, err := s.management.commercialReadIdentity(ctx, query.ServerNo)
	if err != nil {
		return nil, err
	}
	return reader.ListAccessoryBilling(ctx, servercommercialcap.AccessoryBillingListRequest{EntitlementNo: owned.EntitlementNo, CustomerNo: owned.CustomerNo, ObjectNo: query.ObjectNo, Kind: kind, Cursor: query.Cursor, Limit: query.Limit})
}

// Read identity depends on retained owner projection, not active channel, live
// mapping or permission to execute. Core separately checks its authoritative ENT.
func (s *Service) commercialReadIdentity(ctx context.Context, serverNo string) (*route, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !serverPattern.MatchString(serverNo) {
		return nil, ErrUnavailable
	}
	customer, err := s.customers.CurrentCustomer(ctx)
	if err != nil || customer == nil || customer.CustomerNo == "" {
		return nil, ErrUnavailable
	}
	var owned *route
	if err = s.database.GetScan(ctx, &owned, `SELECT server_no,customer_no,entitlement_no FROM plugin_idc_resource_server_entitlement_projection WHERE tenant_id=0 AND server_no=? AND customer_no=?`, serverNo, customer.CustomerNo); err != nil || owned == nil {
		return nil, ErrUnavailable
	}
	return owned, nil
}
