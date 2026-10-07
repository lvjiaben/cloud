// This file verifies that account financial projections preserve exact
// approved-credit facts without presenting them as cash paid.

package account

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/billing"
)

func TestFinancialRenewalServiceMappingUsesGenericEntitlementIdentity(t *testing.T) {
	mapped := toAPIFinancialRenewalServices([]billing.RenewalServiceDirectoryItem{{
		EntitlementNo: "ENT_mapping_1", EntitlementVersion: 3, ProductNameZhCN: "证书服务",
		ProductNameEnUS: "Certificate service", CurrencyCode: "CNY",
	}})
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("marshal renewal services: %v", err)
	}
	for _, forbidden := range []string{"productTypeCode", "runtimeStatusCode", "renewalModeCode", "serviceNo", "serviceVersion"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("renewal entitlement exposed obsolete server fact %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"entitlementNo":"ENT_mapping_1"`) ||
		!strings.Contains(string(encoded), `"entitlementVersion":3`) {
		t.Fatalf("renewal entitlement omitted controlled action identity: %s", encoded)
	}
}

func TestFinancialCreditMappingPreservesExactNonCashAmount(t *testing.T) {
	settlement := toAPIFinancialCreditSettlement(&billing.AccountCreditPayment{
		SettlementNo: "FST_mapping_1", InvoiceNo: "INV_mapping_1",
		OrderNo: "ORD_mapping_1", CurrencyCode: "CNY",
		CreditAmountMinor: math.MaxInt64, StatusCode: "credit_settled", Reused: true,
	})
	if settlement == nil || settlement.CreditAmountMinor != "9223372036854775807" ||
		settlement.StatusCode != "credit_settled" || !settlement.Reused {
		t.Fatalf("credit settlement mapping lost exact semantics: %+v", settlement)
	}

	now := time.UnixMilli(1_786_406_400_123)
	invoice := toAPIFinancialInvoice(billing.InvoiceDirectoryItem{
		InvoiceNo: "INV_mapping_1", OrderNo: "ORD_mapping_1", SourceCode: "order",
		CurrencyCode: "CNY", StatusCode: "credit_settled", ItemCount: 1,
		TotalAmountMinor: math.MaxInt64, CreditAmountMinor: math.MaxInt64,
		PaymentDueAt: now.Add(time.Hour), SettledAt: &now, CreatedAt: now,
	})
	if invoice.CreditAmountMinor != "9223372036854775807" ||
		invoice.PaidAmountMinor != "0" || invoice.OutstandingAmountMinor != "0" {
		t.Fatalf("invoice credit was exposed as cash or lost precision: %+v", invoice)
	}
}
