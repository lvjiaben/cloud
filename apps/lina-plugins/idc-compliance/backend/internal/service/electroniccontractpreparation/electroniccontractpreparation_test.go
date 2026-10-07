package electroniccontractpreparation

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/contractcap"
)

func TestSnapshotBundleFreezesSevenDayPreview(t *testing.T) {
	createdAt := time.Date(2026, time.August, 15, 8, 30, 0, 0, time.UTC)
	order := validOrderSnapshot()
	defer order.Clear()
	preview, err := order.BuildPreviewVariables("CTR_0123456789ABCDEF")
	if err != nil {
		t.Fatalf("build preview variables: %v", err)
	}
	defer preview.Clear()

	bundle, digests, payload, err := newSnapshotBundle(
		bytes.NewReader(bytes.Repeat([]byte{0x31}, bundleSaltBytes)),
		"CTR_0123456789ABCDEF", createdAt, order, preview, testCustomerSigner(order), testProviderLaunch(order),
	)
	if err != nil {
		t.Fatalf("create snapshot bundle: %v", err)
	}
	defer bundle.Clear()
	defer clear(payload)
	if !bundle.SigningExpiresAt.Equal(createdAt.Add(7 * 24 * time.Hour)) {
		t.Fatalf("signing expiry=%s want=%s", bundle.SigningExpiresAt, createdAt.Add(SigningValidity))
	}
	if bundle.Preview.SigningDate != "" ||
		bundle.Preview.SigningDateNoticeCode != "filled_after_authoritative_signature" {
		t.Fatalf("pre-sign date was not kept blank: %+v", bundle.Preview)
	}
	if !validHexDigest(digests.Variables) || !validHexDigest(digests.Signers) ||
		!validHexDigest(digests.Order) {
		t.Fatalf("snapshot digests are invalid: %+v", digests)
	}

	decoded, err := decodeSnapshotBundle(payload, digests)
	if err != nil {
		t.Fatalf("decode snapshot bundle: %v", err)
	}
	decoded.Clear()

	var tampered snapshotBundle
	if err = json.Unmarshal(payload, &tampered); err != nil {
		t.Fatalf("decode tamper fixture: %v", err)
	}
	tampered.Preview.AmountMinor++
	tamperedPayload, err := json.Marshal(&tampered)
	tampered.Clear()
	if err != nil {
		t.Fatalf("encode tampered snapshot: %v", err)
	}
	defer clear(tamperedPayload)
	if decoded, err = decodeSnapshotBundle(tamperedPayload, digests); err == nil {
		decoded.Clear()
		t.Fatal("tampered snapshot bundle passed its integrity check")
	}
}

