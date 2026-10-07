package reconciliation

import (
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

func TestBuildLinesClassifiesBoundedPaymentAndRefundDifferences(t *testing.T) {
	day := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	payments := gdb.Result{
		gdb.Record{"attempt_no": value("pay-ok"), "customer_ref_digest": value(strings.Repeat("a", 64)), "currency_code": value("CNY"), "amount_minor": value(1000), "external_trade_reference": value("trade-ok"), "status_code": value("succeeded")},
		gdb.Record{"attempt_no": value("pay-missing-provider"), "customer_ref_digest": value(strings.Repeat("b", 64)), "currency_code": value("CNY"), "amount_minor": value(2000), "external_trade_reference": value("trade-local"), "status_code": value("succeeded")},
	}
	refunds := gdb.Result{gdb.Record{"refund_attempt_no": value("refund-status"), "currency_code": value("CNY"), "amount_minor": value(500), "external_refund_reference": value("refund-ext"), "status_code": value("submitted")}}
	records := []SourceRecord{
		{RecordType: "payment", LocalSubjectNo: "pay-ok", ProviderEventReference: "event-ok", ExternalReference: "trade-ok", CurrencyCode: "CNY", AmountMinor: 1000, StatusCode: "succeeded", EvidenceDigest: strings.Repeat("1", 64), ObservedAt: day},
		{RecordType: "payment", LocalSubjectNo: "pay-local-missing", ProviderEventReference: "event-new", ExternalReference: "trade-new", CurrencyCode: "CNY", AmountMinor: 3000, StatusCode: "succeeded", EvidenceDigest: strings.Repeat("2", 64), ObservedAt: day},
		{RecordType: "refund", LocalSubjectNo: "refund-status", ProviderEventReference: "event-refund", ExternalReference: "refund-ext", CurrencyCode: "CNY", AmountMinor: 500, StatusCode: "refunded", EvidenceDigest: strings.Repeat("3", 64), ObservedAt: day},
	}
	lines, paymentCount, refundCount, err := buildLines("batch-1", records, payments, refunds)
	if err != nil {
		t.Fatal(err)
	}
	if paymentCount != 2 || refundCount != 1 || len(lines) != 4 {
		t.Fatalf("unexpected counts payments=%d refunds=%d lines=%d", paymentCount, refundCount, len(lines))
	}
	codes := map[string]bool{}
	for _, line := range lines {
		codes[line.DifferenceCode] = true
	}
	for _, expected := range []string{"none", "local_missing", "provider_missing", "status_mismatch"} {
		if !codes[expected] {
			t.Fatalf("missing difference %s in %+v", expected, codes)
		}
	}
}

func TestParseProviderStatementsRecognizesAlipayAndWechatRows(t *testing.T) {
	day := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	alipay := "商户订单号,支付宝交易号,订单金额（元）,交易状态\npay-1,ali-1,12.34,成功\n"
	records, err := parseStatement(paymentcap.ProviderAlipay, []byte(alipay), day, 10)
	if err != nil || len(records) != 1 || records[0].AmountMinor != 1234 || records[0].CurrencyCode != "CNY" {
		t.Fatalf("alipay records=%+v err=%v", records, err)
	}
	wechat := "商户订单号,微信订单号,货币种类,应结订单金额,交易状态,商户退款单号,微信退款单号,退款金额,退款状态\npay-2,wx-1,CNY,10.00,成功,refund-1,wxr-1,2.00,成功\n"
	records, err = parseStatement(paymentcap.ProviderWechatPay, []byte(wechat), day, 10)
	if err != nil || len(records) != 2 || records[1].RecordType != "refund" || records[1].AmountMinor != 200 {
		t.Fatalf("wechat records=%+v err=%v", records, err)
	}
}

func TestFixedSupplementRejectionReasons(t *testing.T) {
	for _, value := range []string{"evidence_insufficient", "ownership_mismatch", "amount_or_currency_mismatch", "provider_reference_conflict", "already_confirmed"} {
		if !validRejection(value) {
			t.Fatalf("expected %s to be valid", value)
		}
	}
	if validRejection("free_text") {
		t.Fatal("free-text rejection must stay invalid")
	}
}

func value(input any) gdb.Value { return gvar.New(input) }
