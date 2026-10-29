package commerce

import (
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func TestDeliveryDirectoryReadsSettledOrderAndEnforcesOwnershipPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioWithMaterialization(t, true)
	scenario := prepared.checkout

	var itemRows []struct {
		OrderItemNo string `orm:"order_item_no"`
	}
	if err := scenario.database.GetScan(scenario.ctx, &itemRows, `
SELECT order_item_no
FROM plugin_idc_core_commerce_order_item
WHERE tenant_id=0 AND order_no=?
ORDER BY line_number`, prepared.order.OrderNo); err != nil || len(itemRows) != 1 {
		t.Fatalf("load settled order items: items=%v err=%v", itemRows, err)
	}
	itemNos := []string{itemRows[0].OrderItemNo}

	account, err := NewAccountDeliveryDirectory(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatalf("create account delivery directory: %v", err)
	}
	deliveries, err := account.BatchGetAccount(scenario.ctx, prepared.order.OrderNo, itemNos)
	if err != nil || len(deliveries) != len(itemNos) ||
		deliveries[0].OrderItemNo != itemNos[0] || deliveries[0].Quantity != 1 ||
		deliveries[0].StatusCode != "processing" || deliveries[0].OngoingManagement {
		t.Fatalf("account deliveries=%+v err=%v", deliveries, err)
	}
	orderReader, err := NewAccountOrderReader(
		scenario.database, scenario.service.principal,
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatalf("create account order reader: %v", err)
	}
	detail, err := orderReader.GetAccountOrder(scenario.ctx, prepared.order.OrderNo)
	if err != nil || len(detail.Deliveries) != 1 || len(detail.ServiceEntitlements) != 0 ||
		detail.Deliveries[0].OrderItemNo != deliveries[0].OrderItemNo ||
		detail.Deliveries[0].Quantity != deliveries[0].Quantity ||
		detail.Deliveries[0].StatusCode != deliveries[0].StatusCode ||
		detail.Deliveries[0].OngoingManagement != deliveries[0].OngoingManagement {
		t.Fatalf("account order delivery detail=%+v err=%v", detail, err)
	}

	foreignAccount, err := NewAccountDeliveryDirectory(scenario.database, commercePrincipalStub{
		audience: &principal.Audience{
			UserID: 202, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-2",
		},
	})
	if err != nil {
		t.Fatalf("create foreign account delivery directory: %v", err)
	}
	if _, err = foreignAccount.BatchGetAccount(scenario.ctx, prepared.order.OrderNo, itemNos); !bizerr.Is(err, CodeOrderDirectoryNotAccessible) {
		t.Fatalf("foreign customer read error=%v", err)
	}

	management, err := NewManagementDeliveryDirectory(
		scenario.database, managementOrderAuthorizerStub{},
	)
	if err != nil {
		t.Fatalf("create management delivery directory: %v", err)
	}
	staffDeliveries, err := management.BatchGetManagement(
		scenario.ctx, prepared.order.OrderNo, itemNos,
	)
	if err != nil || len(staffDeliveries) != 1 ||
		staffDeliveries[0].OrderItemNo != deliveries[0].OrderItemNo ||
		staffDeliveries[0].Quantity != deliveries[0].Quantity ||
		staffDeliveries[0].StatusCode != deliveries[0].StatusCode ||
		staffDeliveries[0].OngoingManagement != deliveries[0].OngoingManagement {
		t.Fatalf("management deliveries=%+v err=%v", staffDeliveries, err)
	}

	outOfScopeAuthorizer, err := NewManagementOrderAuthorizer(
		riskReviewBizCtxStub{current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 3}},
		riskReviewPrincipalStub{audience: &principal.Audience{
			UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
		}},
		riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionOrderDirectoryDetail): true,
		}},
	)
	if err != nil {
		t.Fatalf("create out-of-scope order authorizer: %v", err)
	}
	denied, err := NewManagementDeliveryDirectory(scenario.database, outOfScopeAuthorizer)
	if err != nil {
		t.Fatalf("create denied management delivery directory: %v", err)
	}
	if _, err = denied.BatchGetManagement(scenario.ctx, prepared.order.OrderNo, itemNos); !bizerr.Is(err, CodeOrderDirectoryNotAccessible) {
		t.Fatalf("out-of-scope employee read error=%v", err)
	}
}

