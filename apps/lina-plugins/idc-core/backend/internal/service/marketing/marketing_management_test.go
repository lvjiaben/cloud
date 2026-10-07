// This file verifies marketing overview dependency and projection validation boundaries.

package marketing

import (
	"testing"
	"time"
)

type typedNilMarketingAuthorizerStub struct{}

// TestMarketingManagementDependencyValidationRejectsTypedNil prevents interface-typed nil panics.
func TestMarketingManagementDependencyValidationRejectsTypedNil(t *testing.T) {
	t.Parallel()
	var authorizer *typedNilMarketingAuthorizerStub
	if !isNilMarketingManagementDependency(authorizer) {
		t.Fatal("typed nil overview dependency must be rejected")
	}
}

// TestValidateManagementOverviewRejectsInvalidBusinessValues covers defensive response validation.
func TestValidateManagementOverviewRejectsInvalidBusinessValues(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	valid := func() *ManagementOverview {
		return &ManagementOverview{
			GeneratedAt: now,
			Currencies:  []MarketingCurrencySummary{{CurrencyCode: "CNY", GrantCount: 1}},
			RecentCampaigns: []MarketingCampaignSummary{{
				CampaignNo: "campaign-1", StartsAt: now, EndsAt: now.Add(time.Hour), UpdatedAt: now,
			}},
			RecentGrants: []MarketingGrantSummary{{
				GrantNo: "grant-1", CurrencyCode: "CNY", CreatedAt: now, UpdatedAt: now,
			}},
		}
	}

	negativeSummary := valid()
	negativeSummary.Summary.PointsDebt = -1
	if err := validateManagementOverview(negativeSummary); err == nil {
		t.Fatal("negative summary value must be rejected")
	}
	negativeCurrency := valid()
	negativeCurrency.Currencies[0].AvailableMinor = -1
	if err := validateManagementOverview(negativeCurrency); err == nil {
		t.Fatal("negative currency value must be rejected")
	}
	invalidCampaign := valid()
	invalidCampaign.RecentCampaigns[0].EndsAt = now.Add(-time.Hour)
	if err := validateManagementOverview(invalidCampaign); err == nil {
		t.Fatal("campaign with reversed time window must be rejected")
	}
	negativeGrant := valid()
	negativeGrant.RecentGrants[0].OriginalMinor = -1
	if err := validateManagementOverview(negativeGrant); err == nil {
		t.Fatal("negative grant amount must be rejected")
	}
	if err := validateManagementOverview(valid()); err != nil {
		t.Fatalf("valid overview rejected: %v", err)
	}
}
