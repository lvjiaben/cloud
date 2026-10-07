// This file binds a selected fixed traffic package to the customer's original
// server. Capacity is derived exclusively from the immutable merchant version;
// callers cannot submit arbitrary GB or redirect the resulting provider action.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// CustomerTrafficPackages checks the real customer, ownership, entitlement and
// fresh provider capability before exposing applicable package prices.
func (s *accessoryPricingImpl) CustomerTrafficPackages(ctx context.Context, serviceNo string, input TrafficPackageSearch) (*TrafficPackagePage, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) || !validReference(serviceNo, 40) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	return s.ForServerTrafficPackages(ctx, subject.CustomerNo, serviceNo, input)
}

// ForServerTrafficPackages is called only by an already-authorized customer or
// staff owner. It retains exact ownership, entitlement and fresh capability checks.
func (s *accessoryPricingImpl) ForServerTrafficPackages(ctx context.Context, customerNo, serviceNo string, input TrafficPackageSearch) (*TrafficPackagePage, error) {
	if !validReference(customerNo, 40) || !validReference(serviceNo, 40) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	var service *serviceRow
	err := s.database.GetScan(ctx, &service, `SELECT service.service_no,service.customer_no,service.currency_code,
service.pool_no,COALESCE(service.channel_no,'') AS channel_no,service.entitlement_status_code,
service.runtime_status_code,service.provider_task_status_code,service.version,
EXISTS (SELECT 1 FROM plugin_idc_core_service_traffic_suspension suspension WHERE suspension.tenant_id=0 AND suspension.service_no=service.service_no AND suspension.active) AS traffic_suspended
FROM plugin_idc_core_service_instance service WHERE service.tenant_id=0 AND service.service_no=? AND service.customer_no=?`, serviceNo, customerNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryQuoteUnavailable)
	}
	if service == nil || service.ChannelNo == "" || !allowedAction(service, serveroperationresourcecap.ActionTrafficAdd) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	directory, err := s.directory.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{ServiceNo: service.ServiceNo, PoolNo: service.PoolNo, ChannelNo: service.ChannelNo})
	if err != nil || directory == nil || !directory.ExpiresAt.After(time.Now().UTC()) || !containsAccessoryAction(directory.Actions, serveroperationresourcecap.ActionTrafficAdd) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	return s.listTrafficPackages(ctx, input, serviceNo, customerNo)
}

// resolveTrafficPackage treats the submitted reference as untrusted and binds
// it to the current server product and currency in a single ownership query.
func (s *accessoryPricingImpl) resolveTrafficPackage(ctx context.Context, input AccessoryQuoteInput) (*TrafficPackage, error) {
	if !validReference(input.PackageNo, 40) || !validReference(input.CustomerNo, 40) || !validReference(input.ServiceNo, 40) {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	parameters := strings.TrimSpace(string(input.Parameters))
	if parameters != "" && parameters != "{}" {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	var result *TrafficPackage
	err := s.trafficPackageModel(ctx).Fields(trafficPackageProjection).
		Where("policy.policy_no", input.PackageNo).Where("policy.enabled", true).
		Where(`EXISTS (SELECT 1 FROM plugin_idc_core_traffic_package_product scope
JOIN plugin_idc_core_service_instance service ON service.tenant_id=scope.tenant_id AND service.product_no=scope.product_no
WHERE scope.tenant_id=0 AND scope.policy_no=policy.policy_no AND scope.policy_version=policy.current_version
 AND service.service_no=? AND service.customer_no=? AND service.currency_code=policy.currency_code)`, input.ServiceNo, input.CustomerNo).Scan(&result)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryQuoteUnavailable)
	}
	if result == nil {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	return result, nil
}

// freezeTrafficParameters creates only the capability's supported additive field.
func freezeTrafficParameters(capacity int) (json.RawMessage, error) {
	return json.Marshal(struct {
		TrafficGB int `json:"trafficGb"`
	}{capacity})
}

// reserveTrafficPackage serializes concurrent checkouts with package edits. The
// addon insert later in this same transaction is the durable inventory hold.
func (s *accessoryPricingImpl) reserveTrafficPackage(ctx context.Context, row *accessoryCheckoutRow) error {
	if row.AccessoryKind != string(serveroperationresourcecap.AccessoryKindTraffic) {
		return nil
	}
	var policy *entity.ServerAccessoryPricePolicy
	if err := s.database.Model(dao.ServerAccessoryPricePolicy.Table()).Ctx(ctx).
		Where(do.ServerAccessoryPricePolicy{TenantId: 0, PolicyNo: row.PolicyNo}).LockUpdate().Scan(&policy); err != nil {
		return err
	}
	if policy == nil || !policy.Enabled || policy.CurrentVersion != row.PolicyVersion || policy.CurrencyCode != row.CurrencyCode || policy.AccessoryKind != row.AccessoryKind {
		return bizerr.NewCode(CodeAccessoryPriceConflict)
	}
	valid, err := s.database.GetValue(ctx, `SELECT EXISTS (SELECT 1 FROM plugin_idc_core_traffic_package_product scope
JOIN plugin_idc_core_service_instance service ON service.tenant_id=scope.tenant_id AND service.product_no=scope.product_no
WHERE scope.tenant_id=0 AND scope.policy_no=? AND scope.policy_version=? AND service.service_no=? AND service.customer_no=?)`, row.PolicyNo, row.PolicyVersion, row.ServiceNo, row.CustomerNo)
	if err != nil {
		return err
	}
	if !valid.Bool() {
		return bizerr.NewCode(CodeAccessoryPriceConflict)
	}
	if policy.StockQuantity != nil {
		var occupancy []struct{ Quantity int64 }
		if err = s.trafficPackageOccupancy(ctx, []string{row.PolicyNo}).Scan(&occupancy); err != nil {
			return err
		}
		quantity := int64(0)
		if len(occupancy) > 0 {
			quantity = occupancy[0].Quantity
		}
		if quantity >= *policy.StockQuantity {
			return bizerr.NewCode(CodeAccessoryPriceConflict)
		}
	}
	return nil
}
