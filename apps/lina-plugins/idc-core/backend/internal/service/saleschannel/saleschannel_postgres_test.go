// Package saleschannel verifies the bounded persistence lookup used by agent pricing.
package saleschannel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestEvaluateStoredPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run the sales-channel PostgreSQL lookup test")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_sales_lookup_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA \""+schema+"\""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, "DROP SCHEMA \""+schema+"\" CASCADE"); cleanupErr != nil {
			t.Errorf("drop isolated sales-channel schema: %v", cleanupErr)
		}
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendSalesChannelSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	fixture := `
CREATE TABLE plugin_idc_core_sales_agent_qualification (
 id BIGINT PRIMARY KEY, tenant_id INT NOT NULL, qualification_no VARCHAR(40) NOT NULL,
 customer_no VARCHAR(40) NOT NULL, level_no VARCHAR(40) NOT NULL, status_code VARCHAR(24) NOT NULL,
 effective_at TIMESTAMPTZ NOT NULL, expires_at TIMESTAMPTZ, version BIGINT NOT NULL);
CREATE TABLE plugin_idc_core_sales_agent_product_rule (
 id BIGINT PRIMARY KEY, tenant_id INT NOT NULL, product_rule_no VARCHAR(40) NOT NULL,
 level_no VARCHAR(40) NOT NULL, product_no VARCHAR(40) NOT NULL, status_code VARCHAR(16) NOT NULL,
 effective_at TIMESTAMPTZ, expires_at TIMESTAMPTZ, version BIGINT NOT NULL, rule_digest CHAR(64) NOT NULL);
CREATE TABLE plugin_idc_core_sales_agent_price_rule (
 id BIGINT PRIMARY KEY, tenant_id INT NOT NULL, price_rule_no VARCHAR(40) NOT NULL,
 product_rule_no VARCHAR(40) NOT NULL, currency_code CHAR(3) NOT NULL, adjustment_type_code VARCHAR(24) NOT NULL,
 fixed_price_minor BIGINT, discount_basis_points INT, minimum_margin_minor BIGINT NOT NULL,
 status_code VARCHAR(16) NOT NULL, effective_at TIMESTAMPTZ, expires_at TIMESTAMPTZ, version BIGINT NOT NULL, rule_digest CHAR(64) NOT NULL);
CREATE TABLE plugin_idc_core_sales_agent_credit_limit (
 id BIGINT PRIMARY KEY, tenant_id INT NOT NULL, limit_no VARCHAR(40) NOT NULL,
 qualification_no VARCHAR(40) NOT NULL, currency_code CHAR(3) NOT NULL, limit_amount_minor BIGINT NOT NULL,
 status_code VARCHAR(16) NOT NULL, effective_at TIMESTAMPTZ NOT NULL, expires_at TIMESTAMPTZ);
INSERT INTO plugin_idc_core_sales_agent_qualification VALUES (1,0,'qual_lookup','cust_lookup','level_gold','active','2026-08-29T00:00:00Z',NULL,4);
INSERT INTO plugin_idc_core_sales_agent_product_rule VALUES (1,0,'prod_rule_lookup','level_gold','prod_lookup','published','2026-08-29T00:00:00Z',NULL,6,repeat('a',64));
INSERT INTO plugin_idc_core_sales_agent_price_rule VALUES (1,0,'price_rule_lookup','prod_rule_lookup','CNY','fixed_price',9000,NULL,1000,'published','2026-08-29T00:00:00Z',NULL,8,repeat('b',64));
INSERT INTO plugin_idc_core_sales_agent_credit_limit VALUES (1,0,'limit_lookup','qual_lookup','CNY',12000,'active','2026-08-29T00:00:00Z',NULL);`
	if _, err = database.Exec(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	service, err := NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	offer, err := service.EvaluateStored(ctx, "cust_lookup", "prod_lookup", "CNY", 10000, 7000, 1000, time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Eligible || offer.PriceMinor != 9000 || offer.RemainingCreditMinor == nil || *offer.RemainingCreditMinor != 2000 || offer.Snapshot.PriceRuleVersion != 8 {
		t.Fatalf("unexpected stored offer: %+v", offer)
	}
}

func TestLoadOverviewPostgreSQLUsesCommissionLedgerDeltas(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run the sales-channel overview query test")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_sales_overview_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`) })
	database, err := gdb.New(gdb.ConfigNode{Link: appendSalesChannelSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_core_sales_agent_qualification (id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT, status_code TEXT);
CREATE TABLE plugin_idc_core_sales_affiliate_link (id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT, status_code TEXT);
CREATE TABLE plugin_idc_core_sales_affiliate_attribution (id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT, status_code TEXT);
CREATE TABLE plugin_idc_core_commission_accrual (id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT, status_code TEXT);
CREATE TABLE plugin_idc_core_sales_commission_settlement (id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT, status_code TEXT);
CREATE TABLE plugin_idc_core_sales_commission_account_entry (id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT, currency_code CHAR(3), entry_type_code TEXT, delta_credited_minor BIGINT, delta_available_minor BIGINT);
INSERT INTO plugin_idc_core_sales_agent_qualification (tenant_id,status_code) VALUES (0,'active');
INSERT INTO plugin_idc_core_sales_affiliate_link (tenant_id,status_code) VALUES (0,'active');
INSERT INTO plugin_idc_core_sales_affiliate_attribution (tenant_id,status_code) VALUES (0,'eligible');
INSERT INTO plugin_idc_core_commission_accrual (tenant_id,status_code) VALUES (0,'pending_maturity');
INSERT INTO plugin_idc_core_sales_commission_settlement (tenant_id,status_code) VALUES (0,'pending_review');
INSERT INTO plugin_idc_core_sales_commission_account_entry (tenant_id,currency_code,entry_type_code,delta_credited_minor,delta_available_minor) VALUES (0,'CNY','accrual',12500,0),(0,'CNY','maturity',0,8000),(0,'USD','accrual',300,0),(0,'USD','maturity',0,100);`); err != nil {
		t.Fatal(err)
	}
	service, err := NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	overview, err := service.LoadOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Qualifications != 1 || overview.AffiliateLinks != 1 || overview.Attributions != 1 || overview.PendingMaturity != 1 || overview.PendingSettlement != 1 {
		t.Fatalf("unexpected sales overview counters: %+v", overview)
	}
	if len(overview.Currencies) != 2 || overview.Currencies[0].CurrencyCode != "CNY" || overview.Currencies[0].AccruedMinor != 12500 || overview.Currencies[0].MaturedMinor != 8000 {
		t.Fatalf("unexpected sales overview currencies: %+v", overview.Currencies)
	}
}

func TestLoadManagementReferrerPostgreSQLProjectsNumericCustomerCode(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run the management referrer query test")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_management_referrer_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`); cleanupErr != nil {
			t.Errorf("drop isolated management referrer schema: %v", cleanupErr)
		}
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendSalesChannelSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_core_sales_affiliate_attribution (
 id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id BIGINT NOT NULL,
 referrer_customer_no TEXT NOT NULL, referred_customer_no TEXT NOT NULL,
 status_code TEXT NOT NULL);
CREATE TABLE plugin_idc_core_customer_subject (
 tenant_id BIGINT NOT NULL, customer_no TEXT NOT NULL,
 customer_code BIGINT NOT NULL, display_name TEXT NOT NULL);
INSERT INTO plugin_idc_core_customer_subject
 (tenant_id,customer_no,customer_code,display_name)
 VALUES (0,'CUS_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',10023,'推荐客户');
INSERT INTO plugin_idc_core_sales_affiliate_attribution
 (tenant_id,referrer_customer_no,referred_customer_no,status_code)
 VALUES (0,'CUS_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','CUS_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','eligible');`); err != nil {
		t.Fatal(err)
	}
	service, err := NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	reader, ok := service.(ManagementReferrerReader)
	if !ok {
		t.Fatal("sales-channel service does not expose the management referrer projection")
	}
	referrer, err := reader.LoadManagementReferrer(ctx, "CUS_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if referrer.CustomerCode != "10023" || referrer.DisplayName != "推荐客户" {
		t.Fatalf("unexpected management referrer: %+v", referrer)
	}
}

func TestPersistStoredAttributionPostgreSQLIsIdempotent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run the sales-channel attribution persistence test")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_sales_attribution_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`) })
	database, err := gdb.New(gdb.ConfigNode{Link: appendSalesChannelSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_core_sales_affiliate_link (
 id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT NOT NULL, link_no TEXT NOT NULL,
 owner_customer_no TEXT NOT NULL, code_digest CHAR(64) NOT NULL, attribution_policy_code TEXT NOT NULL,
 attribution_window_seconds INT NOT NULL, status_code TEXT NOT NULL, expires_at TIMESTAMPTZ,
 version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_sales_affiliate_touch (
 id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT NOT NULL, touch_no TEXT NOT NULL, link_no TEXT NOT NULL,
 visitor_digest CHAR(64) NOT NULL, device_digest CHAR(64) NOT NULL, ip_digest CHAR(64) NOT NULL,
 evidence_digest CHAR(64) NOT NULL, occurred_at TIMESTAMPTZ NOT NULL, expires_at TIMESTAMPTZ NOT NULL);
CREATE TABLE plugin_idc_core_sales_affiliate_attribution (
 id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT NOT NULL, attribution_no TEXT NOT NULL UNIQUE,
 link_no TEXT NOT NULL, touch_no TEXT NOT NULL, referrer_customer_no TEXT NOT NULL,
 referred_customer_no TEXT NOT NULL, policy_code TEXT NOT NULL, status_code TEXT NOT NULL,
 risk_reason_code TEXT, evidence_digest CHAR(64) NOT NULL, attributed_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL);
CREATE UNIQUE INDEX attribution_one_eligible ON plugin_idc_core_sales_affiliate_attribution(referred_customer_no) WHERE status_code='eligible';
CREATE TABLE plugin_idc_core_customer_individual_profile (
 tenant_id INT NOT NULL, customer_no TEXT NOT NULL, identity_status_code TEXT NOT NULL,
 identity_lookup_digest CHAR(64) NOT NULL);
CREATE TABLE plugin_idc_core_customer_enterprise_profile (
 tenant_id INT NOT NULL, customer_no TEXT NOT NULL, verification_status_code TEXT NOT NULL,
 registration_lookup_digest CHAR(64) NOT NULL);
CREATE TABLE plugin_idc_core_marketing_abuse_case (
 id BIGINT GENERATED ALWAYS AS IDENTITY, tenant_id INT NOT NULL, case_no TEXT NOT NULL UNIQUE,
 campaign_no TEXT, customer_no TEXT, signal_code TEXT NOT NULL,
 status_code TEXT NOT NULL DEFAULT 'pending_review', related_account_count INT NOT NULL,
 evidence_summary_code TEXT NOT NULL, evidence_digest CHAR(64) NOT NULL,
 window_started_at TIMESTAMPTZ NOT NULL, window_ended_at TIMESTAMPTZ NOT NULL,
 observed_count INT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE UNIQUE INDEX marketing_abuse_evidence ON plugin_idc_core_marketing_abuse_case
 (COALESCE(campaign_no,''),signal_code,evidence_digest);
INSERT INTO plugin_idc_core_sales_affiliate_link
 (tenant_id,link_no,owner_customer_no,code_digest,attribution_policy_code,attribution_window_seconds,status_code,expires_at)
 VALUES (0,'AFF_STORED','CUST_OWNER',repeat('a',64),'first_touch',3600,'active','2026-08-30T14:00:00Z');
INSERT INTO plugin_idc_core_sales_affiliate_touch
 (tenant_id,touch_no,link_no,visitor_digest,device_digest,ip_digest,evidence_digest,occurred_at,expires_at)
 VALUES (0,'TOUCH_STORED','AFF_STORED',repeat('b',64),repeat('c',64),repeat('d',64),repeat('e',64),'2026-08-30T11:50:00Z','2026-08-30T12:50:00Z');
INSERT INTO plugin_idc_core_customer_individual_profile
 (tenant_id,customer_no,identity_status_code,identity_lookup_digest)
 VALUES (0,'CUST_OWNER','verified',repeat('f',64)),(0,'CUST_SAME_IDENTITY','verified',repeat('f',64));`); err != nil {
		t.Fatal(err)
	}
	service, err := NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	decision, err := service.PersistStoredAttribution(ctx, now, "AFF_STORED", "TOUCH_STORED", "CUST_REFERRED")
	if err != nil || decision.StatusCode != AffiliateEligible || decision.ReferrerCustomerNo != "CUST_OWNER" {
		t.Fatalf("persist stored attribution decision=%+v err=%v", decision, err)
	}
	count, err := database.Model("plugin_idc_core_sales_affiliate_attribution").Ctx(ctx).Count()
	if err != nil || count != 1 {
		t.Fatalf("persisted attribution count=%d err=%v", count, err)
	}
	duplicate, err := service.PersistStoredAttribution(ctx, now, "AFF_STORED", "TOUCH_STORED", "CUST_REFERRED")
	if err != nil || duplicate.StatusCode != AffiliateRejectedRisk || duplicate.RiskReasonCode != AffiliateReasonDuplicate {
		t.Fatalf("duplicate attribution decision=%+v err=%v", duplicate, err)
	}
	count, err = database.Model("plugin_idc_core_sales_affiliate_attribution").Ctx(ctx).Count()
	if err != nil || count != 1 {
		t.Fatalf("duplicate changed attribution count=%d err=%v", count, err)
	}

	sameIdentity, err := service.PersistStoredAttribution(ctx, now, "AFF_STORED", "TOUCH_STORED", "CUST_SAME_IDENTITY")
	if err != nil || sameIdentity.StatusCode != AffiliateRejectedSelfReferral || sameIdentity.RiskReasonCode != AffiliateReasonRelatedIdentity {
		t.Fatalf("same-identity attribution decision=%+v err=%v", sameIdentity, err)
	}
	count, err = database.Model("plugin_idc_core_sales_affiliate_attribution").Ctx(ctx).Count()
	if err != nil || count != 2 {
		t.Fatalf("same-identity attribution count=%d err=%v", count, err)
	}
	var abuseCase *struct {
		CustomerNo          string `orm:"customer_no"`
		StatusCode          string `orm:"status_code"`
		RelatedAccountCount int    `orm:"related_account_count"`
		EvidenceSummaryCode string `orm:"evidence_summary_code"`
		EvidenceDigest      string `orm:"evidence_digest"`
	}
	if err = database.GetScan(ctx, &abuseCase, `SELECT customer_no,status_code,related_account_count,evidence_summary_code,evidence_digest
FROM plugin_idc_core_marketing_abuse_case WHERE signal_code='self_referral'`); err != nil {
		t.Fatal(err)
	}
	if abuseCase == nil || abuseCase.CustomerNo != "CUST_SAME_IDENTITY" || abuseCase.StatusCode != "pending_review" ||
		abuseCase.RelatedAccountCount != 2 || abuseCase.EvidenceSummaryCode != "verified_subject_self_referral" || len(abuseCase.EvidenceDigest) != 64 {
		t.Fatalf("unexpected self-referral review case: %+v", abuseCase)
	}
	replayed, err := service.PersistStoredAttribution(ctx, now, "AFF_STORED", "TOUCH_STORED", "CUST_SAME_IDENTITY")
	if err != nil || replayed.StatusCode != AffiliateRejectedSelfReferral || replayed.RiskReasonCode != AffiliateReasonRelatedIdentity {
		t.Fatalf("replayed same-identity decision=%+v err=%v", replayed, err)
	}
	caseCount, err := database.Model("plugin_idc_core_marketing_abuse_case").Ctx(ctx).Count()
	if err != nil || caseCount != 1 {
		t.Fatalf("replayed same-identity case count=%d err=%v", caseCount, err)
	}
}

func appendSalesChannelSearchPath(dsn, schema string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "search_path=" + url.QueryEscape(schema)
}
