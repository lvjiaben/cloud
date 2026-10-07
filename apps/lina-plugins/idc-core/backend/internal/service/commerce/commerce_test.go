package commerce

import (
	"bytes"
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestGuestTokenStoresOnlyDigest(t *testing.T) {
	random := bytes.NewReader(make([]byte, 32))
	token, digest, err := newGuestToken(random)
	if err != nil {
		t.Fatalf("create guest token: %v", err)
	}
	if len(token) != 49 || len(digest) != 64 || digest == token || digest != digestGuestToken(token) {
		t.Fatalf("guest token=%q digest=%q", token, digest)
	}
	if normalized, normalizeErr := normalizeGuestToken(token); normalizeErr != nil || normalized != digest {
		t.Fatalf("normalize guest token digest=%q err=%v", normalized, normalizeErr)
	}
	for _, malformed := range []string{"", " " + token, token + " ", "cart_" + token, "gcart_invalid"} {
		if _, normalizeErr := normalizeGuestToken(malformed); !bizerr.Is(normalizeErr, CodeNotAccessible) {
			t.Fatalf("malformed guest token %q error=%v", malformed, normalizeErr)
		}
	}
}

func TestCartItemRepairProjection(t *testing.T) {
	for _, test := range []struct {
		issue  string
		status string
		repair string
	}{
		{"", CartItemStatusActive, ""},
		{"product_unavailable", CartItemStatusUnavailable, CartRepairRemove},
		{"version_replaced", CartItemStatusNeedsReprice, CartRepairReplaceVersion},
		{"billing_unavailable", CartItemStatusNeedsReprice, CartRepairRequote},
		{"unknown", CartItemStatusUnavailable, CartRepairRemove},
	} {
		status, repair := cartItemRepair(test.issue)
		if status != test.status || repair != test.repair {
			t.Fatalf("issue=%q status=%q repair=%q want=%q/%q",
				test.issue, status, repair, test.status, test.repair)
		}
	}
}

func TestCurrentCustomerRequiresActiveCustomerAudience(t *testing.T) {
	for _, audience := range []*principal.Audience{
		nil,
		{Code: principal.AudienceStaff, Status: principal.AudienceStatusActive},
		{Code: principal.AudienceCustomer, Status: principal.AudienceStatusRevoked, CustomerNo: "customer-1"},
		{Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive},
	} {
		service := &serviceImpl{principal: commercePrincipalStub{audience: audience}}
		if _, err := service.currentCustomerNo(context.Background()); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("audience=%+v error=%v", audience, err)
		}
	}
	service := &serviceImpl{principal: commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-1",
	}}}
	if customerNo, err := service.currentCustomerNo(context.Background()); err != nil || customerNo != "customer-1" {
		t.Fatalf("active customer=%q err=%v", customerNo, err)
	}
}
