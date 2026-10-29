package marketing

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/xuri/excelize/v2"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const marketingIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type marketingInsightsAuthorizerStub struct {
	performanceCalls int
	abuseListCalls   int
	abuseReviewCalls int
	costListCalls    int
	costCorrectCalls int
	newcomerSetCalls int
}

type couponPrincipalStub struct{ customerNo string }

func (s couponPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{
		UserID: 88, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: s.customerNo, Version: 1,
	}, nil
}

func TestPublicCampaignsPostgreSQLFilterAndDeduplicateExposure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_category
 (category_no, name_zh_cn, name_en_us, status_code)
VALUES ('category-public', '公开云服务器', 'Public cloud servers', 'active');
INSERT INTO plugin_idc_core_catalog_product
 (product_no, category_no, status_code, public_slug,
  name_zh_cn, name_en_us, current_version_no)
VALUES ('product-public', 'category-public', 'active',
        'starter-public', '公开入门服务器', 'Public starter server', NULL);
INSERT INTO plugin_idc_core_catalog_product_version
 (version_no, product_no, version_number, status_code, source_code, published_at)
VALUES ('version-public', 'product-public', 1, 'published', 'manual', CURRENT_TIMESTAMP);
UPDATE plugin_idc_core_catalog_product
SET current_version_no='version-public' WHERE product_no='product-public';
INSERT INTO plugin_idc_core_marketing_campaign
 (campaign_no, campaign_code, name_zh_cn, name_en_us, description_zh_cn, description_en_us,
  campaign_type_code, target_segment_no, target_snapshot_no, starts_at, ends_at, priority,
  stack_policy_code, stack_group_code, status_code)
VALUES
 ('campaign-public', 'campaign_public', '公开活动', 'Public campaign', '公开活动说明', 'Public campaign description',
  'discount', NULL, NULL, CURRENT_TIMESTAMP-INTERVAL '1 hour', CURRENT_TIMESTAMP+INTERVAL '1 day',
  1, 'stackable', 'public', 'active'),
 ('campaign-targeted', 'campaign_targeted', '指定分群活动', 'Targeted campaign', '', '',
  'discount', 'segment-engagement', 'snapshot-engagement', CURRENT_TIMESTAMP-INTERVAL '1 hour',
  CURRENT_TIMESTAMP+INTERVAL '1 day', 2, 'stackable', 'targeted', 'active'),
 ('campaign-excluded', 'campaign_excluded', '排除活动', 'Excluded campaign', '', '',
  'discount', NULL, NULL, CURRENT_TIMESTAMP-INTERVAL '1 hour', CURRENT_TIMESTAMP+INTERVAL '1 day',
  3, 'stackable', 'excluded', 'active'),
 ('campaign-expired', 'campaign_expired', '过期活动', 'Expired campaign', '', '',
  'discount', NULL, NULL, CURRENT_TIMESTAMP-INTERVAL '2 days', CURRENT_TIMESTAMP-INTERVAL '1 day',
  4, 'stackable', 'expired', 'active'),
 ('campaign-wrong-product', 'campaign_wrong_product', '其他商品活动', 'Other product campaign', '', '',
  'discount', NULL, NULL, CURRENT_TIMESTAMP-INTERVAL '1 hour', CURRENT_TIMESTAMP+INTERVAL '1 day',
  5, 'stackable', 'other', 'active');
INSERT INTO plugin_idc_core_marketing_campaign_scope
 (campaign_no, scope_kind_code, match_mode_code, reference_code)
VALUES
 ('campaign-public', 'product', 'include', 'product-public'),
	 ('campaign-targeted', 'all_products', 'include', ''),
	 ('campaign-excluded', 'all_products', 'include', ''),
 ('campaign-excluded', 'product', 'exclude', 'product-public'),
	 ('campaign-expired', 'all_products', 'include', ''),
 ('campaign-wrong-product', 'product', 'include', 'different-product');
INSERT INTO plugin_idc_core_marketing_benefit_rule
 (benefit_no, campaign_no, campaign_type_code, benefit_type_code, currency_code,
  amount_minor, purchase_phase_code, cycle_scope_code)
VALUES
 ('benefit-public-cny', 'campaign-public', 'discount', 'fixed_discount', 'CNY', 100, 'both', 'all'),
 ('benefit-public-usd', 'campaign-public', 'discount', 'fixed_discount', 'USD', 100, 'both', 'all'),
 ('benefit-targeted', 'campaign-targeted', 'discount', 'fixed_discount', 'CNY', 100, 'both', 'all'),
 ('benefit-excluded', 'campaign-excluded', 'discount', 'fixed_discount', 'CNY', 100, 'both', 'all'),
 ('benefit-expired', 'campaign-expired', 'discount', 'fixed_discount', 'CNY', 100, 'both', 'all'),
 ('benefit-wrong-product', 'campaign-wrong-product', 'discount', 'fixed_discount', 'CNY', 100, 'both', 'all')`); err != nil {
		t.Fatalf("seed public campaigns: %v", err)
	}
	service, err := NewPublicCampaignService(database, bytes.Repeat([]byte{0x4d}, 32))
	if err != nil {
		t.Fatalf("create public campaign service: %v", err)
	}
	campaigns, err := service.List(ctx, PublicCampaignListInput{PublicSlug: "starter-public", CurrencyCode: "CNY"})
	if err != nil || len(campaigns) != 1 || campaigns[0].CampaignNo != "campaign-public" {
		t.Fatalf("public campaign projection=%+v err=%v", campaigns, err)
	}
	deviceID := "public-browser-device-001122334455"
	input := PublicCampaignExposureInput{
		PublicSlug: "starter-public", CampaignNo: "campaign-public", CurrencyCode: "CNY", DeviceID: deviceID,
	}
	first, err := service.RecordExposure(ctx, input)
	if err != nil || first == nil || !first.Recorded {
		t.Fatalf("first public exposure=%+v err=%v", first, err)
	}
	second, err := service.RecordExposure(ctx, input)
	if err != nil || second == nil || second.Recorded {
		t.Fatalf("deduplicated public exposure=%+v err=%v", second, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_marketing_public_exposure_state
SET window_started_at=CURRENT_TIMESTAMP-INTERVAL '31 minutes'
WHERE campaign_no='campaign-public' AND currency_code='CNY'`); err != nil {
		t.Fatalf("expire public exposure window: %v", err)
	}
	third, err := service.RecordExposure(ctx, input)
	if err != nil || third == nil || !third.Recorded {
		t.Fatalf("renewed public exposure=%+v err=%v", third, err)
	}
	usd := input
	usd.CurrencyCode = "USD"
	fourth, err := service.RecordExposure(ctx, usd)
	if err != nil || fourth == nil || !fourth.Recorded {
		t.Fatalf("currency-isolated public exposure=%+v err=%v", fourth, err)
	}
	var stored struct {
		Facts      int `orm:"facts"`
		RawDevices int `orm:"raw_devices"`
		States     int `orm:"states"`
	}
	if err = database.GetScan(ctx, &stored, `SELECT
 (SELECT count(*) FROM plugin_idc_core_marketing_effect_fact
  WHERE campaign_no='campaign-public' AND event_type_code='exposure') AS facts,
 (SELECT count(*) FROM plugin_idc_core_marketing_public_exposure_state
  WHERE device_digest=?) AS raw_devices,
 (SELECT count(*) FROM plugin_idc_core_marketing_public_exposure_state
  WHERE campaign_no='campaign-public') AS states`, deviceID); err != nil {
		t.Fatalf("inspect public exposure storage: %v", err)
	}
	if stored.Facts != 3 || stored.States != 2 || stored.RawDevices != 0 {
		t.Fatalf("public exposure storage=%+v", stored)
	}
}

func (s *marketingInsightsAuthorizerStub) AuthorizeMarketingPerformance(context.Context) (int, error) {
	s.performanceCalls++
	return 71, nil
}

func (s *marketingInsightsAuthorizerStub) AuthorizeMarketingAbuseList(context.Context) (int, error) {
	s.abuseListCalls++
	return 72, nil
}

func (s *marketingInsightsAuthorizerStub) AuthorizeMarketingAbuseReview(context.Context) (int, error) {
	s.abuseReviewCalls++
	return 73, nil
}

func (s *marketingInsightsAuthorizerStub) AuthorizeMarketingCostList(context.Context) (int, error) {
	s.costListCalls++
	return 74, nil
}

func (s *marketingInsightsAuthorizerStub) AuthorizeMarketingCostCorrect(context.Context) (int, error) {
	s.costCorrectCalls++
	return 75, nil
}

func (s *marketingInsightsAuthorizerStub) AuthorizeMarketingNewcomerSet(context.Context) (int, error) {
	s.newcomerSetCalls++
	return 76, nil
}

func TestMarketingInsightsPostgreSQLKeepsCurrencyAndReviewBoundaries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_effect_fact
 (fact_no, campaign_no, customer_no, event_type_code, currency_code, amount_minor,
  source_type_code, source_reference, content_digest, occurred_at)
VALUES
 ('mfx_test_exposure', 'campaign-coupon', NULL, 'exposure', 'CNY', 0,
  'integration_test', 'exposure-1', repeat('1', 64), CURRENT_TIMESTAMP - INTERVAL '2 hours'),
 ('mfx_test_claim', 'campaign-coupon', 'customer-a', 'claim', 'CNY', 0,
  'integration_test', 'claim-1', repeat('2', 64), CURRENT_TIMESTAMP - INTERVAL '90 minutes'),
 ('mfx_test_order', 'campaign-coupon', 'customer-a', 'order', 'CNY', 0,
  'integration_test', 'order-1', repeat('3', 64), CURRENT_TIMESTAMP - INTERVAL '1 hour'),
 ('mfx_test_receipt', 'campaign-coupon', 'customer-a', 'receipt', 'CNY', 1001,
  'integration_test', 'receipt-1', repeat('4', 64), CURRENT_TIMESTAMP - INTERVAL '50 minutes'),
 ('mfx_test_refund', 'campaign-coupon', 'customer-a', 'refund', 'CNY', 201,
  'integration_test', 'refund-1', repeat('5', 64), CURRENT_TIMESTAMP - INTERVAL '40 minutes'),
 ('mfx_test_cost', 'campaign-coupon', 'customer-a', 'cost', 'CNY', 300,
  'integration_test', 'cost-1', repeat('6', 64), CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
 ('mfx_test_points', 'campaign-coupon', 'customer-a', 'use', NULL, 0,
  'integration_test', 'points-1', repeat('7', 64), CURRENT_TIMESTAMP - INTERVAL '20 minutes');
INSERT INTO plugin_idc_core_marketing_abuse_case
 (case_no, campaign_no, customer_no, signal_code, related_account_count,
  evidence_summary_code, evidence_digest, window_started_at, window_ended_at, observed_count)
VALUES
 ('mab_test_duplicate', 'campaign-coupon', 'customer-a', 'duplicate_newcomer_claim', 2,
  'verified_subject_reused', repeat('8', 64), CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP, 2)`); err != nil {
		t.Fatalf("seed marketing insights: %v", err)
	}
	authorizer := &marketingInsightsAuthorizerStub{}
	audit := &marketingActionAuditIntegrationStub{}
	service, err := NewMarketingInsightsService(database, authorizer, audit)
	if err != nil {
		t.Fatalf("create marketing insights service: %v", err)
	}
	report, err := service.Performance(ctx, MarketingPerformanceInput{
		StartedAt: time.Now().Add(-24 * time.Hour), EndedAt: time.Now().Add(time.Minute),
	})
	if err != nil || report == nil || len(report.Rows) != 2 {
		t.Fatalf("read marketing performance report=%+v err=%v", report, err)
	}
	var moneyRow, pointsRow *MarketingPerformanceRow
	for index := range report.Rows {
		row := &report.Rows[index]
		if row.CurrencyCode == "CNY" {
			moneyRow = row
		} else if row.CurrencyCode == "" {
			pointsRow = row
		}
	}
	if moneyRow == nil || moneyRow.Exposure != 1 || moneyRow.Claim != 1 || moneyRow.Order != 1 ||
		moneyRow.ReceiptMinor != 1001 || moneyRow.RefundMinor != 201 || moneyRow.CostMinor != 300 ||
		moneyRow.NetMinor != 500 || !moneyRow.CostComplete || pointsRow == nil || pointsRow.Use != 1 ||
		pointsRow.ReceiptMinor != 0 || authorizer.performanceCalls != 1 {
		t.Fatalf("unexpected currency-safe performance money=%+v points=%+v authorizer=%+v", moneyRow, pointsRow, authorizer)
	}
	page, err := service.AbuseCases(ctx, MarketingAbuseSearchInput{Status: "pending_review", Limit: 10})
	if err != nil || len(page.Cases) != 1 || page.Cases[0].RelatedAccountCount != 2 || authorizer.abuseListCalls != 1 {
		t.Fatalf("read redacted abuse queue page=%+v err=%v", page, err)
	}
	reviewed, err := service.ReviewAbuse(ctx, MarketingAbuseReviewInput{
		CaseNo: "mab_test_duplicate", Decision: "confirmed_abuse",
		Reason: "verified duplicate newcomer claim", ExpectedVersion: 1,
	})
	if err != nil || reviewed.StatusCode != "confirmed_abuse" || reviewed.Version != 2 ||
		authorizer.abuseReviewCalls != 1 || len(audit.transitions) != 1 {
		t.Fatalf("review abuse result=%+v err=%v authorizer=%+v audit=%+v", reviewed, err, authorizer, audit.transitions)
	}
	restrictionValue, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_campaign_restriction WHERE campaign_no='campaign-coupon' AND customer_no='customer-a' AND status_code='active'`)
	if err != nil {
		t.Fatalf("read campaign restriction: %v", err)
	}
	activeCustomerValue, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_customer_subject WHERE customer_no='customer-a' AND status_code='active'`)
	if err != nil {
		t.Fatalf("read customer status: %v", err)
	}
	changedOrderValue, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_commerce_order WHERE customer_no='customer-a' AND status_code NOT IN ('pending_payment','payment_processing','paid','fulfilling','partially_fulfilled','fulfilled','cancelled','partially_refunded','refunded','manual_review')`)
	if err != nil {
		t.Fatalf("read order status safety: %v", err)
	}
	restrictionCount, activeCustomerCount, changedOrderCount := restrictionValue.Int(), activeCustomerValue.Int(), changedOrderValue.Int()
	if restrictionCount != 1 || activeCustomerCount != 1 || changedOrderCount != 0 {
		t.Fatalf("review boundary restriction=%d active_customer=%d changed_order=%d", restrictionCount, activeCustomerCount, changedOrderCount)
	}
}

