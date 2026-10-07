// This file checks staff traffic purchases against real server and package SQL.
// Only identity and upstream capabilities are fixtures; no provider is contacted.

package serveroperation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// trafficManagementAuthorization grants precisely the tested staff actions.
type trafficManagementAuthorization struct{ trafficTestAuthorization }

func (trafficManagementAuthorization) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{
		authz.PermissionKey(PermissionServerAccessoryView): true,
		authz.PermissionKey(PermissionTrafficAdd):          true,
	}, nil
}

// trafficManagementCapabilities is the read-only capability owner boundary.
type trafficManagementCapabilities struct{}

func (trafficManagementCapabilities) ListCapabilities(_ context.Context, routes []serveroperationresourcecap.CapabilityRoute) ([]serveroperationresourcecap.CapabilityProjection, error) {
	items := make([]serveroperationresourcecap.CapabilityProjection, 0, len(routes))
	for _, route := range routes {
		items = append(items, serveroperationresourcecap.CapabilityProjection{ServiceNo: route.ServiceNo, Actions: []serveroperationresourcecap.Action{serveroperationresourcecap.ActionTrafficAdd}})
	}
	return items, nil
}

// trafficManagementPricing observes entry into the real staff checkout boundary.
type trafficManagementPricing struct {
	AccessoryPricing
	checkouts int
}

func (pricing *trafficManagementPricing) CheckoutForStaff(ctx context.Context, customer string, input AccessoryCheckoutInput, actor int, permission string) (*AccessoryCheckout, error) {
	pricing.checkouts++
	return pricing.AccessoryPricing.CheckoutForStaff(ctx, customer, input, actor, permission)
}

// TestTrafficManagementVersionPostgreSQL proves a stale displayed price cannot
// silently create an order at a newer price, and normal staff purchases stay unpaid.
func TestTrafficManagementVersionPostgreSQL(t *testing.T) {
	fixture := newTrafficPackageFixture(t)
	applyTrafficFixtureMigration(t, fixture, "065-server-customer-metadata.sql")
	resource := trafficTestResource{time.Now().UTC().Add(time.Hour)}
	pricing := &trafficManagementPricing{AccessoryPricing: fixture.service}
	authorization := trafficManagementAuthorization{}
	fixture.service.authorization = authorization
	management := &managementServiceImpl{
		bizCtx: fixture.service.bizCtx, principal: fixture.service.principal,
		authorization: authorization, pricing: pricing, parameters: resource,
		directory: &directoryImpl{database: fixture.database, capabilities: trafficManagementCapabilities{}, accessories: resource},
	}
	command := ManagementCommand{
		ServiceNo: "SRV_checkout_original", Action: serveroperationresourcecap.ActionTrafficAdd,
		PackageNo: "POL_checkout_test", PackageVersion: 2, Confirmed: true,
		Parameters: json.RawMessage(`{}`), IdempotencyKey: "staff-traffic-version-test",
	}
	if _, err := management.Create(fixture.ctx, command); !bizerr.Is(err, CodeAccessoryPriceConflict) {
		t.Fatalf("stale package version must reject before checkout: %+v", err)
	}
	if pricing.checkouts != 0 {
		t.Fatal("stale displayed price reached checkout")
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_billing_invoice`, 0)
	command.PackageVersion = 1
	result, err := management.Create(fixture.ctx, command)
	if err != nil {
		t.Fatalf("staff purchase: %+v", err)
	}
	if !result.PaymentRequired || result.AmountMinor != 1000 || result.Status != "awaiting_financial_settlement" || result.InvoiceNo == "" {
		t.Fatalf("staff purchase bypassed unpaid gate: %+v", result)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_server_operation_execution`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_financial_settlement`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_service_instance`, 1)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 1)
	replayed, err := management.Create(fixture.ctx, command)
	if err != nil || replayed.OrderNo != result.OrderNo {
		t.Fatalf("staff retry changed order: %+v / %+v", replayed, err)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 1)
	command.PackageVersion = 9
	if _, err = management.Create(fixture.ctx, command); !bizerr.Is(err, CodeAccessoryPriceConflict) {
		t.Fatalf("changed package version reused an existing purchase: %+v", err)
	}
	command.PackageVersion = 1
	command.Parameters = json.RawMessage(`{"trafficGb":999}`)
	if _, err = management.Create(fixture.ctx, command); !bizerr.Is(err, CodeAccessoryManagementInvalid) {
		t.Fatalf("caller-supplied capacity reused an existing purchase: %+v", err)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 1)
}