func TestSnapshotDigestsUsePerContractSalt(t *testing.T) {
	createdAt := time.Date(2026, time.August, 15, 8, 30, 0, 0, time.UTC)
	orderA := validOrderSnapshot()
	previewA, err := orderA.BuildPreviewVariables("CTR_0123456789ABCDEF")
	if err != nil {
		t.Fatal(err)
	}
	bundleA, digestsA, payloadA, err := newSnapshotBundle(
		bytes.NewReader(bytes.Repeat([]byte{0x41}, bundleSaltBytes)),
		"CTR_0123456789ABCDEF", createdAt, orderA, previewA, testCustomerSigner(orderA), testProviderLaunch(orderA),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer bundleA.Clear()
	defer clear(payloadA)

	orderB := validOrderSnapshot()
	previewB, err := orderB.BuildPreviewVariables("CTR_0123456789ABCDEF")
	if err != nil {
		t.Fatal(err)
	}
	bundleB, digestsB, payloadB, err := newSnapshotBundle(
		bytes.NewReader(bytes.Repeat([]byte{0x42}, bundleSaltBytes)),
		"CTR_0123456789ABCDEF", createdAt, orderB, previewB, testCustomerSigner(orderB), testProviderLaunch(orderB),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer bundleB.Clear()
	defer clear(payloadB)

	if equalSnapshotDigests(digestsA, digestsB) {
		t.Fatal("two contracts with different random salts produced identical snapshot digests")
	}
}

func TestBuildContractFlowRequestUsesFrozenTemplateOrderAndOfficialFields(t *testing.T) {
	createdAt := time.Date(2026, time.August, 15, 8, 30, 0, 0, time.UTC)
	order := validOrderSnapshot()
	defer order.Clear()
	preview, err := order.BuildPreviewVariables("CTR_0123456789ABCDEF")
	if err != nil {
		t.Fatal(err)
	}
	defer preview.Clear()
	bundle := &snapshotBundle{
		Version: bundleVersion, ContractNo: preview.ContractNo, CreatedAt: createdAt,
		SigningExpiresAt: createdAt.Add(SigningValidity), Order: cloneFrozenOrderSnapshot(order),
		Preview: clonePreviewVariables(preview), CustomerSigner: cloneCustomerSigner(testCustomerSigner(order)),
		ProviderLaunch: testProviderLaunch(order).Clone(),
	}
	defer bundle.Clear()
	service := &serviceImpl{artifactTexts: testArtifactTextCatalog(t)}
	request, err := service.buildContractFlowRequest(bundle)
	if err != nil {
		t.Fatalf("build contract flow request: %v", err)
	}
	if request.TemplateRef != bundle.ProviderLaunch.TemplateRef || request.DeadlineUnix != bundle.SigningExpiresAt.Unix() ||
		len(request.FormFields) != 2 || request.FormFields[0].ComponentName != "合同编号" ||
		request.FormFields[1].TypeCode != "DYNAMIC_TABLE" || len(request.Approvers) != 2 ||
		request.Approvers[0].SequenceNo != 1 || request.Approvers[0].PartyCode != providerPartyCustomer ||
		request.Approvers[0].Mobile != "13800001234" || request.Approvers[1].SequenceNo != 2 ||
		request.Approvers[1].PartyCode != providerPartyMerchant {
		t.Fatalf("unexpected contract flow request: %+v", request)
	}
	var table serviceTablePayload
	if err = json.Unmarshal([]byte(request.FormFields[1].Value), &table); err != nil ||
		table.RowCount != 2 || len(table.Headers) != 4 || table.Headers[0].Content != "服务名称" || len(table.Body.Cells) != 8 ||
		table.Body.Cells[0].Content != "独立服务器" || table.Body.Cells[3].Content != "999.00" {
		t.Fatalf("unexpected dynamic service table: %+v err=%v", table, err)
	}
}

func TestContractLaunchFormattingFailsClosed(t *testing.T) {
	service := &serviceImpl{artifactTexts: testArtifactTextCatalog(t)}
	if value, err := formatMinorAmount("CNY", 129900); err != nil || value != "1299.00" {
		t.Fatalf("CNY value=%q err=%v", value, err)
	}
	if value, err := formatMinorAmount("JPY", 1299); err != nil || value != "1299" {
		t.Fatalf("JPY value=%q err=%v", value, err)
	}
	if value, err := service.localizedServiceTerm("zh-CN", "month", 1, 12); err != nil || value != "12个月" {
		t.Fatalf("Chinese term=%q err=%v", value, err)
	}
	if value, err := service.localizedServiceTerm("en-US", "year", 1, 1); err != nil || value != "1 year" {
		t.Fatalf("English term=%q err=%v", value, err)
	}
	if _, err := service.localizedServiceTerm("en-US", "week", 1, 1); err == nil {
		t.Fatal("unsupported period unit was accepted")
	}
}

func testArtifactTextCatalog(t *testing.T) ArtifactTextCatalog {
	t.Helper()
	resources := map[string][]byte{
		"i18n/en-US/electronic-contract.json": []byte(`{
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.service":"Service",
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.term":"Term",
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.quantity":"Quantity",
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.lineAmount":"Line amount",
  "plugin.idc-compliance.contract.artifact.serviceTerm.once.one":"time",
  "plugin.idc-compliance.contract.artifact.serviceTerm.once.other":"times",
  "plugin.idc-compliance.contract.artifact.serviceTerm.hour.one":"hour",
  "plugin.idc-compliance.contract.artifact.serviceTerm.hour.other":"hours",
  "plugin.idc-compliance.contract.artifact.serviceTerm.day.one":"day",
  "plugin.idc-compliance.contract.artifact.serviceTerm.day.other":"days",
  "plugin.idc-compliance.contract.artifact.serviceTerm.month.one":"month",
  "plugin.idc-compliance.contract.artifact.serviceTerm.month.other":"months",
  "plugin.idc-compliance.contract.artifact.serviceTerm.year.one":"year",
  "plugin.idc-compliance.contract.artifact.serviceTerm.year.other":"years"
}`),
		"i18n/zh-CN/electronic-contract.json": []byte(`{
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.service":"服务名称",
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.term":"服务期限",
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.quantity":"数量",
  "plugin.idc-compliance.contract.artifact.serviceTable.headers.lineAmount":"分项金额",
  "plugin.idc-compliance.contract.artifact.serviceTerm.once.one":"次",
  "plugin.idc-compliance.contract.artifact.serviceTerm.once.other":"次",
  "plugin.idc-compliance.contract.artifact.serviceTerm.hour.one":"小时",
  "plugin.idc-compliance.contract.artifact.serviceTerm.hour.other":"小时",
  "plugin.idc-compliance.contract.artifact.serviceTerm.day.one":"天",
  "plugin.idc-compliance.contract.artifact.serviceTerm.day.other":"天",
  "plugin.idc-compliance.contract.artifact.serviceTerm.month.one":"个月",
  "plugin.idc-compliance.contract.artifact.serviceTerm.month.other":"个月",
  "plugin.idc-compliance.contract.artifact.serviceTerm.year.one":"年",
  "plugin.idc-compliance.contract.artifact.serviceTerm.year.other":"年"
}`),
	}
	catalog, err := parseArtifactTextCatalog(resources)
	if err != nil {
		t.Fatalf("parse test contract artifact translations: %v", err)
	}
	return catalog
}

func TestAuthoritativeSigningDateRequiresTencentResult(t *testing.T) {
	service := &serviceImpl{}
	signedAt := time.Date(2026, time.August, 15, 9, 0, 0, 0, time.UTC)
	if err := service.ValidateAuthoritativeSigningDate(ProviderTencentESSBasic, signedAt); err != nil {
		t.Fatalf("Tencent authoritative signing date rejected: %v", err)
	}
	if err := service.ValidateAuthoritativeSigningDate("browser_return", signedAt); err == nil {
		t.Fatal("browser return was accepted as an authoritative signing date")
	}
	if err := service.ValidateAuthoritativeSigningDate(ProviderTencentESSBasic, time.Time{}); err == nil {
		t.Fatal("empty Tencent signing date was accepted")
	}
}

func TestContractPreparationInputValidation(t *testing.T) {
	if !validIdempotencyKey("contract-create-00000001") {
		t.Fatal("valid idempotency key was rejected")
	}
	if validIdempotencyKey("short") || validIdempotencyKey("contract-create\n00000001") {
		t.Fatal("invalid idempotency key was accepted")
	}
	if !validBusinessNo("ORDER_20260815-001", 40) || validBusinessNo("ORDER/20260815", 40) {
		t.Fatal("business number validation is inconsistent")
	}
	if !validHexDigest(digestText("test\x00", "value")) || validHexDigest(string(bytes.Repeat([]byte{'A'}, 64))) {
		t.Fatal("digest validation is inconsistent")
	}
}

func TestSigningReturnURLAllowsOnlyHTTPSOrExplicitLoopbackHTTP(t *testing.T) {
	for _, valid := range []string{
		"https://customer.example.test/account/contracts?signing_return=1",
		"http://127.0.0.1:6032/account/contracts?signing_return=1",
		"http://localhost:6032/account/contracts?signing_return=1",
	} {
		if !validSigningReturnURL(valid) {
			t.Errorf("safe signing return URL rejected: %q", valid)
		}
	}
	for _, invalid := range []string{
		"http://customer.example.test/account/contracts?signing_return=1",
		"https://customer.example.test/other?signing_return=1",
		"https://user@customer.example.test/account/contracts?signing_return=1",
	} {
		if validSigningReturnURL(invalid) {
			t.Errorf("unsafe signing return URL accepted: %q", invalid)
		}
	}
}

func validOrderSnapshot() *contractcap.OrderSnapshot {
	return &contractcap.OrderSnapshot{
		OrderNo: "ORDER_20260815-001", StatusCode: "pending_payment", CurrencyCode: "CNY",
		AmountMinor: 129900, RequiresContract: true, OrderVersion: 3,
		Template: contractcap.TemplateSnapshot{
			TemplateNo: "TPL_001", VersionNo: 2, Name: "服务器服务合同",
			LanguageCode: "zh-CN", ScopeCode: "personal", SigningOrderCode: "customer_first",
		},
		Merchant: contractcap.MerchantIdentity{
			MerchantNo: "MERCHANT_001", LegalName: []byte("示例商家有限公司"),
			CountryCode: "CN", Version: 2,
		},
		Customer: contractcap.VerifiedIdentity{
			CustomerType: contractcap.CustomerTypeIndividual, CustomerNo: "CUSTOMER_001",
			LegalName: []byte("示例客户"), SubjectLookupDigest: digestText("identity\x00", "customer"),
			VerificationCaseNo: "CASE_001", VerificationVersion: 4,
			VerifiedAt:    time.Date(2026, time.August, 15, 8, 0, 0, 0, time.UTC),
			VerifiedPhone: []byte("+8613800001234"), PhoneContactNo: "CONTACT_001", PhoneContactVersion: 2,
			PhoneVerifiedAt: time.Date(2026, time.August, 15, 8, 5, 0, 0, time.UTC),
		},
		Items: []contractcap.OrderItem{
			{
				OrderItemNo: "ITEM_001", ProductNo: "PRODUCT_001", ProductNameZhCN: "独立服务器",
				ProductNameEnUS: "Dedicated Server", ContractProductName: "独立服务器",
				Quantity: 1, PurchaseCycles: 1, PeriodUnitCode: "month", PeriodCount: 12,
				RequiresContract: true, AmountMinor: 99900,
			},
			{
				OrderItemNo: "ITEM_002", ProductNo: "PRODUCT_002", ProductNameZhCN: "托管运维",
				ProductNameEnUS: "Managed Operations", ContractProductName: "托管运维",
				Quantity: 1, PurchaseCycles: 1, PeriodUnitCode: "month", PeriodCount: 12,
				RequiresContract: true, AmountMinor: 30000,
			},
		},
		SnapshotObservedAt: time.Date(2026, time.August, 15, 8, 25, 0, 0, time.UTC),
	}
}

func testCustomerSigner(order *contractcap.OrderSnapshot) *customerSignerSnapshot {
	return &customerSignerSnapshot{SignerNo: "personal:" + order.Customer.VerificationCaseNo, CustomerNo: order.Customer.CustomerNo,
		CustomerType: order.Customer.CustomerType, Name: append([]byte(nil), order.Customer.LegalName...), Phone: append([]byte(nil), order.Customer.VerifiedPhone...),
		IdentityVerificationVersion: order.Customer.VerificationVersion, IdentityVerifiedAt: order.Customer.VerifiedAt,
		PhoneVerificationNo: order.Customer.PhoneContactNo, PhoneVerificationVersion: order.Customer.PhoneContactVersion,
		PhoneVerifiedAt: order.Customer.PhoneVerifiedAt, AuthorizationVersion: 1}
}

func testProviderLaunch(order *contractcap.OrderSnapshot) *providerLaunchSnapshot {
	customerOrder := int64(1)
	merchantOrder := int64(2)
	return &providerLaunchSnapshot{
		ProviderCode: ProviderTencentESSBasic, TemplateRef: "0123456789ABCDEF0123456789ABCDEF",
		ProviderVersion: "20260815001", ProviderSnapshotDigest: digestText("provider\x00", "snapshot"),
		MappingSetNo: "MAP_001", ScopeCode: order.Template.ScopeCode,
		SigningOrderCode: order.Template.SigningOrderCode, LanguageCode: order.Template.LanguageCode,
		Fields: []providerFieldSnapshot{
			{ComponentRef: "contract-number", Name: "合同编号", TypeCode: "TEXT", Required: true, ValueCode: "contract_no"},
			{ComponentRef: "service-lines", Name: "服务明细", TypeCode: "DYNAMIC_TABLE", Required: true, ValueCode: "service_lines"},
			{ComponentRef: "signing-date", Name: "签署日期", TypeCode: "SIGN_DATE", Required: true, ValueCode: "signing_date"},
		},
		Recipients: []providerRecipientSnapshot{
			{RecipientRef: "customer", SequenceNo: 1, TypeCode: providerRecipientIndividual, RequireSign: true, RoutingOrder: &customerOrder, SignComponentCount: 1, PartyCode: providerPartyCustomer},
			{RecipientRef: "merchant", SequenceNo: 2, TypeCode: providerRecipientPromoter, RequireSign: true, RoutingOrder: &merchantOrder, IsPromoter: true, SignComponentCount: 1, PartyCode: providerPartyMerchant},
		},
	}
}