func TestRegistrationAbuseSignalsPostgreSQLUseOnlyDigestsAndReviewWithoutCampaignRestriction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject
 (customer_no, customer_type_code, status_code, display_name)
VALUES ('customer-c', 'individual', 'active', 'Customer C')`); err != nil {
		t.Fatalf("seed third registration customer: %v", err)
	}
	recorder, err := NewRegistrationSignalRecorder(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create registration signal recorder: %v", err)
	}
	deviceID := "device-00112233445566778899aabbccddeeff0011"
	for index, customerNo := range []string{"customer-a", "customer-b", "customer-c"} {
		result, recordErr := recorder.RecordRegistration(ctx, RegistrationSignalInput{
			CustomerNo: customerNo, DeviceID: deviceID, SourceIP: "203.0.113.8",
		})
		if recordErr != nil || result == nil || (index < 2 && result.CreatedCaseCount != 0) ||
			(index == 2 && result.CreatedCaseCount != 2) {
			t.Fatalf("record registration index=%d result=%+v err=%v", index, result, recordErr)
		}
	}
	replay, err := recorder.RecordRegistration(ctx, RegistrationSignalInput{
		CustomerNo: "customer-c", DeviceID: "device-ffeeddccbbaa99887766554433221100ffee", SourceIP: "198.51.100.2",
	})
	if err != nil || !replay.Reused || replay.CreatedCaseCount != 0 {
		t.Fatalf("replay registration signal result=%+v err=%v", replay, err)
	}
	rawCount, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_registration_signal
WHERE device_digest=? OR ip_digest=?`, deviceID, "203.0.113.8")
	if err != nil || rawCount.Int() != 0 {
		t.Fatalf("raw registration source persisted count=%d err=%v", rawCount.Int(), err)
	}
	var cases []MarketingAbuseCase
	if err = database.GetScan(ctx, &cases, `SELECT case_no, COALESCE(campaign_no, '') AS campaign_no,
 customer_no, signal_code, status_code, related_account_count, evidence_summary_code,
 window_started_at, window_ended_at, observed_count, blocked_until,
 COALESCE(review_reason, '') AS review_reason, COALESCE(reviewed_by, 0) AS reviewed_by,
 reviewed_at, version, created_at, updated_at
FROM plugin_idc_core_marketing_abuse_case WHERE signal_code='bulk_registration' ORDER BY evidence_summary_code`); err != nil {
		t.Fatalf("read registration abuse cases: %v", err)
	}
	if len(cases) != 2 || cases[0].CampaignNo != "" || cases[1].CampaignNo != "" ||
		cases[0].RelatedAccountCount != 3 || cases[1].RelatedAccountCount != 3 {
		t.Fatalf("registration abuse cases=%+v", cases)
	}
	service, err := NewMarketingInsightsService(database, &marketingInsightsAuthorizerStub{}, &marketingActionAuditIntegrationStub{})
	if err != nil {
		t.Fatalf("create marketing insights: %v", err)
	}
	if _, err = service.ReviewAbuse(ctx, MarketingAbuseReviewInput{
		CaseNo: cases[0].CaseNo, Decision: "confirmed_abuse", Reason: "confirmed bulk registration source", ExpectedVersion: 1,
	}); err != nil {
		t.Fatalf("review campaign-free registration case: %v", err)
	}
	restrictionCount, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_campaign_restriction WHERE case_no=?`, cases[0].CaseNo)
	if err != nil || restrictionCount.Int() != 0 {
		t.Fatalf("campaign-free case created restriction count=%d err=%v", restrictionCount.Int(), err)
	}
}

func TestNewcomerCampaignPostgreSQLBlocksSecondVerifiedSubjectClaim(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_individual_profile
 (customer_no, legal_name, legal_name_masked, identity_identifier_masked,
  identity_lookup_digest, identity_case_no, identity_status_code, identity_verified_at)
VALUES ('customer-a', 'Verified Customer', 'V********r', '11**************11',
        repeat('9',64), 'identity-case-newcomer', 'verified', CURRENT_TIMESTAMP);
UPDATE plugin_idc_core_marketing_campaign
SET is_newcomer_exclusive=TRUE WHERE campaign_no='campaign-coupon'`); err != nil {
		t.Fatalf("seed newcomer campaign and identity: %v", err)
	}
	points, err := NewPointsService(database)
	if err != nil {
		t.Fatalf("create points service: %v", err)
	}
	grants, err := NewValueGrantService(database)
	if err != nil {
		t.Fatalf("create grant service: %v", err)
	}
	financial, err := billing.NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatalf("create financial adjustment service: %v", err)
	}
	audit := &marketingActionAuditIntegrationStub{}
	service, err := NewManagementActionService(database, audit, &managementActionAuthorizerStub{}, points, grants, financial)
	if err != nil {
		t.Fatalf("create management action service: %v", err)
	}
	if second, secondErr := service.IssueAsset(ctx, AssetIssueInput{
		RequestNo: "newcomer-claim-request-0001", CampaignNo: "campaign-coupon",
		BenefitNo: "benefit-coupon", CustomerNo: "customer-a", ReasonCode: "customer_service",
	}); second != nil || !bizerr.Is(secondErr, CodeManagementActionConflict) {
		t.Fatalf("second newcomer claim result=%+v err=%v", second, secondErr)
	}
	assetCount, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_promotion_asset
WHERE customer_no='customer-a' AND campaign_no='campaign-coupon'`)
	if err != nil || assetCount.Int() != 1 {
		t.Fatalf("newcomer asset count=%d err=%v", assetCount.Int(), err)
	}
	var abuse MarketingAbuseCase
	if err = database.GetScan(ctx, &abuse, `SELECT case_no, campaign_no, customer_no, signal_code,
 status_code, related_account_count, evidence_summary_code, window_started_at, window_ended_at,
 observed_count, blocked_until, COALESCE(review_reason,'') AS review_reason,
 COALESCE(reviewed_by,0) AS reviewed_by, reviewed_at, version, created_at, updated_at
FROM plugin_idc_core_marketing_abuse_case WHERE signal_code='duplicate_newcomer_claim'`); err != nil {
		t.Fatalf("read duplicate newcomer case: %v", err)
	}
	if abuse.CampaignNo != "campaign-coupon" || abuse.CustomerNo != "customer-a" || abuse.ObservedCount != 2 ||
		abuse.EvidenceSummaryCode != "verified_subject_reused" {
		t.Fatalf("duplicate newcomer case=%+v", abuse)
	}
	insights, err := NewMarketingInsightsService(database, &marketingInsightsAuthorizerStub{}, audit)
	if err != nil {
		t.Fatalf("create marketing insights: %v", err)
	}
	if _, err = insights.ReviewAbuse(ctx, MarketingAbuseReviewInput{
		CaseNo: abuse.CaseNo, Decision: "confirmed_abuse", Reason: "confirmed repeated verified newcomer claim", ExpectedVersion: 1,
	}); err != nil {
		t.Fatalf("confirm duplicate newcomer case: %v", err)
	}
	evaluator, err := New(database, Config{})
	if err != nil {
		t.Fatalf("create marketing evaluator: %v", err)
	}
	evaluationInput := promotionIntegrationInput("customer-a", "quote-newcomer-restricted")
	evaluationInput.ProductNo = "coupon-product"
	evaluationInput.AssetNumbers = []string{"asset-coupon-a"}
	decision, err := evaluator.Evaluate(ctx, evaluationInput)
	if err != nil || decision == nil || len(decision.Applied) != 0 {
		t.Fatalf("campaign restriction evaluation=%+v err=%v", decision, err)
	}
}

func TestCampaignNewcomerSettingPostgreSQLIsAuditedAndIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	authorizer := &marketingInsightsAuthorizerStub{}
	audit := &marketingActionAuditIntegrationStub{}
	service, err := NewMarketingInsightsService(database, authorizer, audit)
	if err != nil {
		t.Fatalf("create marketing insights: %v", err)
	}
	versionValue, err := database.GetValue(ctx, `SELECT version FROM plugin_idc_core_marketing_campaign WHERE campaign_no='campaign-coupon'`)
	if err != nil {
		t.Fatalf("read campaign version: %v", err)
	}
	version := versionValue.Int64()
	input := MarketingCampaignNewcomerInput{
		RequestNo: "newcomer-setting-request-0001", CampaignNo: "campaign-coupon",
		Enabled: true, ExpectedVersion: version,
	}
	first, err := service.SetCampaignNewcomer(ctx, input)
	if err != nil || first == nil || !first.Enabled || first.Version != version+1 {
		t.Fatalf("first newcomer setting=%+v err=%v", first, err)
	}
	if authorizer.newcomerSetCalls != 1 || len(audit.transitions) != 1 ||
		audit.transitions[0].RequiredPermission != permissionMarketingNewcomerSet {
		t.Fatalf("newcomer permission/audit calls=%d transitions=%+v", authorizer.newcomerSetCalls, audit.transitions)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_marketing_campaign SET version=version+1, updated_at=CURRENT_TIMESTAMP WHERE campaign_no='campaign-coupon'`); err != nil {
		t.Fatalf("advance campaign version: %v", err)
	}
	replay, err := service.SetCampaignNewcomer(ctx, input)
	if err != nil || replay == nil || *replay != *first || len(audit.transitions) != 1 {
		t.Fatalf("idempotent newcomer replay first=%+v replay=%+v audit=%d err=%v", first, replay, len(audit.transitions), err)
	}
	changed := input
	changed.Enabled = false
	if result, changedErr := service.SetCampaignNewcomer(ctx, changed); result != nil || !bizerr.Is(changedErr, CodeInsightsConflict) {
		t.Fatalf("changed newcomer replay result=%+v err=%v", result, changedErr)
	}
}

func TestCouponClaimPostgreSQLHidesFailuresAndBlocksBothSources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	key := bytes.Repeat([]byte{0x5a}, 32)
	service, err := NewCouponClaimService(database, couponPrincipalStub{customerNo: "customer-b"}, key)
	if err != nil {
		t.Fatalf("create coupon claim service: %v", err)
	}
	implementation := service.(*couponClaimServiceImpl)
	validCode := "SAVE2026AB"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_promotion_asset
 (asset_no, campaign_no, benefit_no, benefit_type_code, asset_type_code, claim_code_digest,
  benefit_snapshot_digest, currency_code, face_amount_minor, usage_limit, valid_from, valid_until, status_code)
