// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package peer

import (
	"encoding/json"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type step struct {
	response string
	fail     bool
}
type scriptedExecutor struct {
	steps    []step
	requests []resourcecap.ConfidentialHTTPRequest
}

func (e *scriptedExecutor) Execute(contextID string, sequence uint8, payload json.RawMessage) (json.RawMessage, error) {
	if contextID != "confidential-context" || int(sequence) != len(e.requests)+1 {
		return nil, errors.New("invalid sequence")
	}
	var request resourcecap.ConfidentialHTTPRequest
	if json.Unmarshal(payload, &request) != nil {
		return nil, errInvalid
	}
	e.requests = append(e.requests, request)
	if len(e.requests) > len(e.steps) {
		return nil, errors.New("unexpected request")
	}
	current := e.steps[len(e.requests)-1]
	if current.fail {
		return nil, errors.New("response lost")
	}
	return json.Marshal(resourcecap.ConfidentialHTTPResponse{StatusCode: 200, Data: json.RawMessage(current.response)})
}

func TestPurchaseFreezesCustomerPriceThenOrdersWithoutCreatingPrincipal(t *testing.T) {
	e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"quoteNo":"quote-1","currency":"CNY","totalAmountMinor":7000}}`}, {response: `{"code":0,"data":{"id":19,"status":"paid","replayed":false}}`}}}
	result, err := Purchase(e, "confidential-context", "intent-1", testPurchase())
	if err != nil || result.Status != productfulfillmentcap.StatusAccepted || result.ExternalReference != "peer-order:19" {
		t.Fatalf("unexpected purchase: %#v %v", result, err)
	}
	if len(e.requests) != 2 || e.requests[0].Path != apiPath+"/quotes" || e.requests[1].Path != apiPath+"/orders" {
		t.Fatalf("unexpected calls: %#v", e.requests)
	}
	var payload map[string]json.RawMessage
	if json.Unmarshal(e.requests[1].JSONBody, &payload) != nil || len(payload) != 2 || string(payload["quoteNo"]) != `"quote-1"` {
		t.Fatal("ordering must use only frozen quote and request identity")
	}
	for _, request := range e.requests {
		if !request.UseAuthentication || len(request.Form) != 0 || len(request.CredentialFields) != 0 {
			t.Fatal("credentials must remain owner injected")
		}
	}
}

func TestLostOrderResponseNeverRepeatsWriteInInvocation(t *testing.T) {
	e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"quoteNo":"quote-1","currency":"CNY","totalAmountMinor":7000}}`}, {fail: true}}}
	result, err := Purchase(e, "confidential-context", "intent-1", testPurchase())
	if err != nil || result.Status != productfulfillmentcap.StatusResultUnknown || len(e.requests) != 2 {
		t.Fatalf("uncertain result must stop: %#v %v", result, err)
	}
}

func TestRejectedBalanceLeavesNoAcceptedPurchase(t *testing.T) {
	e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"quoteNo":"quote-1","currency":"CNY","totalAmountMinor":7000}}`}, {response: `{"code":51,"data":null}`}}}
	result, err := Purchase(e, "confidential-context", "intent-1", testPurchase())
	if err != nil || result.Status != productfulfillmentcap.StatusNotAccepted {
		t.Fatalf("rejection was treated as accepted: %#v %v", result, err)
	}
}

func TestQueryRecoversLostOrderAndReadsCompletedDelivery(t *testing.T) {
	e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"found":true,"order":{"id":19,"coreOrderNo":"order-1","status":"paid","financiallySettled":true}}}`}, {response: `{"code":0,"data":{"orderNo":"order-1","items":[{"deliveryNo":"delivery-1","status":"completed","startedAt":1791676800000,"quantity":1,"ongoing":true,"services":[{"serviceNo":"service-1","status":"active","startedAt":1791676800000}]}]}}`}}}
	result, err := QueryPurchase(e, "confidential-context", "intent-1")
	if err != nil || result.Status != productfulfillmentcap.StatusSucceeded || result.ExternalReference != "peer-order:19" {
		t.Fatalf("unexpected recovery: %#v %v", result, err)
	}
	for _, request := range e.requests {
		if request.Method != "GET" {
			t.Fatal("recovery must not write")
		}
	}
	if e.requests[1].Path != apiPath+"/orders/19/deliveries" {
		t.Fatal("service lookup must be scoped to recovered order")
	}
}

func TestQueryMissingAndForeignService(t *testing.T) {
	missing := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"found":false,"order":null}}`}}}
	result, err := QueryPurchase(missing, "confidential-context", "intent-1")
	if err != nil || result.Status != productfulfillmentcap.StatusNotFound || len(missing.requests) != 1 {
		t.Fatalf("missing order: %#v %v", result, err)
	}
	foreign := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"found":true,"order":{"id":19,"coreOrderNo":"order-1","financiallySettled":true}}}`}, {response: `{"code":0,"data":{"list":[{"serviceNo":"service-2","orderNo":"order-2","status":"active","startedAt":1791676800000}]}}`}}}
	if result, err = QueryPurchase(foreign, "confidential-context", "intent-1"); err == nil || result != nil {
		t.Fatal("foreign service accepted")
	}
}

func testPurchase() channelcommercecap.Purchase {
	return channelcommercecap.Purchase{PublicSlug: "server", ConfigurationRevision: "revision", BillingTermToken: "term", CurrencyCode: "CNY", PurchaseCycles: 1, Quantity: 1, Values: []channelcommercecap.PurchaseValue{}}
}

func TestCompletedOneTimeAndMultipleDeliveriesDoNotRequireSingleService(t *testing.T) {
	for _, items := range []string{
		`[{"deliveryNo":"one-time","status":"completed","startedAt":1791676800000,"quantity":1,"ongoing":false,"services":[]}]`,
		`[{"deliveryNo":"first","status":"completed","startedAt":1791676800000,"quantity":1,"ongoing":true,"services":[{"serviceNo":"service-a","status":"active","startedAt":1791676800000}]},{"deliveryNo":"second","status":"completed","startedAt":1791676800000,"quantity":1,"ongoing":false,"services":[]}]`,
	} {
		e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"found":true,"order":{"id":19,"coreOrderNo":"order-1","financiallySettled":true}}}`}, {response: `{"code":0,"data":{"orderNo":"order-1","items":` + items + `}}`}}}
		result, err := QueryPurchase(e, "confidential-context", "intent-1")
		if err != nil || result.Status != productfulfillmentcap.StatusSucceeded {
			t.Fatalf("valid non-server delivery rejected: %#v %v", result, err)
		}
	}
}
