package marketing

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/billing"
)

// All writes are confined to configureMarketingIntegrationDB's random schema.
// Real migrations retain the production constraints; no live business rows change.
func TestMarketingNumberedDirectoriesPostgreSQL(t *testing.T) {
	ctx := context.Background()
	db := configureMarketingIntegrationDB(t, ctx)
	seedPromotionService(t, ctx, db)
	_, err := db.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_segment
 (segment_no,segment_code,name_zh_cn,name_en_us,mode_code,definition_digest,status_code)
 SELECT 'segment-page-'||n,'page_'||n,'分页分群'||n,'Page segment '||n,'static',repeat('e',64),'active'
 FROM generate_series(1,25) n;
INSERT INTO plugin_idc_core_marketing_campaign
 (campaign_no,campaign_code,name_zh_cn,name_en_us,campaign_type_code,starts_at,ends_at,priority,stack_policy_code,stack_group_code,status_code)
 SELECT 'campaign-page-'||n,'page_'||n,'分页活动'||n,'Page campaign '||n,'discount',CURRENT_TIMESTAMP-INTERVAL '1 hour',CURRENT_TIMESTAMP+INTERVAL '1 day',10,'stackable','default','active'
 FROM generate_series(1,25) n;
INSERT INTO plugin_idc_core_marketing_promotion_asset
 (asset_no,campaign_no,benefit_no,benefit_type_code,asset_type_code,customer_no,benefit_snapshot_digest,currency_code,face_amount_minor,usage_limit,valid_from,valid_until,status_code,claimed_at)
 SELECT 'asset-page-'||n,campaign_no,benefit_no,benefit_type_code,asset_type_code,customer_no,benefit_snapshot_digest,currency_code,face_amount_minor,usage_limit,valid_from,valid_until,status_code,CURRENT_TIMESTAMP
 FROM plugin_idc_core_marketing_promotion_asset CROSS JOIN generate_series(1,25) n WHERE asset_no='asset-coupon-a';
INSERT INTO plugin_idc_core_customer_subject (customer_no,customer_type_code,status_code,display_name)
 SELECT 'customer-page-'||n,'individual','active','Page customer '||n FROM generate_series(1,25) n;
INSERT INTO plugin_idc_core_marketing_points_account (customer_no)
 SELECT 'customer-page-'||n FROM generate_series(1,25) n;
INSERT INTO plugin_idc_core_marketing_effect_fact
 (fact_no,campaign_no,event_type_code,currency_code,amount_minor,source_type_code,source_reference,content_digest,occurred_at)
 SELECT 'fact-page-'||n||'-'||currency,'campaign-page-'||n,'receipt',currency,123,'pagination_test','source-'||n||'-'||currency,repeat('1',64),CURRENT_TIMESTAMP
 FROM generate_series(1,25) n CROSS JOIN (VALUES ('CNY'),('USD')) money(currency);
INSERT INTO plugin_idc_core_marketing_abuse_case
 (case_no,campaign_no,customer_no,signal_code,related_account_count,evidence_summary_code,evidence_digest,window_started_at,window_ended_at,observed_count)
 SELECT 'abuse-page-'||n,'campaign-coupon','customer-a','duplicate_newcomer_claim',2,'verified_subject_reused',md5(n::text)||md5(n::text),CURRENT_TIMESTAMP-INTERVAL '1 day',CURRENT_TIMESTAMP,2
 FROM generate_series(1,25) n;`)
	if err != nil {
		t.Fatalf("seed numbered directories: %v", err)
	}
	grants, err := NewValueGrantService(db)
	if err != nil {
		t.Fatal(err)
	}
	points, err := NewPointsService(db)
	if err != nil {
		t.Fatal(err)
	}
	financial, err := billing.NewFinancialAdjustmentService(db)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := NewManagementActionService(db, &marketingActionAuditIntegrationStub{}, &managementActionAuthorizerStub{}, points, grants, financial)
	if err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 25; n++ {
		grant, createErr := grants.CreatePending(ctx, ValueGrantCreateInput{GrantType: ValueGrantTypeRechargeBonus,
			CampaignNo: "campaign-recharge-bonus", BenefitNo: "benefit-recharge-bonus", CustomerNo: fmt.Sprintf("customer-page-%d", n),
			CurrencyCode: "CNY", SourceReference: fmt.Sprintf("page-recharge-%d", n), AmountMinor: 5000})
		if createErr != nil {
			t.Fatalf("create grant %d: %v", n, createErr)
		}
		if _, createErr = actions.RequestValueGrantPosting(ctx, ValueGrantFinancialRequestInput{RequestNo: fmt.Sprintf("page-grant-request-%d", n), GrantNo: grant.GrantNo, ReasonCode: "customer_service"}); createErr != nil {
			t.Fatalf("create reviewed adjustment %d: %v", n, createErr)
		}
	}
	directory, err := NewManagementDirectoryReader(db, &marketingDirectoryAuthorizerStub{}, bytes.Repeat([]byte{0x66}, 32))
	if err != nil {
		t.Fatal(err)
	}
	assets, err := NewAssetDirectoryReader(db, &assetDirectoryAuthorizerStub{}, bytes.Repeat([]byte{0x67}, 32))
	if err != nil {
		t.Fatal(err)
	}
	insights, err := NewMarketingInsightsService(db, &marketingInsightsAuthorizerStub{}, &marketingActionAuditIntegrationStub{})
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]func(int, int) ([]string, int, error){
		"segments": func(page, size int) ([]string, int, error) {
			r, e := directory.SearchSegments(ctx, SegmentManagementSearchInput{NamePrefix: "page_", Page: page, Limit: size})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Segments {
				ids = append(ids, v.SegmentNo)
			}
			return ids, r.Total, nil
		},
		"campaigns": func(page, size int) ([]string, int, error) {
			r, e := directory.SearchCampaigns(ctx, CampaignManagementSearchInput{NamePrefix: "page_", Page: page, Limit: size})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Campaigns {
				ids = append(ids, v.CampaignNo)
			}
			return ids, r.Total, nil
		},
		"assets": func(page, size int) ([]string, int, error) {
			r, e := assets.SearchAssets(ctx, PromotionAssetSearchInput{Page: page, Limit: size, CustomerNo: "customer-a", Currency: "CNY"})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Assets {
				ids = append(ids, v.AssetNo)
			}
			return ids, r.Total, nil
		},
		"points": func(page, size int) ([]string, int, error) {
			r, e := assets.SearchPointsAccounts(ctx, PointsAccountSearchInput{Page: page, Limit: size})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Accounts {
				ids = append(ids, v.CustomerNo)
			}
			return ids, r.Total, nil
		},
		"grants": func(page, size int) ([]string, int, error) {
			r, e := assets.SearchValueGrants(ctx, ValueGrantSearchInput{Page: page, Limit: size})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Grants {
				ids = append(ids, v.GrantNo)
			}
			return ids, r.Total, nil
		},
		"grant adjustments": func(page, size int) ([]string, int, error) {
			r, e := actions.ListValueGrantAdjustments(ctx, ValueGrantFinancialAdjustmentSearchInput{Page: page, Limit: size, Status: "posted"})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Adjustments {
				ids = append(ids, v.AdjustmentNo)
			}
			return ids, r.Total, nil
		},
		"abuse": func(page, size int) ([]string, int, error) {
			r, e := insights.AbuseCases(ctx, MarketingAbuseSearchInput{Page: page, Limit: size, Status: "pending_review"})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Cases {
				ids = append(ids, v.CaseNo)
			}
			return ids, r.Total, nil
		},
		"performance": func(page, size int) ([]string, int, error) {
			r, e := insights.Performance(ctx, MarketingPerformanceInput{Page: page, Limit: size, StartedAt: time.Now().Add(-time.Hour), EndedAt: time.Now().Add(time.Hour)})
			if e != nil {
				return nil, 0, e
			}
			ids := []string{}
			for _, v := range r.Rows {
				if v.ReceiptMinor != 123 || v.NetMinor != 123 {
					t.Errorf("currency amounts changed: %+v", v)
				}
				ids = append(ids, v.CampaignNo+":"+v.CurrencyCode)
			}
			return ids, r.Total, nil
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			want := 25
			if name == "assets" {
				want = 26
			}
			if name == "performance" {
				want = 50
			}
			assertMarketingNumberedPages(t, want, read)
		})
	}
	for name, read := range map[string]func() (int, error){
		"segments": func() (int, error) {
			r, e := directory.SearchSegments(ctx, SegmentManagementSearchInput{NamePrefix: "missing_%", Page: 2, Limit: 10})
			if e != nil {
				return 0, e
			}
			return r.Total, nil
		},
		"campaigns": func() (int, error) {
			r, e := directory.SearchCampaigns(ctx, CampaignManagementSearchInput{Status: "archived", Page: 2, Limit: 10})
			if e != nil {
				return 0, e
			}
			return r.Total, nil
		},
		"assets": func() (int, error) {
			r, e := assets.SearchAssets(ctx, PromotionAssetSearchInput{CustomerNo: "customer-b", Page: 2, Limit: 10})
			if e != nil {
				return 0, e
			}
			return r.Total, nil
		},
		"points": func() (int, error) {
			r, e := assets.SearchPointsAccounts(ctx, PointsAccountSearchInput{CustomerNo: "customer-b", Page: 2, Limit: 10})
			if e != nil {
				return 0, e
			}
			return r.Total, nil
		},
		"grants": func() (int, error) {
			r, e := assets.SearchValueGrants(ctx, ValueGrantSearchInput{CustomerNo: "customer-b", Page: 2, Limit: 10})
			if e != nil {
				return 0, e
			}
			return r.Total, nil
		},
		"abuse": func() (int, error) {
			r, e := insights.AbuseCases(ctx, MarketingAbuseSearchInput{Status: "false_positive", Page: 2, Limit: 10})
			if e != nil {
				return 0, e
			}
			return r.Total, nil
		},
	} {
		total, e := read()
		if e != nil || total != 0 {
			t.Errorf("empty %s total=%d err=%v", name, total, e)
		}
	}
}

func assertMarketingNumberedPages(t *testing.T, want int, read func(int, int) ([]string, int, error)) {
	t.Helper()
	all, total, err := read(1, 100)
	if err != nil || total != want || len(all) != want {
		t.Fatalf("size100 count=%d rows=%d want=%d err=%v", total, len(all), want, err)
	}
	seen := map[string]bool{}
	for _, id := range all {
		if seen[id] {
			t.Fatalf("duplicate group/item %q", id)
		}
		seen[id] = true
	}
	for _, page := range []int{1, 2, 3, 50} {
		rows, count, e := read(page, 10)
		start := min((page-1)*10, want)
		end := min(page*10, want)
		if e != nil || count != want || !reflect.DeepEqual(rows, all[start:end]) {
			t.Fatalf("page %d rows=%v count=%d want=%v/%d err=%v", page, rows, count, all[start:end], want, e)
		}
	}
}