VALUES ('asset-code-b','campaign-coupon','benefit-coupon','fixed_discount','coupon',?,
 repeat('9',64),'CNY',500,1,CURRENT_TIMESTAMP-INTERVAL '1 hour',CURRENT_TIMESTAMP+INTERVAL '1 day','issued')`,
		implementation.digestBytes("coupon-code", validCode)); err != nil {
		t.Fatalf("seed issued coupon: %v", err)
	}
	input := CouponClaimInput{Code: "WRONG2026", DeviceID: "browser-device-00000001", SourceIP: "203.0.113.25"}
	for attempt := 1; attempt <= 10; attempt++ {
		if item, claimErr := service.Claim(ctx, input); item != nil || !bizerr.Is(claimErr, CodeCouponInvalid) {
			t.Fatalf("wrong code attempt %d item=%+v err=%v", attempt, item, claimErr)
		}
	}
	caseCount, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_abuse_case WHERE signal_code='coupon_bruteforce'`)
	if err != nil || caseCount.Int() != 2 {
		t.Fatalf("coupon abuse cases=%d err=%v", caseCount.Int(), err)
	}
	if item, claimErr := service.Claim(ctx, CouponClaimInput{Code: "save2026ab", DeviceID: input.DeviceID, SourceIP: input.SourceIP}); item != nil || !bizerr.Is(claimErr, CodeCouponInvalid) {
		t.Fatalf("blocked valid claim item=%+v err=%v", item, claimErr)
	}
	owner, err := database.GetValue(ctx, `SELECT COALESCE(customer_no,'') FROM plugin_idc_core_marketing_promotion_asset WHERE asset_no='asset-code-b'`)
	if err != nil || owner.String() != "" {
		t.Fatalf("blocked claim changed owner=%q err=%v", owner.String(), err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_marketing_coupon_source_state SET blocked_until=CURRENT_TIMESTAMP-INTERVAL '1 second', window_started_at=CURRENT_TIMESTAMP-INTERVAL '11 minutes'`); err != nil {
		t.Fatalf("expire coupon source block: %v", err)
	}
	claimed, err := service.Claim(ctx, CouponClaimInput{Code: "save2026ab", DeviceID: input.DeviceID, SourceIP: input.SourceIP})
	if err != nil || claimed == nil || claimed.AssetNo != "asset-code-b" || claimed.Status != "active" {
		t.Fatalf("claim valid coupon=%+v err=%v", claimed, err)
	}
	items, err := service.ListMine(ctx)
	if err != nil || len(items) != 1 || items[0].AssetNo != "asset-code-b" {
		t.Fatalf("list claimed coupons=%+v err=%v", items, err)
	}
	for _, raw := range []string{input.DeviceID, input.SourceIP, validCode} {
		value, queryErr := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_coupon_source_state WHERE source_digest=?`, raw)
		if queryErr != nil || value.Int() != 0 {
			t.Fatalf("raw coupon signal persisted value=%q count=%d err=%v", raw, value.Int(), queryErr)
		}
	}
}

func TestCouponBatchPostgreSQLReturnsOnlyCompleteXLSXCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	audit := &marketingActionAuditIntegrationStub{}
	key := bytes.Repeat([]byte{0x6c}, 32)
	service, err := NewCouponBatchService(database, audit, &managementActionAuthorizerStub{}, key)
	if err != nil {
		t.Fatalf("create coupon batch service: %v", err)
	}
	file, err := service.Generate(ctx, CouponBatchGenerateInput{
		RequestNo: "coupon-batch-request-0001", CampaignNo: "campaign-coupon",
		BenefitNo: "benefit-coupon", CodeLength: 12, Count: 3,
	})
	if err != nil || file == nil || len(file.Body) == 0 || file.Count != 3 {
		t.Fatalf("generate coupon batch file=%+v err=%v", file, err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(file.Body))
	if err != nil {
		t.Fatalf("open coupon workbook: %v", err)
	}
	defer workbook.Close()
	rows, err := workbook.GetRows("Coupon Codes")
	if err != nil || len(rows) != 4 || len(rows[1]) < 4 || len(rows[1][3]) != 12 {
		t.Fatalf("coupon workbook rows=%+v err=%v", rows, err)
	}
	code := rows[1][3]
	var stored struct {
		Assets        int `orm:"assets"`
		Batches       int `orm:"batches"`
		DigestMatches int `orm:"digest_matches"`
	}
	if err = database.GetScan(ctx, &stored, `SELECT
 (SELECT count(*) FROM plugin_idc_core_marketing_promotion_asset WHERE coupon_batch_no=?) AS assets,
 (SELECT count(*) FROM plugin_idc_core_marketing_coupon_batch_request WHERE batch_no=?) AS batches,
 (SELECT count(*) FROM plugin_idc_core_marketing_promotion_asset WHERE claim_code_digest=?) AS digest_matches`,
		file.BatchNo, file.BatchNo, service.(*couponBatchServiceImpl).couponDigest(code)); err != nil {
		t.Fatalf("inspect coupon batch storage: %v", err)
	}
	if stored.Assets != 3 || stored.Batches != 1 || stored.DigestMatches != 1 || len(audit.transitions) != 1 {
		t.Fatalf("coupon batch storage=%+v audit=%+v", stored, audit.transitions)
	}
	claimService, err := NewCouponClaimService(database, couponPrincipalStub{customerNo: "customer-b"}, key)
	if err != nil {
		t.Fatalf("create coupon claim service: %v", err)
	}
	claimable, err := database.GetValue(ctx, `SELECT asset.status_code='issued' AND asset.customer_no IS NULL
 AND asset.valid_from<=CURRENT_TIMESTAMP AND asset.valid_until>CURRENT_TIMESTAMP
 AND campaign.status_code='active' AND campaign.starts_at<=CURRENT_TIMESTAMP AND campaign.ends_at>CURRENT_TIMESTAMP
FROM plugin_idc_core_marketing_promotion_asset asset
JOIN plugin_idc_core_marketing_campaign campaign ON campaign.campaign_no=asset.campaign_no
	WHERE asset.claim_code_digest=?`, claimService.(*couponClaimServiceImpl).digestBytes("coupon-code", strings.ToUpper(code)))
	if err != nil || !claimable.Bool() {
		t.Fatalf("generated coupon must be immediately claimable: value=%v err=%v", claimable, err)
	}
	claimed, err := claimService.Claim(ctx, CouponClaimInput{
		Code: strings.ToLower(code), DeviceID: "batch-coupon-browser-device-0001", SourceIP: "198.51.100.44",
	})
	if err != nil || claimed == nil {
		t.Fatalf("claim generated coupon=%+v err=%v", claimed, err)
	}
	if replay, replayErr := service.Generate(ctx, CouponBatchGenerateInput{
		RequestNo: "coupon-batch-request-0001", CampaignNo: "campaign-coupon",
		BenefitNo: "benefit-coupon", CodeLength: 12, Count: 3,
	}); replay != nil || !bizerr.Is(replayErr, CodeManagementActionConflict) {
		t.Fatalf("completed batch must not reveal codes again: replay=%+v err=%v", replay, replayErr)
	}
}

func TestEffectReconcilerPostgreSQLRunsRefundArbitrageScan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	service, err := NewEffectReconciler(database)
	if err != nil {
		t.Fatalf("create effect reconciler: %v", err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_commerce_quote_snapshot
 (quote_no,customer_no,customer_subject_version,source_code,currency_code,item_count,
  subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,
  input_digest,pricing_rule_digest,eligibility_digest,inventory_digest,content_digest,expires_at)
SELECT 'quote-arb-'||i,'customer-a',1,'customer','CNY',1,1000,100,900,0,900,
       md5('i'||i)||md5('ii'||i),md5('p'||i)||md5('pp'||i),md5('e'||i)||md5('ee'||i),
       md5('v'||i)||md5('vv'||i),md5('c'||i)||md5('cc'||i),CURRENT_TIMESTAMP+INTERVAL '1 day'
FROM generate_series(1,5) i;
INSERT INTO plugin_idc_core_commerce_order
 (order_no,customer_no,quote_no,currency_code,source_code,status_code,risk_decision_code,
  item_count,subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,
  checkout_idempotency_digest,checkout_request_digest,payment_due_at,paid_at)
SELECT 'order-arb-'||i,'customer-a','quote-arb-'||i,'CNY','portal',
       CASE WHEN i<=4 THEN 'partially_refunded' ELSE 'paid' END,'approved',1,1000,100,900,0,900,
       md5('oi'||i)||md5('oii'||i),md5('or'||i)||md5('orr'||i),
       CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP
FROM generate_series(1,5) i;
INSERT INTO plugin_idc_core_marketing_campaign_reservation
 (reservation_no,campaign_no,benefit_no,customer_no,quote_reference,request_snapshot_digest,
  currency_code,reserved_units,quoted_value_minor,benefit_snapshot_digest,status_code,expires_at,consumed_at)
SELECT 'res-arb-c-'||i,'campaign-coupon','benefit-coupon','customer-a','quote-arb-'||i,
       md5('rc'||i)||md5('rcc'||i),'CNY',1,50,repeat('a',64),'consumed',CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP
FROM generate_series(1,5) i;
INSERT INTO plugin_idc_core_marketing_campaign_reservation
 (reservation_no,campaign_no,benefit_no,customer_no,quote_reference,request_snapshot_digest,
  currency_code,reserved_units,quoted_value_minor,benefit_snapshot_digest,status_code,expires_at,consumed_at)
SELECT 'res-arb-q-'||i,'campaign-quote','benefit-quote','customer-a','quote-arb-'||i,
       md5('rq'||i)||md5('rqq'||i),'CNY',1,50,repeat('b',64),'consumed',CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP
FROM generate_series(1,5) i;`); err != nil {
		t.Fatalf("seed refund-arbitrage orders: %v", err)
	}
	if _, err = service.Run(ctx, 100); err != nil {
		t.Fatalf("run effect and refund-arbitrage reconciliation: %v", err)
	}
	count, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_abuse_case
WHERE customer_no='customer-a' AND signal_code='refund_arbitrage'`)
	if err != nil || count.Int() != 2 {
		t.Fatalf("refund arbitrage cases=%d err=%v", count.Int(), err)
	}
	if _, err = service.Run(ctx, 100); err != nil {
		t.Fatalf("rerun refund-arbitrage reconciliation: %v", err)
	}
	count, err = database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_marketing_abuse_case
WHERE customer_no='customer-a' AND signal_code='refund_arbitrage'`)
	if err != nil || count.Int() != 2 {
		t.Fatalf("refund arbitrage rerun cases=%d err=%v", count.Int(), err)
	}
}

func TestPromotionReservationPostgreSQLConvergesQuotaAndLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, database)
	service, err := newService(
		database,
		Config{ReservationTTL: 5 * time.Minute},
		bytes.NewReader(deterministicReservationEntropy()),
	)
	if err != nil {
		t.Fatalf("create promotion service: %v", err)
	}
	inputs := []EvaluateInput{
		promotionIntegrationInput("customer-a", "quote-a-0001"),
		promotionIntegrationInput("customer-b", "quote-b-0001"),
	}
	results := make([]*Reservation, len(inputs))
	errors := make([]error, len(inputs))
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.Reserve(ctx, inputs[index])
		}(index)
	}
	close(start)
	wait.Wait()
	winner := -1
	loser := -1
	for index := range inputs {
		switch {
		case errors[index] == nil:
			if winner != -1 {
				t.Fatalf("global quota allowed two winners: results=%+v", results)
			}
			winner = index
		case bizerr.Is(errors[index], CodeQuotaUnavailable):
			loser = index
		default:
			t.Fatalf("unexpected concurrent reservation error[%d]=%v", index, errors[index])
		}
	}
	if winner == -1 || loser == -1 || len(results[winner].Applied) != 1 {
		t.Fatalf("unexpected concurrent reservation outcome: winner=%d loser=%d results=%+v errors=%+v", winner, loser, results, errors)
	}
	reused, err := service.Reserve(ctx, inputs[winner])
	if err != nil || !reused.Reused || reused.Status != ReservationStatusActive {
		t.Fatalf("reuse immutable reservation: result=%+v err=%v", reused, err)
	}
	changed := inputs[winner]
	changed.SubtotalMinor++
	if _, err = service.Reserve(ctx, changed); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed quote replay should conflict, got %v", err)
	}
	released, err := service.Release(ctx, inputs[winner].CustomerNo, inputs[winner].QuoteReference)
	if err != nil || released.Status != ReservationStatusReleased {
		t.Fatalf("release winning reservation: result=%+v err=%v", released, err)
	}
	second, err := service.Reserve(ctx, inputs[loser])
	if err != nil || second.Status != ReservationStatusActive {
		t.Fatalf("reserve released quota for loser: result=%+v err=%v", second, err)
	}
	consumed, err := service.Consume(ctx, inputs[loser].CustomerNo, inputs[loser].QuoteReference)
	if err != nil || consumed.Status != ReservationStatusConsumed {
		t.Fatalf("consume promotion reservation: result=%+v err=%v", consumed, err)
	}
	consumedAgain, err := service.Consume(ctx, inputs[loser].CustomerNo, inputs[loser].QuoteReference)
	if err != nil || !consumedAgain.Reused || consumedAgain.Status != ReservationStatusConsumed {
		t.Fatalf("idempotently consume promotion reservation: result=%+v err=%v", consumedAgain, err)
	}
	var counters struct {
		Reserved int64 `orm:"global_reserved_count"`
		Consumed int64 `orm:"global_consumed_count"`
	}
	if err = database.GetScan(ctx, &counters, `
SELECT global_reserved_count, global_consumed_count
FROM plugin_idc_core_marketing_campaign WHERE campaign_no = 'campaign-quota'`); err != nil ||
		counters.Reserved != 0 || counters.Consumed != 1 {
		t.Fatalf("unexpected terminal campaign counters: %+v err=%v", counters, err)
	}
	verifyQuoteWidePromotionReservation(t, ctx, database, service)
	verifyCouponAssetReservationLifecycle(t, ctx, database, service)
	verifyPointsAwardReversalAndDebtLifecycle(t, ctx, database)
	verifyValueGrantLifecycle(t, ctx, database)
	verifyMarketingEngagementLifecycle(t, ctx, database)
	verifyMarketingManagementOverview(t, ctx, database)
	verifyMarketingManagementDirectory(t, ctx, database)
	verifyMarketingAssetDirectory(t, ctx, database)
	verifyMarketingManagementActions(t, ctx, database)
}

func verifyQuoteWidePromotionReservation(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	service *serviceImpl,
) {
	t.Helper()
	input := QuoteReserveInput{
		CustomerNo: "customer-a", QuoteReference: "quote-wide-0001", CurrencyCode: "CNY",
		PurchasePhase: PurchasePhaseNewPurchase, CycleNumber: 1,
		Lines: []QuoteLineInput{
			{ReferenceNo: "quote-line-1", ProductNo: "product-quote", CategoryNo: "category-1", SubtotalMinor: 600},
			{ReferenceNo: "quote-line-2", ProductNo: "product-other", CategoryNo: "category-2", SubtotalMinor: 400},
		},
	}
	reserved, err := service.ReserveQuote(ctx, input)
	if err != nil || reserved == nil || reserved.ExpiresAt == nil || len(reserved.Applied) != 1 ||
		reserved.Applied[0].CampaignNo != "campaign-quote" ||
		reserved.Applied[0].DiscountAmountMinor != 100 ||
		reserved.Applied[0].ReservationReference == "" || len(reserved.Applied[0].Allocations) != 1 ||
		reserved.Applied[0].Allocations[0] != (QuoteBenefitAllocation{LineReference: "quote-line-1", AmountMinor: 100}) {
		t.Fatalf("reserve quote-wide promotion result=%+v err=%v", reserved, err)
	}
	replayed, err := service.ReserveQuote(ctx, input)
	if err != nil || replayed.Applied[0].ReservationReference != reserved.Applied[0].ReservationReference {
		t.Fatalf("replay quote-wide promotion result=%+v err=%v", replayed, err)
	}
	changed := input
	changed.Lines = append([]QuoteLineInput(nil), input.Lines...)
	changed.Lines[0].SubtotalMinor++
	if _, err = service.ReserveQuote(ctx, changed); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed quote-wide replay returned %v", err)
	}
	count, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation
WHERE quote_reference = 'quote-wide-0001' AND quoted_value_minor = 100`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("quote-wide campaign reservation count=%d err=%v", count.Int(), err)
	}
	if released, releaseErr := service.Release(ctx, input.CustomerNo, input.QuoteReference); releaseErr != nil ||
		released.Status != ReservationStatusReleased {
		t.Fatalf("release quote-wide promotion result=%+v err=%v", released, releaseErr)
	}
}

