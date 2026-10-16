// This file proves response-loss recovery never relies on a local operation
// number matching its upstream counterpart and never resubmits from query.

package peer

import (
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

func renewalInput() renewalownercap.PeerRequest {
	return renewalownercap.PeerRequest{Request: renewalownercap.TaskRequest{Version: "v1", OwnerID: "idc-peer", OperationNo: "local-operation-1", EntitlementNo: "local-service", PriceSnapshotNo: "price-1", PluginID: "idc-peer", PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", DeclarationDigest: strings.Repeat("a", 64), RenewalMethod: "renew", ExpectedEntitlementVersion: 1, OriginalExpiresAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), ProjectedExpiresAt: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)}, ExternalReference: "peer-order:19", UpstreamServiceNo: "upstream-service", PurchaseCycles: 1, CurrencyCode: "CNY", PeriodUnitCode: "month", PeriodCount: 1}
}

func TestRenewalResponseLossRecoversByOriginalRequest(t *testing.T) {
	input := renewalInput()
	submit := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"observedAt":1791676800123,"orderNo":"upstream-order","items":[{"deliveryNo":"delivery","status":"completed","ongoing":true,"services":[{"serviceNo":"upstream-service","currencyCode":"CNY","periodUnit":"month","periodCount":1,"status":"active","expiresAt":1791676800000}]}]}}`}, {fail: true}}}
	result, err := Renew(submit, "confidential-context", input)
	if err != nil || result.Status != renewalownercap.TaskStatusResultUnknown || renewalownercap.ValidateTask(*result) != nil {
		t.Fatalf("lost response: %#v %v", result, err)
	}
	if result.ObservedAt.UnixMilli() != 1791676800123 {
		t.Fatal("unknown result must retain authenticated delivery observation")
	}
	query := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"observedAt":1791676800123,"orderNo":"upstream-order","items":[{"deliveryNo":"delivery","status":"completed","ongoing":true,"services":[{"serviceNo":"upstream-service","currencyCode":"CNY","periodUnit":"month","periodCount":1,"status":"active","expiresAt":1794355200000}]}]}}`}, {response: `{"code":0,"data":{"observedAt":1791676800456,"found":true,"status":"succeeded","currencyCode":"CNY","currentExpiresAt":1794355200000}}`}}}
	result, err = QueryRenewal(query, "confidential-context", input)
	if err != nil || result.Status != renewalownercap.TaskStatusSucceeded || renewalownercap.ValidateTask(*result) != nil {
		t.Fatalf("recovery: %#v %v", result, err)
	}
	if result.ObservedAt.UnixMilli() != 1791676800456 {
		t.Fatal("recovered result must use renewal query observation")
	}
	if query.requests[1].Path != apiPath+"/services/upstream-service/renewals/by-request/local-operation-1" {
		t.Fatal("query assumed local and remote operation numbers match")
	}
	for _, request := range query.requests {
		if request.Method != "GET" {
			t.Fatal("recovery repeated payment")
		}
	}
}

func TestRenewalRejectsChangedExpiryBeforePayment(t *testing.T) {
	e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"observedAt":1791676800123,"orderNo":"upstream-order","items":[{"deliveryNo":"delivery","status":"completed","ongoing":true,"services":[{"serviceNo":"upstream-service","currencyCode":"CNY","periodUnit":"month","periodCount":1,"status":"active","expiresAt":1791763200000}]}]}}`}}}
	if result, err := Renew(e, "confidential-context", renewalInput()); err == nil || result != nil || len(e.requests) != 1 {
		t.Fatal("expiry drift reached payment")
	}
}

func TestRenewalRejectsChangedFrozenBillingBeforePayment(t *testing.T) {
	const valid = `{"code":0,"data":{"observedAt":1791676800123,"orderNo":"upstream-order","items":[{"deliveryNo":"delivery","status":"completed","ongoing":true,"services":[{"serviceNo":"upstream-service","currencyCode":"CNY","periodUnit":"month","periodCount":1,"status":"active","expiresAt":1791676800000}]}]}}`
	for _, replacement := range [][2]string{{`"currencyCode":"CNY"`, `"currencyCode":"USD"`}, {`"periodUnit":"month"`, `"periodUnit":"year"`}, {`"periodCount":1`, `"periodCount":3`}, {`"serviceNo":"upstream-service"`, `"serviceNo":"foreign-service"`}} {
		e := &scriptedExecutor{steps: []step{{response: strings.Replace(valid, replacement[0], replacement[1], 1)}}}
		if result, err := Renew(e, "confidential-context", renewalInput()); err == nil || result != nil || len(e.requests) != 1 {
			t.Fatalf("billing drift reached payment: %s", replacement[1])
		}
	}
}

func TestRenewalRequiresAuthenticatedObservations(t *testing.T) {
	const delivery = `{"code":0,"data":{"observedAt":1791676800123,"orderNo":"upstream-order","items":[{"deliveryNo":"delivery","status":"completed","ongoing":true,"services":[{"serviceNo":"upstream-service","currencyCode":"CNY","periodUnit":"month","periodCount":1,"status":"active","expiresAt":1791676800000}]}]}}`
	missingDelivery := &scriptedExecutor{steps: []step{{response: strings.Replace(delivery, `"observedAt":1791676800123,`, "", 1)}}}
	if result, err := Renew(missingDelivery, "confidential-context", renewalInput()); err == nil || result != nil || len(missingDelivery.requests) != 1 {
		t.Fatal("renewal wrote after unauthenticated observation")
	}
	missingQuery := &scriptedExecutor{steps: []step{{response: delivery}, {response: `{"code":0,"data":{"found":true,"status":"succeeded","currencyCode":"CNY","currentExpiresAt":1794355200000}}`}}}
	if result, err := QueryRenewal(missingQuery, "confidential-context", renewalInput()); err == nil || result != nil {
		t.Fatal("successful renewal synthesized a local observation")
	}
}