func TestDeliveryDirectoryReadsOngoingServiceEntitlementPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptions(t, true, true)
	reader, err := NewAccountOrderReader(prepared.checkout.database, prepared.checkout.service.principal, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create pending order reader: %v", err)
	}
	pending, err := reader.GetAccountOrder(prepared.checkout.ctx, prepared.order.OrderNo)
	if err != nil || len(pending.ServiceEntitlements) != 0 {
		t.Fatalf("pending activation order must remain readable without a managed service: detail=%+v err=%v", pending, err)
	}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer(
		"test-key-v1", []byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatalf("create provider evidence sealer: %v", err)
	}
	worker, err := productfulfillmentservice.NewWorker(
		prepared.checkout.database, &fulfillmentWorkerExtensions{},
		&fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("create fulfillment worker: %v", err)
	}
	first, err := worker.Run(prepared.checkout.ctx, 1)
	if err != nil || first.Scheduled != 1 {
		t.Fatalf("schedule ongoing fulfillment: batch=%+v err=%v", first, err)
	}
	// Model a long provisioning wait: the purchased month starts at success.
	if _, err = prepared.checkout.database.Exec(prepared.checkout.ctx, `UPDATE plugin_idc_core_service_entitlement SET started_at=CURRENT_TIMESTAMP-INTERVAL '2 days' WHERE order_no=?`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	activatedAfter := time.Now().Add(-time.Second)
	advanceFulfillmentTaskClock(t, prepared.checkout, prepared.order.OrderNo)
	second, err := worker.Run(prepared.checkout.ctx, 1)
	if err != nil || second.Completed != 1 {
		t.Fatalf("complete ongoing fulfillment: batch=%+v err=%v", second, err)
	}
	var period struct {
		StartedAt, ExpiresAt time.Time
		MatchesFrozenPeriod  bool
	}
	if err = prepared.checkout.database.GetScan(prepared.checkout.ctx, &period, `SELECT e.started_at,e.expires_at,(e.expires_at=((e.started_at AT TIME ZONE 'UTC')+make_interval(months=>item.period_count*item.purchase_cycles)) AT TIME ZONE 'UTC') AS matches_frozen_period FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_commerce_order_item item USING(order_item_no) WHERE e.order_no=?`, prepared.order.OrderNo); err != nil || !period.MatchesFrozenPeriod || period.StartedAt.Before(activatedAfter) || !period.ExpiresAt.After(period.StartedAt) {
		t.Fatalf("activated monthly service has incorrect dates: %+v err=%v", period, err)
	}
	// Repeated completion and suspend/resume must not consume another period.
	if _, err = prepared.checkout.database.Exec(prepared.checkout.ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active' WHERE order_no=?;`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	var replay struct{ StartedAt, ExpiresAt time.Time }
	if err = prepared.checkout.database.GetScan(prepared.checkout.ctx, &replay, `SELECT started_at,expires_at FROM plugin_idc_core_service_entitlement WHERE order_no=?`, prepared.order.OrderNo); err != nil || !replay.StartedAt.Equal(period.StartedAt) || !replay.ExpiresAt.Equal(period.ExpiresAt) {
		t.Fatalf("completion replay changed the purchased period: %+v err=%v", replay, err)
	}
	reader, err = NewAccountOrderReader(
		prepared.checkout.database, prepared.checkout.service.principal,
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatalf("create ongoing order reader: %v", err)
	}
	detail, err := reader.GetAccountOrder(prepared.checkout.ctx, prepared.order.OrderNo)
	if err != nil || len(detail.Deliveries) != 1 ||
		detail.Deliveries[0].StatusCode != "completed" ||
		!detail.Deliveries[0].OngoingManagement || len(detail.ServiceEntitlements) != 1 ||
		detail.ServiceEntitlements[0].EntitlementNo == "" ||
		detail.ServiceEntitlements[0].StatusCode != "active" ||
		detail.ServiceEntitlements[0].OrderItemNo != detail.Deliveries[0].OrderItemNo {
		t.Fatalf("ongoing order delivery detail=%+v err=%v", detail, err)
	}
	// Management preparation must not change the completed delivery, active
	// lifecycle or frozen billing dates attached to this exact customer order.
	for state, want := range map[string]string{"pending": "preparing", "retry_wait": "preparing", "delivering": "preparing", "delivered": "ready", "manual_attention": "needs_attention"} {
		result, updateErr := prepared.checkout.database.Exec(prepared.checkout.ctx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code=? WHERE entitlement_no=?`, state, detail.ServiceEntitlements[0].EntitlementNo)
		if updateErr != nil {
			t.Fatal(updateErr)
		}
		affected, updateErr := result.RowsAffected()
		if updateErr != nil || affected != 1 {
			t.Fatalf("missing own management notification: affected=%d err=%v", affected, updateErr)
		}
		updated, readErr := reader.GetAccountOrder(prepared.checkout.ctx, prepared.order.OrderNo)
		if readErr != nil || len(updated.ServiceEntitlements) != 1 || updated.ServiceEntitlements[0].ManagementPreparation != want || updated.Deliveries[0].StatusCode != "completed" || updated.ServiceEntitlements[0].StatusCode != "active" || !updated.ServiceEntitlements[0].StartedAt.Equal(period.StartedAt) || !updated.ServiceEntitlements[0].ExpiresAt.Equal(period.ExpiresAt) {
			t.Fatalf("management readiness changed commerce facts: detail=%+v err=%v", updated, readErr)
		}
	}

}

func TestDeliveryDirectoryRejectsPartialAndDuplicateBatchesPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioWithMaterialization(t, true)
	scenario := prepared.checkout
	var item struct {
		OrderItemNo string `orm:"order_item_no"`
	}
	if err := scenario.database.GetScan(scenario.ctx, &item, `
SELECT order_item_no FROM plugin_idc_core_commerce_order_item
WHERE tenant_id=0 AND order_no=? LIMIT 1`, prepared.order.OrderNo); err != nil || item.OrderItemNo == "" {
		t.Fatalf("load settled order item: item=%q err=%v", item.OrderItemNo, err)
	}
	itemNo := item.OrderItemNo
	reader, err := NewAccountDeliveryDirectory(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatalf("create account delivery directory: %v", err)
	}
	if _, err = reader.BatchGetAccount(scenario.ctx, prepared.order.OrderNo, []string{itemNo, itemNo}); !bizerr.Is(err, CodeOrderDirectoryInvalid) {
		t.Fatalf("duplicate item batch error=%v", err)
	}
	if _, err = reader.BatchGetAccount(
		scenario.ctx, prepared.order.OrderNo,
		[]string{itemNo, "order-item-does-not-exist"},
	); !bizerr.Is(err, CodeOrderDirectoryNotAccessible) {
		t.Fatalf("partial item batch error=%v", err)
	}
}