type marketingActionAuditIntegrationStub struct {
	transitions  []idccap.BusinessAuditTransition
	err          error
	fingerprints map[string][]byte
}

func (s *marketingActionAuditIntegrationStub) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	if input.BeforeSnapshot == nil || input.AfterSnapshot == nil {
		return errors.New("missing audit snapshots")
	}
	switch input.ResultCode {
	case "succeeded", "denied", "failed", "result_unknown":
	default:
		return errors.New("invalid audit result")
	}

	s.transitions = append(s.transitions, input)
	fingerprint, marshalErr := json.Marshal(input)
	if marshalErr != nil {
		return marshalErr
	}
	if s.fingerprints == nil {
		s.fingerprints = map[string][]byte{}
	}
	key := input.IdempotencyScope + "\x00" + input.IdempotencyKey
	if existing, ok := s.fingerprints[key]; ok && !bytes.Equal(existing, fingerprint) {
		return context.DeadlineExceeded
	}
	s.fingerprints[key] = fingerprint
	return s.err
}

func (*marketingActionAuditIntegrationStub) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func verifyMarketingManagementActions(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	authorizer := &managementActionAuthorizerStub{}
	auditWriter := &marketingActionAuditIntegrationStub{}
	points, err := NewPointsService(database)
	if err != nil {
		t.Fatalf("create managed points service: %v", err)
	}
	valueGrants, err := NewValueGrantService(database)
	if err != nil {
		t.Fatalf("create managed value-grant service: %v", err)
	}
	financial, err := billing.NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatalf("create managed financial adjustment service: %v", err)
	}
	service, err := NewManagementActionService(
		database, auditWriter, authorizer, points, valueGrants, financial,
	)
	if err != nil {
		t.Fatalf("create marketing management action service: %v", err)
	}
	issueInput := AssetIssueInput{
		RequestNo: "managed-asset-issue-0001", CampaignNo: "campaign-coupon",
		BenefitNo: "benefit-coupon", CustomerNo: "customer-b", ReasonCode: "customer_service",
	}
	issued, err := service.IssueAsset(ctx, issueInput)
	if err != nil || issued == nil || issued.AssetNo == "" || issued.Status != "active" || issued.Reused {
		t.Fatalf("issue managed asset: result=%+v err=%v", issued, err)
	}
	reusedIssue, err := service.IssueAsset(ctx, issueInput)
	if err != nil || reusedIssue == nil || !reusedIssue.Reused || reusedIssue.AssetNo != issued.AssetNo {
		t.Fatalf("replay managed asset issue: result=%+v err=%v", reusedIssue, err)
	}
	changedIssue := issueInput
	changedIssue.CustomerNo = "customer-a"
	if _, err = service.IssueAsset(ctx, changedIssue); !bizerr.Is(err, CodeManagementActionConflict) {
		t.Fatalf("changed managed asset replay must conflict, got %v", err)
	}
	expectedUpdatedAt := time.UnixMilli(issued.UpdatedAt.UnixMilli()).UTC()
	revoked, err := service.RevokeAsset(ctx, AssetRevokeInput{
		RequestNo: "managed-asset-revoke-0001", AssetNo: issued.AssetNo,
		ExpectedUpdatedAt: expectedUpdatedAt, ReasonCode: "risk_control",
	})
	if err != nil || revoked == nil || revoked.Status != "revoked" || revoked.Reused {
		t.Fatalf("revoke managed asset: result=%+v err=%v", revoked, err)
	}
	reusedRevoke, err := service.RevokeAsset(ctx, AssetRevokeInput{
		RequestNo: "managed-asset-revoke-0001", AssetNo: issued.AssetNo,
		ExpectedUpdatedAt: expectedUpdatedAt, ReasonCode: "risk_control",
	})
	if err != nil || reusedRevoke == nil || !reusedRevoke.Reused || reusedRevoke.Status != "revoked" {
		t.Fatalf("replay managed asset revoke: result=%+v err=%v", reusedRevoke, err)
	}
	creditInput := PointsCreditInput{
		RequestNo: "managed-points-credit-0001", CustomerNo: "customer-b",
		Points: 100, ReasonCode: "manual_correction",
	}
	credited, err := service.CreditPoints(ctx, creditInput)
	if err != nil || credited == nil || credited.LotNo == "" || credited.Reused {
		t.Fatalf("credit managed points: result=%+v err=%v", credited, err)
	}
	reusedCredit, err := service.CreditPoints(ctx, creditInput)
	if err != nil || reusedCredit == nil || !reusedCredit.Reused || reusedCredit.LotNo != credited.LotNo {
		t.Fatalf("replay managed points credit: result=%+v err=%v", reusedCredit, err)
	}
	reverseInput := PointsReverseManagementInput{
		RequestNo: "managed-points-reverse-0001", CustomerNo: "customer-b",
		AwardSourceType: "manual", AwardSourceReference: creditInput.RequestNo,
		Points: 40, ReasonCode: "refund_correction",
	}
	reversed, err := service.ReversePointsAward(ctx, reverseInput)
	if err != nil || reversed == nil || reversed.LotNo != credited.LotNo || reversed.Reused {
		t.Fatalf("reverse managed points: result=%+v err=%v", reversed, err)
	}
	reusedReverse, err := service.ReversePointsAward(ctx, reverseInput)
	if err != nil || reusedReverse == nil || !reusedReverse.Reused || reusedReverse.LotNo != credited.LotNo {
		t.Fatalf("replay managed points reverse: result=%+v err=%v", reusedReverse, err)
	}
	grant, err := valueGrants.CreatePending(ctx, ValueGrantCreateInput{
		GrantType: ValueGrantTypeCashback, CampaignNo: "campaign-cashback",
		BenefitNo: "benefit-cashback", CustomerNo: "customer-b",
		CurrencyCode: "CNY", SourceReference: "managed-grant-financial-0001",
		AmountMinor: 1000,
	})
	if err != nil || grant == nil || grant.Status != "pending" {
		t.Fatalf("create managed value grant: result=%+v err=%v", grant, err)
	}
	postInput := ValueGrantFinancialRequestInput{
		RequestNo: "managed-grant-post-0001", GrantNo: grant.GrantNo,
		ReasonCode: "campaign_fulfillment",
	}
	postRequest, err := service.RequestValueGrantPosting(ctx, postInput)
	if err != nil || postRequest == nil || postRequest.Status != string(billing.AdjustmentStatusPosted) ||
		postRequest.AmountMinor != 1000 || postRequest.LedgerTransactionNo == "" {
		t.Fatalf("request managed value-grant posting: result=%+v err=%v", postRequest, err)
	}
	postReplay, err := service.RequestValueGrantPosting(ctx, postInput)
	if err != nil || postReplay == nil || !postReplay.Reused || postReplay.AdjustmentNo != postRequest.AdjustmentNo {
		t.Fatalf("replay managed value-grant posting request: result=%+v err=%v", postReplay, err)
	}
	reversalInput := ValueGrantFinancialRequestInput{
		RequestNo: "managed-grant-reverse-0001", GrantNo: grant.GrantNo,
		AmountMinor: 250, ReasonCode: "refund_correction",
	}
	reversalRequest, err := service.RequestValueGrantReversal(ctx, reversalInput)
	if err != nil || reversalRequest == nil || reversalRequest.Status != string(billing.AdjustmentStatusPosted) ||
		reversalRequest.AmountMinor != 250 {
		t.Fatalf("request managed value-grant reversal: result=%+v err=%v", reversalRequest, err)
	}
	reversalReplay, err := service.RequestValueGrantReversal(ctx, reversalInput)
	if err != nil || reversalReplay == nil || !reversalReplay.Reused ||
		reversalReplay.AdjustmentNo != reversalRequest.AdjustmentNo {
		t.Fatalf("replay managed value-grant reversal request: result=%+v err=%v", reversalReplay, err)
	}
	storedGrant, err := loadValueGrant(ctx, database, grant.GrantNo, "customer-b", false)
	if err != nil || storedGrant.AvailableAmountMinor != 750 || storedGrant.ReversedAmountMinor != 250 {
		t.Fatalf("read financially reviewed value grant: result=%+v err=%v", storedGrant, err)
	}
	var financialState struct {
		Transactions int   `orm:"transactions"`
		Entries      int   `orm:"entries"`
		DebitTotal   int64 `orm:"debit_total"`
		CreditTotal  int64 `orm:"credit_total"`
	}
	if err = database.GetScan(ctx, &financialState, `
SELECT count(DISTINCT transaction.transaction_no) AS transactions,
       count(entry.entry_no) AS entries,
       coalesce(sum(CASE WHEN entry.direction_code = 'debit' THEN entry.amount_minor ELSE 0 END), 0) AS debit_total,
       coalesce(sum(CASE WHEN entry.direction_code = 'credit' THEN entry.amount_minor ELSE 0 END), 0) AS credit_total
FROM plugin_idc_core_billing_ledger_transaction transaction
JOIN plugin_idc_core_billing_ledger_entry entry
  ON entry.transaction_no = transaction.transaction_no
WHERE transaction.source_reference IN (?, ?)`, postRequest.AdjustmentNo, reversalRequest.AdjustmentNo); err != nil ||
		financialState.Transactions != 2 || financialState.Entries != 4 ||
		financialState.DebitTotal != 1250 || financialState.CreditTotal != 1250 {
		t.Fatalf("unexpected managed value-grant ledger state: %+v err=%v", financialState, err)
	}
	if len(auditWriter.transitions) != 8 {
		t.Fatalf("each committed mutation must retain its audit, got %d transitions", len(auditWriter.transitions))
	}
	wantPermissions := []string{
		permissionMarketingAssetIssue, permissionMarketingAssetIssue,
		permissionMarketingAssetRevoke, permissionMarketingAssetRevoke,
		permissionMarketingPointsCredit, permissionMarketingPointsReverse,
		permissionMarketingGrantPost, permissionMarketingGrantReverse,
	}
	for index, transition := range auditWriter.transitions {
		if transition.RequiredPermission != wantPermissions[index] {
			t.Fatalf("audit transition %d permission=%q want=%q", index, transition.RequiredPermission, wantPermissions[index])
		}
	}

	failingAudit := &marketingActionAuditIntegrationStub{err: context.Canceled}
	failingService, err := NewManagementActionService(
		database, failingAudit, authorizer, points, valueGrants, financial,
	)
	if err != nil {
		t.Fatalf("create audit-failing management service: %v", err)
	}
	rollbackReference := "managed-points-credit-rollback"
	if _, err = failingService.CreditPoints(ctx, PointsCreditInput{
		RequestNo: rollbackReference, CustomerNo: "customer-b", Points: 25,
		ReasonCode: "manual_correction",
	}); !bizerr.Is(err, CodeManagementActionUnavailable) {
		t.Fatalf("audit failure must fail the management action, got %v", err)
	}
	count, err := database.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_core_marketing_points_lot
WHERE customer_no = 'customer-b' AND source_type_code = 'manual' AND source_reference = ?`, rollbackReference)
	if err != nil || count.Int() != 0 {
		t.Fatalf("audit failure must roll back nested points mutation: count=%d err=%v", count.Int(), err)
	}
}

type marketingOverviewAuthorizerStub struct {
	calls int
	err   error
}

func (s *marketingOverviewAuthorizerStub) AuthorizeMarketingOverview(context.Context) (int, error) {
	s.calls++
	return 7, s.err
}

func verifyMarketingManagementOverview(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	authorizer := &marketingOverviewAuthorizerStub{}
	reader, err := NewManagementOverviewReader(database, authorizer)
	if err != nil {
		t.Fatalf("create marketing overview reader: %v", err)
	}
	overview, err := reader.GetOverview(ctx)
	if err != nil {
		t.Fatalf("read marketing overview: %v", err)
	}
	if authorizer.calls != 1 || overview == nil || overview.GeneratedAt.IsZero() ||
		overview.Summary.SegmentTotal < 1 || overview.Summary.CampaignTotal < 1 ||
		overview.Summary.AssetTotal < 1 || overview.Summary.PointsAccountCount < 1 ||
		overview.Summary.ConsentOptedOut < 1 || len(overview.RecentCampaigns) == 0 ||
		len(overview.RecentGrants) == 0 {
		t.Fatalf("unexpected marketing overview: %+v authorizer=%+v", overview, authorizer)
	}
	foundCNY := false
	for _, item := range overview.Currencies {
		if item.CurrencyCode == "CNY" && item.GrantCount > 0 {
			foundCNY = true
		}
	}
	if !foundCNY {
		t.Fatalf("marketing overview must keep CNY grants in an explicit currency row: %+v", overview.Currencies)
	}
}

func verifyMarketingManagementDirectory(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	authorizer := &marketingDirectoryAuthorizerStub{}
	reader, err := NewManagementDirectoryReader(database, authorizer, bytes.Repeat([]byte{0x66}, 32))
	if err != nil {
		t.Fatalf("create marketing directory reader: %v", err)
	}
	segments, err := reader.SearchSegments(ctx, SegmentManagementSearchInput{NamePrefix: "engagement", Status: "active", Limit: 1})
	if err != nil || len(segments.Segments) != 1 || segments.Segments[0].SegmentNo != "segment-engagement" || segments.Segments[0].LatestSnapshotMembers != 1 {
		t.Fatalf("read marketing segment page: page=%+v err=%v", segments, err)
	}
	segment, err := reader.GetSegment(ctx, "segment-engagement")
	if err != nil || segment == nil || len(segment.Snapshots) != 1 || segment.Snapshots[0].SnapshotNo != "snapshot-engagement" || segment.Conditions == nil {
		t.Fatalf("read marketing segment detail: detail=%+v err=%v", segment, err)
	}
	campaigns, err := reader.SearchCampaigns(ctx, CampaignManagementSearchInput{Type: "discount", Status: "active", Limit: 1})
	if err != nil || len(campaigns.Campaigns) != 1 || campaigns.NextCursor == "" {
		t.Fatalf("read first marketing campaign page: page=%+v err=%v", campaigns, err)
	}
	second, err := reader.SearchCampaigns(ctx, CampaignManagementSearchInput{Type: "discount", Status: "active", Limit: 1, Cursor: campaigns.NextCursor})
	if err != nil || len(second.Campaigns) != 1 || second.Campaigns[0].CampaignNo == campaigns.Campaigns[0].CampaignNo {
		t.Fatalf("read second marketing campaign page: first=%+v second=%+v err=%v", campaigns, second, err)
	}
	campaign, err := reader.GetCampaign(ctx, "campaign-quota")
	if err != nil || campaign == nil || len(campaign.Scopes) != 1 || len(campaign.Benefits) != 1 || campaign.Benefits[0].CurrencyCode != "CNY" {
		t.Fatalf("read marketing campaign detail: detail=%+v err=%v", campaign, err)
	}
	if authorizer.segmentListCalls != 1 || authorizer.segmentQueryCalls != 1 || authorizer.campaignListCalls != 2 || authorizer.campaignQueryCalls != 1 {
		t.Fatalf("unexpected directory authorization calls: %+v", authorizer)
	}
}

func verifyMarketingAssetDirectory(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	authorizer := &assetDirectoryAuthorizerStub{}
	reader, err := NewAssetDirectoryReader(database, authorizer, bytes.Repeat([]byte{0x67}, 32))
	if err != nil {
		t.Fatalf("create marketing asset directory reader: %v", err)
	}
	assets, err := reader.SearchAssets(ctx, PromotionAssetSearchInput{CustomerNo: "customer-a", Currency: "CNY", Limit: 25})
	if err != nil || len(assets.Assets) == 0 {
		t.Fatalf("read marketing asset page: page=%+v err=%v", assets, err)
	}
	asset, err := reader.GetAsset(ctx, "asset-coupon-a")
	if err != nil || asset == nil || asset.CurrencyCode != "CNY" || len(asset.Reservations) == 0 {
		t.Fatalf("read marketing promotion asset detail: detail=%+v err=%v", asset, err)
	}
	accounts, err := reader.SearchPointsAccounts(ctx, PointsAccountSearchInput{CustomerNo: "customer-a", Limit: 25})
	if err != nil || len(accounts.Accounts) != 1 || accounts.Accounts[0].CustomerNo != "customer-a" {
		t.Fatalf("read marketing points account page: page=%+v err=%v", accounts, err)
	}
	account, err := reader.GetPointsAccount(ctx, "customer-a")
	if err != nil || account == nil || len(account.Lots) == 0 || len(account.Entries) == 0 {
		t.Fatalf("read marketing points account detail: detail=%+v err=%v", account, err)
	}
	grants, err := reader.SearchValueGrants(ctx, ValueGrantSearchInput{CustomerNo: "customer-a", Currency: "CNY", Limit: 25})
	if err != nil || len(grants.Grants) == 0 {
		t.Fatalf("read marketing value grant page: page=%+v err=%v", grants, err)
	}
	grant, err := reader.GetValueGrant(ctx, grants.Grants[0].GrantNo)
	if err != nil || grant == nil || grant.CurrencyCode != "CNY" || grant.Entries == nil {
		t.Fatalf("read marketing value grant detail: detail=%+v err=%v", grant, err)
	}
	if authorizer.assetListCalls != 1 || authorizer.assetQueryCalls != 1 || authorizer.pointsListCalls != 1 ||
		authorizer.pointsQueryCalls != 1 || authorizer.grantListCalls != 1 || authorizer.grantQueryCalls != 1 {
		t.Fatalf("unexpected asset directory authorization calls: %+v", authorizer)
	}
}

func verifyValueGrantLifecycle(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	service, err := newValueGrantService(database, rand.Reader)
	if err != nil {
		t.Fatalf("create value grant service: %v", err)
	}
	invalid := ValueGrantCreateInput{
		GrantType: ValueGrantTypeRechargeBonus, CampaignNo: "campaign-recharge-bonus",
		BenefitNo:  "benefit-recharge-bonus",
		CustomerNo: "customer-a", CurrencyCode: "CNY", SourceReference: "recharge-invalid",
		AmountMinor: 5000, Withdrawable: true,
	}
	if _, err = service.CreatePending(ctx, invalid); !bizerr.Is(err, CodeValueGrantInvalid) {
		t.Fatalf("withdrawable recharge bonus should be invalid, got %v", err)
	}
	create := invalid
	create.SourceReference = "recharge-paid-0001"
	create.Withdrawable = false
	pending, err := service.CreatePending(ctx, create)
	if err != nil || pending.Status != "pending" || pending.PendingAmountMinor != 5000 || pending.Withdrawable {
		t.Fatalf("create pending recharge bonus: result=%+v err=%v", pending, err)
	}
	pendingReplay, err := service.CreatePending(ctx, create)
	if err != nil || !pendingReplay.Reused || pendingReplay.GrantNo != pending.GrantNo {
		t.Fatalf("replay pending recharge bonus: result=%+v err=%v", pendingReplay, err)
	}
	changedCreate := create
	changedCreate.AmountMinor++
	if _, err = service.CreatePending(ctx, changedCreate); !bizerr.Is(err, CodeValueGrantConflict) {
		t.Fatalf("changed value grant replay should conflict, got %v", err)
	}
	post := ValueGrantPostInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a",
		FinancialPostingReference: "ledger-gift-posting-0001",
	}
	posted, err := service.Post(ctx, post)
	if err != nil || posted.Status != "available" || posted.PendingAmountMinor != 0 ||
		posted.AvailableAmountMinor != 5000 || posted.FinancialPostingReference != post.FinancialPostingReference {
		t.Fatalf("post recharge bonus: result=%+v err=%v", posted, err)
	}
	postReplay, err := service.Post(ctx, post)
	if err != nil || !postReplay.Reused {
		t.Fatalf("replay recharge bonus posting: result=%+v err=%v", postReplay, err)
	}
	changedPost := post
	changedPost.FinancialPostingReference = "ledger-gift-posting-changed"
	if _, err = service.Post(ctx, changedPost); !bizerr.Is(err, CodeValueGrantConflict) {
		t.Fatalf("changed financial posting should conflict, got %v", err)
	}
	mutation := ValueGrantMutationInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a", SourceType: "order",
		SourceReference: "order-gift-freeze-0001", AmountMinor: 3000,
	}
	frozen, err := service.Freeze(ctx, mutation)
	if err != nil || frozen.AvailableAmountMinor != 2000 || frozen.FrozenAmountMinor != 3000 {
		t.Fatalf("freeze recharge bonus: result=%+v err=%v", frozen, err)
	}
	frozenReplay, err := service.Freeze(ctx, mutation)
	if err != nil || !frozenReplay.Reused {
		t.Fatalf("replay recharge bonus freeze: result=%+v err=%v", frozenReplay, err)
	}
	consume := mutation
	consume.SourceReference = "order-gift-consume-0001"
	consume.AmountMinor = 1000
	consumed, err := service.Consume(ctx, consume)
	if err != nil || consumed.FrozenAmountMinor != 2000 || consumed.ConsumedAmountMinor != 1000 {
		t.Fatalf("consume recharge bonus: result=%+v err=%v", consumed, err)
	}
	release := mutation
	release.SourceReference = "order-gift-release-0001"
	release.AmountMinor = 500
	released, err := service.Release(ctx, release)
	if err != nil || released.AvailableAmountMinor != 2500 || released.FrozenAmountMinor != 1500 {
		t.Fatalf("release recharge bonus: result=%+v err=%v", released, err)
	}
	reverse := mutation
	reverse.SourceType = "refund"
	reverse.SourceReference = "refund-gift-0001"
	reverse.AmountMinor = 4000
	reversed, err := service.Reverse(ctx, reverse)
	if err != nil || reversed.Status != "exhausted" || reversed.AvailableAmountMinor != 0 ||
		reversed.FrozenAmountMinor != 0 || reversed.ConsumedAmountMinor != 1000 ||
		reversed.ReversedAmountMinor != 4000 {
		t.Fatalf("reverse unconsumed recharge bonus: result=%+v err=%v", reversed, err)
	}
	consumedReverse := reverse
	consumedReverse.SourceReference = "refund-gift-0002"
	consumedReverse.AmountMinor = 500
	reversedConsumed, err := service.Reverse(ctx, consumedReverse)
	if err != nil || reversedConsumed.ConsumedAmountMinor != 500 ||
		reversedConsumed.ReversedAmountMinor != 4500 {
		t.Fatalf("reverse consumed recharge bonus: result=%+v err=%v", reversedConsumed, err)
	}
	tooMuch := consumedReverse
	tooMuch.SourceReference = "refund-gift-too-much"
	tooMuch.AmountMinor = 501
	if _, err = service.Reverse(ctx, tooMuch); !bizerr.Is(err, CodeValueGrantConflict) {
		t.Fatalf("over-reversal should conflict, got %v", err)
	}
	finalReverse := consumedReverse
	finalReverse.SourceReference = "refund-gift-0003"
	fullyReversed, err := service.Reverse(ctx, finalReverse)
	if err != nil || fullyReversed.Status != "reversed" ||
		fullyReversed.ReversedAmountMinor != fullyReversed.OriginalAmountMinor {
		t.Fatalf("fully reverse recharge bonus: result=%+v err=%v", fullyReversed, err)
	}
	verifyValueGrantCancellation(t, ctx, service)
	verifyValueGrantExpiry(t, ctx, database, service)
	verifyConcurrentValueGrantFreeze(t, ctx, service)
	verifyConcurrentValueGrantCreation(t, ctx, database, service)
	verifyConcurrentValueGrantReplay(t, ctx, service)
	var entries struct {
		PostCount    int   `orm:"post_count"`
		ReverseCount int   `orm:"reverse_count"`
		DeltaSum     int64 `orm:"delta_sum"`
	}
	if err = database.GetScan(ctx, &entries, `
SELECT count(*) FILTER (WHERE entry_type_code = 'post') AS post_count,
       count(*) FILTER (WHERE entry_type_code = 'reverse') AS reverse_count,
       coalesce(sum(pending_delta_minor + available_delta_minor + frozen_delta_minor +
           consumed_delta_minor + expired_delta_minor + reversed_delta_minor), 0) AS delta_sum
FROM plugin_idc_core_marketing_value_entry
WHERE grant_no = ?`, pending.GrantNo); err != nil ||
		entries.PostCount != 1 || entries.ReverseCount != 3 || entries.DeltaSum != 0 {
		t.Fatalf("unexpected append-only value grant evidence: %+v err=%v", entries, err)
	}
}

func verifyMarketingEngagementLifecycle(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	service, err := newEngagementService(database, rand.Reader)
	if err != nil {
		t.Fatalf("create marketing engagement service: %v", err)
	}
	gate := MarketingSendGateInput{
		CampaignNo: "campaign-engagement", SnapshotNo: "snapshot-engagement",
		CustomerNo: "customer-a", ContactNo: "contact-a", Channel: "email",
		ContentDigest: strings.Repeat("1", 64),
	}
	decisions, err := service.CheckSendBatch(ctx, []MarketingSendGateInput{gate})
	if err != nil || len(decisions) != 1 || decisions[0].ReasonCode != MarketingGateConsentMissing {
		t.Fatalf("missing consent should deny marketing: decisions=%+v err=%v", decisions, err)
	}
	consent := MarketingConsentInput{
		CustomerNo: "customer-a", ContactNo: "contact-a", Channel: "email",
		Action: MarketingConsentGranted, PolicyReference: "idc://marketing/email",
		PolicyVersion: "1", Locale: "zh-CN", Source: "portal",
		EvidenceDigest: strings.Repeat("2", 64), IdempotencyKey: "consent-email-0001",
	}
	granted, err := service.SetConsent(ctx, consent)
	if err != nil || granted.Status != "opted_in" || granted.Version != 1 || granted.Reused {
		t.Fatalf("grant marketing consent: result=%+v err=%v", granted, err)
	}
	replayed, err := service.SetConsent(ctx, consent)
	if err != nil || !replayed.Reused || replayed.EventNo != granted.EventNo {
		t.Fatalf("replay marketing consent: result=%+v err=%v", replayed, err)
	}
	changed := consent
	changed.PolicyVersion = "2"
	if _, err = service.SetConsent(ctx, changed); !bizerr.Is(err, CodeEngagementConflict) {
		t.Fatalf("changed consent replay should conflict, got %v", err)
	}
	decisions, err = service.CheckSendBatch(ctx, []MarketingSendGateInput{gate})
	if err != nil || !decisions[0].Allowed || decisions[0].ConsentEventNo != granted.EventNo ||
		!lowerHexDigest(decisions[0].DecisionDigest) {
		t.Fatalf("opted-in marketing gate: decisions=%+v err=%v", decisions, err)
	}
	verifyConcurrentMarketingConsent(t, ctx, service)
	databaseTime, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		t.Fatalf("read quiet-hours fixture time: %v", err)
	}
	now := databaseTime.Time().UTC()
	minute := now.Hour()*60 + now.Minute()
	quiet, err := service.SetQuietHours(ctx, MarketingQuietHoursInput{
		CustomerNo: "customer-a", TimezoneName: "UTC",
		StartMinute: (minute + 1439) % 1440, EndMinute: (minute + 2) % 1440,
		Enabled: true,
	})
	if err != nil || quiet.Version != 1 {
		t.Fatalf("create marketing quiet hours: result=%+v err=%v", quiet, err)
	}
	decisions, err = service.CheckSendBatch(ctx, []MarketingSendGateInput{gate})
	if err != nil || decisions[0].ReasonCode != MarketingGateQuietHours {
		t.Fatalf("quiet hours should deny marketing: decisions=%+v err=%v", decisions, err)
	}
	quiet, err = service.SetQuietHours(ctx, MarketingQuietHoursInput{
		CustomerNo: "customer-a", TimezoneName: "Asia/Shanghai",
		StartMinute: 0, EndMinute: 0, Enabled: false, ExpectedVersion: quiet.Version,
	})
	if err != nil || quiet.Version != 2 || quiet.Enabled {
		t.Fatalf("disable marketing quiet hours: result=%+v err=%v", quiet, err)
	}
	if _, err = service.SetQuietHours(ctx, MarketingQuietHoursInput{
		CustomerNo: "customer-a", TimezoneName: "UTC", StartMinute: 1,
		EndMinute: 2, Enabled: true, ExpectedVersion: 1,
	}); !bizerr.Is(err, CodeEngagementConflict) {
		t.Fatalf("stale quiet-hours update should conflict, got %v", err)
	}
	batch := []MarketingSendGateInput{gate, {
		CampaignNo: "campaign-engagement", SnapshotNo: "snapshot-engagement",
		CustomerNo: "customer-b", ContactNo: "contact-b", Channel: "email",
		ContentDigest: strings.Repeat("1", 64),
	}}
	decisions, err = service.CheckSendBatch(ctx, batch)
	if err != nil || len(decisions) != 2 || !decisions[0].Allowed ||
		decisions[1].ReasonCode != MarketingGateOutsideAudience {
		t.Fatalf("batch marketing gate: decisions=%+v err=%v", decisions, err)
	}
	duplicateTarget := gate
	duplicateTarget.ContentDigest = strings.Repeat("4", 64)
	if _, err = service.CheckSendBatch(ctx, []MarketingSendGateInput{gate, duplicateTarget}); !bizerr.Is(err, CodeEngagementInvalid) {
		t.Fatalf("same marketing target with different content should be rejected, got %v", err)
	}
	withdraw := consent
	withdraw.Action = MarketingConsentWithdrawn
	withdraw.Source = "notify"
	withdraw.EvidenceDigest = strings.Repeat("3", 64)
	withdraw.IdempotencyKey = "unsubscribe-email-0001"
	withdrawn, err := service.SetConsent(ctx, withdraw)
	if err != nil || withdrawn.Status != "opted_out" || withdrawn.Version != 2 {
		t.Fatalf("withdraw marketing consent: result=%+v err=%v", withdrawn, err)
	}
	decisions, err = service.CheckSendBatch(ctx, []MarketingSendGateInput{gate})
	if err != nil || decisions[0].ReasonCode != MarketingGateConsentWithdrawn {
		t.Fatalf("withdrawn consent should deny marketing: decisions=%+v err=%v", decisions, err)
	}
	var evidence struct {
		EventCount int  `orm:"event_count"`
		Enabled    bool `orm:"enabled"`
	}
	if err = database.GetScan(ctx, &evidence, `
SELECT count(e.id) AS event_count, bool_and(p.enabled) AS enabled
FROM plugin_idc_core_marketing_channel_consent_event e
JOIN plugin_idc_core_customer_contact_preference p
  ON p.customer_no = e.customer_no AND p.contact_no = e.contact_no
 AND p.channel_code = e.channel_code AND p.topic_code = 'marketing'
WHERE e.customer_no = 'customer-a' AND e.contact_no = 'contact-a'
  AND e.channel_code = 'email'
GROUP BY p.enabled`); err != nil || evidence.EventCount != 2 || evidence.Enabled {
		t.Fatalf("unexpected marketing consent evidence: %+v err=%v", evidence, err)
	}
}

func verifyConcurrentMarketingConsent(
	t *testing.T,
	ctx context.Context,
	service *engagementServiceImpl,
) {
	t.Helper()
	input := MarketingConsentInput{
		CustomerNo: "customer-a", ContactNo: "contact-a", Channel: "sms",
		Action: MarketingConsentGranted, PolicyReference: "idc://marketing/sms",
		PolicyVersion: "1", Locale: "zh-CN", Source: "portal",
		EvidenceDigest: strings.Repeat("4", 64), IdempotencyKey: "consent-sms-concurrent",
	}
	results := make([]*MarketingConsentResult, 2)
	errors := make([]error, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.SetConsent(ctx, input)
		}(index)
	}
	close(start)
	wait.Wait()
	if errors[0] != nil || errors[1] != nil || results[0] == nil || results[1] == nil ||
		results[0].EventNo != results[1].EventNo || results[0].Reused == results[1].Reused {
		t.Fatalf("concurrent consent replay results=%+v errors=%v", results, errors)
	}
}

func verifyConcurrentValueGrantReplay(
	t *testing.T,
	ctx context.Context,
	service *valueGrantServiceImpl,
) {
	t.Helper()
	input := ValueGrantCreateInput{
		GrantType: ValueGrantTypeCashback, CampaignNo: "campaign-cashback",
		BenefitNo: "benefit-cashback", CustomerNo: "customer-b", CurrencyCode: "CNY",
		SourceReference: "cashback-concurrent-replay", AmountMinor: 1000,
	}
	results := make([]*ValueGrantProjection, 2)
	errors := make([]error, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.CreatePending(ctx, input)
		}(index)
	}
	close(start)
	wait.Wait()
	if errors[0] != nil || errors[1] != nil || results[0] == nil || results[1] == nil ||
		results[0].GrantNo != results[1].GrantNo || results[0].Reused == results[1].Reused {
		t.Fatalf("concurrent value grant replay results=%+v errors=%v", results, errors)
	}
	if _, err := service.CancelPending(ctx, ValueGrantMutationInput{
		GrantNo: results[0].GrantNo, CustomerNo: input.CustomerNo, SourceType: "activity",
		SourceReference: "cashback-concurrent-replay-cancel", AmountMinor: input.AmountMinor,
	}); err != nil {
		t.Fatalf("cancel concurrent replay fixture: %v", err)
	}
}

func verifyConcurrentValueGrantCreation(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	service *valueGrantServiceImpl,
) {
	t.Helper()
	inputs := []ValueGrantCreateInput{
		{
			GrantType: ValueGrantTypeCashback, CampaignNo: "campaign-value-quota",
			BenefitNo: "benefit-value-quota", CustomerNo: "customer-a", CurrencyCode: "CNY",
			SourceReference: "value-quota-a", AmountMinor: 700,
		},
		{
			GrantType: ValueGrantTypeCashback, CampaignNo: "campaign-value-quota",
			BenefitNo: "benefit-value-quota", CustomerNo: "customer-b", CurrencyCode: "CNY",
			SourceReference: "value-quota-b", AmountMinor: 700,
		},
	}
	results := make([]*ValueGrantProjection, len(inputs))
	errors := make([]error, len(inputs))
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.CreatePending(ctx, inputs[index])
		}(index)
	}
	close(start)
	wait.Wait()
	winner := -1
	loser := -1
	for index, operationErr := range errors {
		if operationErr == nil {
			winner = index
		} else if bizerr.Is(operationErr, CodeQuotaUnavailable) {
			loser = index
		} else {
			t.Fatalf("unexpected concurrent value grant creation error[%d]=%v", index, operationErr)
		}
	}
	if winner == -1 || loser == -1 {
		t.Fatalf("value grant quota allowed unexpected outcome results=%+v errors=%v", results, errors)
	}
	cancelled, err := service.CancelPending(ctx, ValueGrantMutationInput{
		GrantNo: results[winner].GrantNo, CustomerNo: inputs[winner].CustomerNo,
		SourceType: "activity", SourceReference: "value-quota-release", AmountMinor: 700,
	})
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("release value grant quota: result=%+v err=%v", cancelled, err)
	}
	second, err := service.CreatePending(ctx, inputs[loser])
	if err != nil || second.Status != "pending" {
		t.Fatalf("reuse released value grant quota: result=%+v err=%v", second, err)
	}
	if _, err = service.Post(ctx, ValueGrantPostInput{
		GrantNo: second.GrantNo, CustomerNo: second.CustomerNo,
		FinancialPostingReference: "ledger-value-quota-consume",
	}); err != nil {
		t.Fatalf("consume value grant quota: %v", err)
	}
	var quota struct {
		Reserved int64 `orm:"global_reserved_count"`
		Consumed int64 `orm:"global_consumed_count"`
	}
	if err = database.GetScan(ctx, &quota, `
SELECT global_reserved_count, global_consumed_count
FROM plugin_idc_core_marketing_campaign
WHERE campaign_no = 'campaign-value-quota'`); err != nil || quota.Reserved != 0 || quota.Consumed != 1 {
		t.Fatalf("unexpected value grant quota counters: %+v err=%v", quota, err)
	}
}

func verifyValueGrantCancellation(
	t *testing.T,
	ctx context.Context,
	service *valueGrantServiceImpl,
) {
	t.Helper()
	pending, err := service.CreatePending(ctx, ValueGrantCreateInput{
		GrantType: ValueGrantTypeRechargeBonus, CampaignNo: "campaign-recharge-bonus",
		BenefitNo:  "benefit-recharge-bonus",
		CustomerNo: "customer-a", CurrencyCode: "CNY", SourceReference: "recharge-cancel-0001",
		AmountMinor: 5000,
	})
	if err != nil {
		t.Fatalf("create cancellable value grant: %v", err)
	}
	cancel := ValueGrantMutationInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a", SourceType: "activity",
		SourceReference: "recharge-cancel-event-0001", AmountMinor: 5000,
	}
	cancelled, err := service.CancelPending(ctx, cancel)
	if err != nil || cancelled.Status != "cancelled" || cancelled.ReversedAmountMinor != 5000 ||
		cancelled.FinancialPostingReference != "" {
		t.Fatalf("cancel pending value grant: result=%+v err=%v", cancelled, err)
	}
	replay, err := service.CancelPending(ctx, cancel)
	if err != nil || !replay.Reused {
		t.Fatalf("replay pending cancellation: result=%+v err=%v", replay, err)
	}
	if _, err = service.Post(ctx, ValueGrantPostInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a",
		FinancialPostingReference: "ledger-should-not-post",
	}); !bizerr.Is(err, CodeValueGrantConflict) {
		t.Fatalf("cancelled grant should not post, got %v", err)
	}
}

func verifyValueGrantExpiry(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	service *valueGrantServiceImpl,
) {
	t.Helper()
	expiresAt := time.Now().Add(time.Hour)
	pending, err := service.CreatePending(ctx, ValueGrantCreateInput{
		GrantType: ValueGrantTypeCashback, CampaignNo: "campaign-cashback",
		BenefitNo:  "benefit-cashback",
		CustomerNo: "customer-a", CurrencyCode: "CNY", SourceReference: "cashback-expiry-0001",
		AmountMinor: 1000, Withdrawable: true, ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("create cashback grant: %v", err)
	}
	if _, err = service.Post(ctx, ValueGrantPostInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a",
		FinancialPostingReference: "ledger-cashback-posting-0001",
	}); err != nil {
		t.Fatalf("post cashback grant: %v", err)
	}
	if _, err = service.Freeze(ctx, ValueGrantMutationInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a", SourceType: "order",
		SourceReference: "cashback-freeze-0001", AmountMinor: 400,
	}); err != nil {
		t.Fatalf("freeze cashback before expiry: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_value_grant
SET expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second' WHERE grant_no = ?`, pending.GrantNo); err != nil {
		t.Fatalf("advance cashback expiry fixture: %v", err)
	}
	expired, err := service.Expire(ctx, ValueGrantMutationInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a", SourceType: "expiry",
		SourceReference: "cashback-expire-available-0001", AmountMinor: 600,
	})
	if err != nil || expired.AvailableAmountMinor != 0 || expired.FrozenAmountMinor != 400 ||
		expired.ExpiredAmountMinor != 600 || expired.Status != "available" {
		t.Fatalf("expire available cashback: result=%+v err=%v", expired, err)
	}
	released, err := service.Release(ctx, ValueGrantMutationInput{
		GrantNo: pending.GrantNo, CustomerNo: "customer-a", SourceType: "order",
		SourceReference: "cashback-release-after-expiry-0001", AmountMinor: 400,
	})
	if err != nil || released.Status != "expired" || released.ExpiredAmountMinor != 1000 ||
		released.AvailableAmountMinor != 0 || released.FrozenAmountMinor != 0 {
		t.Fatalf("release frozen cashback after expiry: result=%+v err=%v", released, err)
	}
}

func verifyConcurrentValueGrantFreeze(
	t *testing.T,
	ctx context.Context,
	service *valueGrantServiceImpl,
) {
	t.Helper()
	pending, err := service.CreatePending(ctx, ValueGrantCreateInput{
		GrantType: ValueGrantTypeCashback, CampaignNo: "campaign-cashback",
		BenefitNo:  "benefit-cashback",
		CustomerNo: "customer-b", CurrencyCode: "CNY", SourceReference: "cashback-concurrent",
		AmountMinor: 1000,
	})
	if err == nil {
		_, err = service.Post(ctx, ValueGrantPostInput{
			GrantNo: pending.GrantNo, CustomerNo: "customer-b",
			FinancialPostingReference: "ledger-cashback-concurrent",
		})
	}
	if err != nil {
		t.Fatalf("prepare concurrent cashback: %v", err)
	}
	errors := make([]error, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range errors {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, errors[index] = service.Freeze(ctx, ValueGrantMutationInput{
				GrantNo: pending.GrantNo, CustomerNo: "customer-b", SourceType: "order",
				SourceReference: "cashback-concurrent-" + string(rune('a'+index)), AmountMinor: 700,
			})
		}(index)
	}
	close(start)
	wait.Wait()
	successes := 0
	conflicts := 0
	for _, operationErr := range errors {
		if operationErr == nil {
			successes++
		} else if bizerr.Is(operationErr, CodeValueGrantConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent value grant error: %v", operationErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent value grant outcome successes=%d conflicts=%d errors=%v", successes, conflicts, errors)
	}
}

func verifyPointsAwardReversalAndDebtLifecycle(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
	t.Helper()
	service, err := newPointsService(database, bytes.NewReader(deterministicReservationEntropy()))
	if err != nil {
		t.Fatalf("create points service: %v", err)
	}
	award := PointsAwardInput{
		CustomerNo: "customer-a", SourceType: "order", SourceReference: "order-points-0001", Points: 100,
	}
	awarded, err := service.Award(ctx, award)
	if err != nil || awarded.AvailablePoints != 100 || awarded.DebtPoints != 0 || awarded.AvailableDelta != 100 {
		t.Fatalf("award points: result=%+v err=%v", awarded, err)
	}
	replayed, err := service.Award(ctx, award)
	if err != nil || !replayed.Reused || replayed.LotNo != awarded.LotNo {
		t.Fatalf("replay points award: result=%+v err=%v", replayed, err)
	}
	changed := award
	changed.Points = 101
	if _, err = service.Award(ctx, changed); !bizerr.Is(err, CodePointsConflict) {
		t.Fatalf("changed points award replay should conflict, got %v", err)
	}
	_, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_lot
SET available_points = 20, frozen_points = 10, consumed_points = 70, status_code = 'available'
WHERE lot_no = ?`, awarded.LotNo)
	if err == nil {
		_, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_account
SET available_points = 20, frozen_points = 10 WHERE customer_no = 'customer-a'`)
	}
	if err == nil {
		_, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_points_entry (
    entry_no, customer_no, lot_no, entry_type_code, available_delta, frozen_delta,
    source_type_code, source_reference, entry_line_no
) VALUES
    ('pent_fixture_freeze', 'customer-a', ?, 'freeze', -10, 10,
     'redemption', 'redeem-fixture-1', 1),
    ('pent_fixture_spend', 'customer-a', ?, 'spend', -70, 0,
	 'redemption', 'redeem-fixture-2', 1)`, awarded.LotNo, awarded.LotNo)
	}
	if err != nil {
		t.Fatalf("prepare consumed points fixture: %v", err)
	}
	reversal := PointsReverseInput{
		CustomerNo: "customer-a", AwardSourceType: "order",
		AwardSourceReference: "order-points-0001", ReversalSourceType: "refund",
		ReversalSourceReference: "refund-points-0001", Points: 60,
	}
	reversed, err := service.ReverseAward(ctx, reversal)
	if err != nil || reversed.AvailableDelta != -20 || reversed.FrozenDelta != -10 ||
		reversed.DebtDelta != 30 || reversed.DebtPoints != 30 ||
		reversed.LotReversedPoints != 30 || reversed.ReversalShortfall != 30 {
		t.Fatalf("reverse awarded points: result=%+v err=%v", reversed, err)
	}
	reversedReplay, err := service.ReverseAward(ctx, reversal)
	if err != nil || !reversedReplay.Reused || reversedReplay.DebtDelta != 30 {
		t.Fatalf("replay points reversal: result=%+v err=%v", reversedReplay, err)
	}
	secondReversal := reversal
	secondReversal.ReversalSourceReference = "refund-points-0002"
	secondReversal.Points = 20
	second, err := service.ReverseAward(ctx, secondReversal)
	if err != nil || second.DebtDelta != 20 || second.DebtPoints != 50 || second.ReversalShortfall != 50 {
		t.Fatalf("apply second partial points reversal: result=%+v err=%v", second, err)
	}
	debtAward := PointsAwardInput{
		CustomerNo: "customer-a", SourceType: "order", SourceReference: "order-points-0002", Points: 60,
	}
	repaid, err := service.Award(ctx, debtAward)
	if err != nil || repaid.DebtDelta != -50 || repaid.DebtPoints != 0 ||
		repaid.AvailableDelta != 10 || repaid.AvailablePoints != 10 || repaid.LotReversedPoints != 50 {
		t.Fatalf("repay points debt from new award: result=%+v err=%v", repaid, err)
	}
	var evidence struct {
		EarnCount    int `orm:"earn_count"`
		ReverseCount int `orm:"reverse_count"`
		DebtRepay    int `orm:"debt_repay_count"`
	}
	if err = database.GetScan(ctx, &evidence, `
SELECT count(*) FILTER (WHERE entry_type_code = 'earn') AS earn_count,
       count(*) FILTER (WHERE entry_type_code = 'reverse') AS reverse_count,
       count(*) FILTER (WHERE entry_type_code = 'debt_repay') AS debt_repay_count
FROM plugin_idc_core_marketing_points_entry
WHERE customer_no = 'customer-a'`); err != nil ||
		evidence.EarnCount != 2 || evidence.ReverseCount != 2 || evidence.DebtRepay != 1 {
		t.Fatalf("unexpected append-only points evidence: %+v err=%v", evidence, err)
	}
	verifyConcurrentPointsReversal(t, ctx, database, service)
}

func verifyConcurrentPointsReversal(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	service *pointsServiceImpl,
) {
	t.Helper()
	award := PointsAwardInput{
		CustomerNo: "customer-b", SourceType: "order", SourceReference: "order-points-concurrent", Points: 100,
	}
	awarded, err := service.Award(ctx, award)
	if err != nil {
		t.Fatalf("prepare concurrent points award: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_lot
SET available_points = 0, consumed_points = 100, status_code = 'exhausted'
WHERE lot_no = ?`, awarded.LotNo); err != nil {
		t.Fatalf("prepare concurrent consumed lot: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_account
SET available_points = 0 WHERE customer_no = 'customer-b'`); err != nil {
		t.Fatalf("prepare concurrent account projection: %v", err)
	}
	errors := make([]error, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range errors {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, errors[index] = service.ReverseAward(ctx, PointsReverseInput{
				CustomerNo: "customer-b", AwardSourceType: "order",
				AwardSourceReference: "order-points-concurrent", ReversalSourceType: "refund",
				ReversalSourceReference: "refund-points-concurrent-" + string(rune('a'+index)), Points: 60,
			})
		}(index)
	}
	close(start)
	wait.Wait()
	successes := 0
	conflicts := 0
	for _, operationErr := range errors {
		if operationErr == nil {
			successes++
		} else if bizerr.Is(operationErr, CodePointsConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent points reversal error: %v", operationErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent points reversal outcome successes=%d conflicts=%d errors=%v", successes, conflicts, errors)
	}
	var state struct {
		Debt      int64 `orm:"debt_points"`
		Shortfall int64 `orm:"reversal_shortfall_points"`
		Entries   int   `orm:"entry_count"`
	}
	if err = database.GetScan(ctx, &state, `
SELECT a.debt_points, l.reversal_shortfall_points,
       count(e.id) FILTER (WHERE e.entry_type_code = 'reverse') AS entry_count
FROM plugin_idc_core_marketing_points_account a
JOIN plugin_idc_core_marketing_points_lot l ON l.customer_no = a.customer_no
LEFT JOIN plugin_idc_core_marketing_points_entry e ON e.lot_no = l.lot_no
WHERE a.customer_no = 'customer-b' AND l.lot_no = ?
GROUP BY a.debt_points, l.reversal_shortfall_points`, awarded.LotNo); err != nil ||
		state.Debt != 60 || state.Shortfall != 60 || state.Entries != 1 {
		t.Fatalf("concurrent points reversal state=%+v err=%v", state, err)
	}
}

func verifyCouponAssetReservationLifecycle(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	service *serviceImpl,
) {
	t.Helper()
	input := promotionIntegrationInput("customer-a", "coupon-quote-0001")
	input.ProductNo = "coupon-product"
	input.AssetNumbers = []string{"asset-coupon-a"}
	reserved, err := service.Reserve(ctx, input)
	if err != nil || len(reserved.Applied) != 1 || reserved.Applied[0].AssetNo != "asset-coupon-a" {
		t.Fatalf("reserve coupon asset: result=%+v err=%v", reserved, err)
	}
	assertCouponCounters(t, ctx, database, 1, 0, 1, 0)
	if _, err = service.Release(ctx, input.CustomerNo, input.QuoteReference); err != nil {
		t.Fatalf("release coupon asset: %v", err)
	}
	assertCouponCounters(t, ctx, database, 0, 0, 0, 0)
	input.QuoteReference = "coupon-quote-0002"
	if _, err = service.Reserve(ctx, input); err != nil {
		t.Fatalf("reserve coupon asset for consumption: %v", err)
	}
	if _, err = service.Consume(ctx, input.CustomerNo, input.QuoteReference); err != nil {
		t.Fatalf("consume coupon asset: %v", err)
	}
	assertCouponCounters(t, ctx, database, 0, 1, 0, 1)
	status, err := database.GetValue(ctx, `
SELECT status_code FROM plugin_idc_core_marketing_promotion_asset
WHERE asset_no = 'asset-coupon-a'`)
	if err != nil || status.String() != "exhausted" {
		t.Fatalf("coupon asset status=%q err=%v", status.String(), err)
	}
}

func assertCouponCounters(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	wantCustomerReserved int,
	wantCustomerConsumed int,
	wantAssetReserved int,
	wantAssetConsumed int,
) {
	t.Helper()
	var counters struct {
		CustomerReserved int `orm:"customer_reserved"`
		CustomerConsumed int `orm:"customer_consumed"`
		AssetReserved    int `orm:"asset_reserved"`
		AssetConsumed    int `orm:"asset_consumed"`
	}
	err := database.GetScan(ctx, &counters, `
SELECT q.reserved_count AS customer_reserved, q.consumed_count AS customer_consumed,
       a.reserved_uses AS asset_reserved, a.consumed_uses AS asset_consumed
FROM plugin_idc_core_marketing_customer_quota q
JOIN plugin_idc_core_marketing_promotion_asset a
  ON a.campaign_no = q.campaign_no AND a.customer_no = q.customer_no
WHERE q.campaign_no = 'campaign-coupon' AND q.customer_no = 'customer-a'`)
	if err != nil || counters.CustomerReserved != wantCustomerReserved ||
		counters.CustomerConsumed != wantCustomerConsumed ||
		counters.AssetReserved != wantAssetReserved || counters.AssetConsumed != wantAssetConsumed {
		t.Fatalf("coupon counters=%+v want=(%d,%d,%d,%d) err=%v", counters,
			wantCustomerReserved, wantCustomerConsumed, wantAssetReserved, wantAssetConsumed, err,
		)
	}
}

func deterministicReservationEntropy() []byte {
	value := make([]byte, 2048)
	for index := range value {
		value[index] = byte(index)
	}
	return value
}

func configureMarketingIntegrationDB(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(marketingIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL promotion integration tests", marketingIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create marketing integration admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate marketing integration schema: %v", err)
	}
	schema := "linapro_promotion_test_" + hex.EncodeToString(suffix[:])
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteMarketingIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated promotion schema: %v", err)
	}
	database, err := gdb.New(gdb.ConfigNode{
		Link: appendMarketingDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped promotion database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped promotion database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_promotion_test_") {
			t.Errorf("refusing to drop unexpected promotion schema %q", schema)
		} else if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteMarketingIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop isolated promotion schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close promotion admin database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, marketingDictionaryFixtureSQL); err != nil {
		t.Fatalf("create promotion dictionary fixtures: %v", err)
	}
	for _, path := range marketingIntegrationMigrationPaths(t) {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read promotion migration %s: %v", filepath.Base(path), readErr)
		}
		attempts := 1
		if filepath.Base(path) == "062-marketing-performance-abuse.sql" {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("apply promotion migration %s attempt %d: %v", filepath.Base(path), attempt+1, err)
			}
		}
	}
	return database
}

func seedPromotionService(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	currency := "CNY"
	amount := int64(500)
	couponDigest, digestErr := benefitDigest(candidateRow{
		CampaignNo: "campaign-coupon", CampaignType: "coupon", CampaignVersion: 1,
		BenefitNo: "benefit-coupon", BenefitType: "fixed_discount", BenefitVersion: 1,
		CurrencyCode: &currency, AmountMinor: &amount, PurchasePhase: "both", CycleScope: "all",
	})
	if digestErr != nil {
		t.Fatalf("build coupon fixture digest: %v", digestErr)
	}
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_subject
    (customer_no, customer_type_code, status_code, display_name)
VALUES
    ('customer-a', 'individual', 'active', 'Customer A'),
    ('customer-b', 'individual', 'active', 'Customer B');
INSERT INTO plugin_idc_core_merchant_profile
    (tenant_id, merchant_no, display_name, legal_name, default_currency, enabled)
VALUES (0, 'merchant-marketing', 'Marketing Merchant', 'Marketing Merchant Ltd.', 'CNY', TRUE);
INSERT INTO plugin_idc_core_customer_contact (
    contact_no, customer_no, display_name, email, email_lookup_digest,
    email_verified_at, phone_country_code, phone, phone_lookup_digest,
    phone_verified_at, preferred_locale
) VALUES
    ('contact-a', 'customer-a', 'Contact A', 'a@example.invalid', repeat('a', 64),
     CURRENT_TIMESTAMP, '+86', '13800000001', repeat('b', 64), CURRENT_TIMESTAMP, 'zh-CN'),
    ('contact-b', 'customer-b', 'Contact B', 'b@example.invalid', repeat('c', 64),
     CURRENT_TIMESTAMP, '+86', '13800000002', repeat('d', 64), CURRENT_TIMESTAMP, 'en-US');
INSERT INTO plugin_idc_core_marketing_segment (
    segment_no, segment_code, name_zh_cn, name_en_us, mode_code,
    refresh_interval_minutes, definition_digest, status_code
) VALUES (
    'segment-engagement', 'segment_engagement', '营销触达分群', 'Engagement audience',
    'static', NULL, repeat('e', 64), 'active'
);
INSERT INTO plugin_idc_core_marketing_segment_snapshot (
    snapshot_no, segment_no, segment_version, definition_digest, status_code,
    member_count, content_digest, completed_at
) VALUES (
    'snapshot-engagement', 'segment-engagement', 1, repeat('e', 64), 'complete',
    1, repeat('f', 64), CURRENT_TIMESTAMP
);
INSERT INTO plugin_idc_core_marketing_snapshot_member (snapshot_no, customer_no)
VALUES ('snapshot-engagement', 'customer-a');
INSERT INTO plugin_idc_core_marketing_campaign (
    campaign_no, campaign_code, name_zh_cn, name_en_us, campaign_type_code,
    target_segment_no, target_snapshot_no, starts_at, ends_at, priority,
    stack_policy_code, stack_group_code, status_code
) VALUES (
    'campaign-engagement', 'campaign_engagement', '营销触达活动', 'Engagement campaign',
    'discount', 'segment-engagement', 'snapshot-engagement',
    CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
    30, 'stackable', 'engagement', 'active'
);
INSERT INTO plugin_idc_core_marketing_campaign (
    campaign_no, campaign_code, name_zh_cn, name_en_us, campaign_type_code,
    starts_at, ends_at, priority, stack_policy_code, stack_group_code,
    global_quota_limit, status_code
) VALUES (
    'campaign-quota', 'campaign_quota', '名额活动', 'Quota campaign', 'discount',
    CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
    10, 'stackable', 'default', 1, 'active'
);
INSERT INTO plugin_idc_core_marketing_campaign_scope
    (campaign_no, scope_kind_code, match_mode_code, reference_code)
VALUES ('campaign-quota', 'product', 'include', 'product-1');
INSERT INTO plugin_idc_core_marketing_benefit_rule (
    benefit_no, campaign_no, campaign_type_code, benefit_type_code, currency_code,
    amount_minor, purchase_phase_code, cycle_scope_code
) VALUES (
    'benefit-quota', 'campaign-quota', 'discount', 'fixed_discount', 'CNY',
    100, 'both', 'all'
);
INSERT INTO plugin_idc_core_marketing_campaign (
    campaign_no, campaign_code, name_zh_cn, name_en_us, campaign_type_code,
    starts_at, ends_at, priority, stack_policy_code, stack_group_code, status_code
) VALUES (
    'campaign-quote', 'campaign_quote', '整单活动', 'Quote-wide campaign', 'discount',
    CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
    15, 'stackable', 'quote', 'active'
);
INSERT INTO plugin_idc_core_marketing_campaign_scope
    (campaign_no, scope_kind_code, match_mode_code, reference_code)
VALUES ('campaign-quote', 'product', 'include', 'product-quote');
INSERT INTO plugin_idc_core_marketing_benefit_rule (
    benefit_no, campaign_no, campaign_type_code, benefit_type_code, currency_code,
    amount_minor, purchase_phase_code, cycle_scope_code
) VALUES (
    'benefit-quote', 'campaign-quote', 'discount', 'fixed_discount', 'CNY',
    100, 'new_purchase', 'first'
);
INSERT INTO plugin_idc_core_marketing_campaign (
    campaign_no, campaign_code, name_zh_cn, name_en_us, campaign_type_code,
    starts_at, ends_at, priority, stack_policy_code, stack_group_code,
    global_quota_limit, per_customer_quota_limit, status_code
) VALUES (
    'campaign-coupon', 'campaign_coupon', '优惠券活动', 'Coupon campaign', 'coupon',
    CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
    5, 'stackable', 'coupon', 10, 1, 'active'
), (
	'campaign-recharge-bonus', 'campaign_recharge_bonus', '充值赠送', 'Recharge bonus',
	'recharge_bonus', CURRENT_TIMESTAMP - INTERVAL '1 hour',
	CURRENT_TIMESTAMP + INTERVAL '1 day', 20, 'stackable', 'value_grant', 100, 10, 'active'
), (
	'campaign-cashback', 'campaign_cashback', '订单返现', 'Order cashback',
	'cashback', CURRENT_TIMESTAMP - INTERVAL '1 hour',
	CURRENT_TIMESTAMP + INTERVAL '1 day', 21, 'stackable', 'value_grant', 100, 10, 'active'
), (
	'campaign-value-quota', 'campaign_value_quota', '返现名额', 'Cashback quota',
	'cashback', CURRENT_TIMESTAMP - INTERVAL '1 hour',
	CURRENT_TIMESTAMP + INTERVAL '1 day', 22, 'stackable', 'value_grant', 1, 1, 'active'
);
INSERT INTO plugin_idc_core_marketing_campaign_scope
    (campaign_no, scope_kind_code, match_mode_code, reference_code)
VALUES ('campaign-coupon', 'product', 'include', 'coupon-product');
INSERT INTO plugin_idc_core_marketing_benefit_rule (
    benefit_no, campaign_no, campaign_type_code, benefit_type_code, currency_code,
    amount_minor, purchase_phase_code, cycle_scope_code
) VALUES (
    'benefit-coupon', 'campaign-coupon', 'coupon', 'fixed_discount', 'CNY',
    500, 'both', 'all'
), (
    'benefit-recharge-bonus', 'campaign-recharge-bonus', 'recharge_bonus',
    'recharge_bonus', 'CNY', 5000, 'both', 'all'
), (
	'benefit-cashback', 'campaign-cashback', 'cashback', 'cashback', 'CNY',
	1000, 'both', 'all'
), (
	'benefit-value-quota', 'campaign-value-quota', 'cashback', 'cashback', 'CNY',
	700, 'both', 'all'
);`)
	if err != nil {
		t.Fatalf("seed promotion service: %v", err)
	}
	_, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_promotion_asset (
    asset_no, campaign_no, benefit_no, benefit_type_code, asset_type_code,
    customer_no, benefit_snapshot_digest, currency_code, face_amount_minor,
    usage_limit, valid_from, valid_until, status_code, claimed_at
) VALUES (
    'asset-coupon-a', 'campaign-coupon', 'benefit-coupon', 'fixed_discount', 'coupon',
    'customer-a', ?, 'CNY', 500, 1, CURRENT_TIMESTAMP - INTERVAL '1 hour',
    CURRENT_TIMESTAMP + INTERVAL '1 day', 'active', CURRENT_TIMESTAMP
)`, couponDigest)
	if err != nil {
		t.Fatalf("seed promotion service: %v", err)
	}
}

func promotionIntegrationInput(customerNo string, quoteReference string) EvaluateInput {
	return EvaluateInput{
		CustomerNo: customerNo, QuoteReference: quoteReference, CurrencyCode: "CNY",
		SubtotalMinor: 1000, PurchasePhase: PurchasePhaseNewPurchase, CycleNumber: 1,
		ProductNo: "product-1", CategoryNo: "category-1",
	}
}

func marketingIntegrationMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve promotion integration migration path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	return []string{
		filepath.Join(root, "001-core-foundation.sql"),
		filepath.Join(root, "003-customer-account.sql"),
		filepath.Join(root, "010-catalog-foundation.sql"),
		filepath.Join(root, "013-catalog-billing-policy.sql"),
		filepath.Join(root, "014-catalog-inventory-routing.sql"),
		filepath.Join(root, "019-catalog-tax-policy.sql"),
		filepath.Join(root, "020-marketing-foundation.sql"),
		filepath.Join(root, "021-marketing-send-governance.sql"),
		filepath.Join(root, "022-commerce-foundation.sql"),
		filepath.Join(root, "023-billing-ledger-foundation.sql"),
		filepath.Join(root, "027-billing-invoice-lifecycle.sql"),
		filepath.Join(root, "028-billing-invoice-transform.sql"),
		filepath.Join(root, "029-billing-reviewed-adjustment.sql"),
		filepath.Join(root, "034-credit-account-foundation.sql"),
		filepath.Join(root, "049-marketing-financial-adjustment.sql"),
		filepath.Join(root, "062-marketing-performance-abuse.sql"),
		filepath.Join(root, "073-general-product-catalog.sql"),
		filepath.Join(root, "133-direct-admin-financial-actions.sql"),
	}
}

func quoteMarketingIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendMarketingDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

const marketingDictionaryFixtureSQL = `
CREATE TABLE sys_dict_type (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, dict_type VARCHAR(128) NOT NULL DEFAULT '',
    label VARCHAR(128) NOT NULL DEFAULT '', value VARCHAR(128) NOT NULL DEFAULT '',
    sort INT NOT NULL DEFAULT 0, tag_style VARCHAR(128) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    UNIQUE (tenant_id, dict_type, value)
);`
