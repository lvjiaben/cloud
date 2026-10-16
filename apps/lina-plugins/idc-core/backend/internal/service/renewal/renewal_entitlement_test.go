package renewal

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func TestBuildRenewalBatchStatementUsesOneStatementForManyRows(t *testing.T) {
	for _, rowCount := range []int{1, 128} {
		rows := make([][]any, rowCount)
		for index := range rows {
			rows[index] = []any{index, "renewal", int64(index + 1), true}
		}
		statement, args, err := buildRenewalBatchStatement("INSERT INTO renewal_fixture VALUES ", rows)
		if err != nil {
			t.Fatalf("build %d renewal rows: %v", rowCount, err)
		}
		if strings.Count(statement, "INSERT INTO") != 1 ||
			strings.Count(statement, "(") != rowCount || len(args) != rowCount*4 {
			t.Fatalf("renewal batch is not one set statement: rows=%d sql=%q args=%d", rowCount, statement, len(args))
		}
	}
}

func TestBuildRenewalBatchStatementsUsesFixedBoundedRoundTrips(t *testing.T) {
	for _, test := range []struct {
		rows    int
		batches int
	}{
		{rows: 1, batches: 1},
		{rows: 250, batches: 1},
		{rows: 251, batches: 2},
		{rows: 4_096, batches: 17},
	} {
		rows := make([][]any, test.rows)
		for index := range rows {
			rows[index] = []any{index, "renewal", int64(index + 1), true}
		}
		batches, err := buildRenewalBatchStatements("INSERT INTO renewal_fixture VALUES ", rows)
		if err != nil {
			t.Fatalf("build %d renewal batch rows: %v", test.rows, err)
		}
		if len(batches) != test.batches {
			t.Fatalf("renewal rows=%d batches=%d want=%d", test.rows, len(batches), test.batches)
		}
		writtenRows := 0
		for batchIndex, batch := range batches {
			batchRows := strings.Count(batch.statement, "(")
			if strings.Count(batch.statement, "INSERT INTO") != 1 ||
				batchRows < 1 || batchRows > maximumRenewalBatchRows ||
				len(batch.args) != batchRows*4 {
				t.Fatalf("renewal rows=%d batch=%d sql=%q args=%d", test.rows, batchIndex, batch.statement, len(batch.args))
			}
			writtenRows += batchRows
		}
		if writtenRows != test.rows {
			t.Fatalf("renewal rows=%d written=%d", test.rows, writtenRows)
		}
	}
}

func TestGenericEntitlementRenewalEligibility(t *testing.T) {
	expires := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	base := entitlementQuoteRow{
		EntitlementNo: "ENT_base", CustomerNo: "CUS_1", CustomerStatus: "active",
		EntitlementStatusCode: "active", EntitlementExpiresAt: &expires, Version: 1,
		PeriodUnitCode: "month", PeriodCount: 1, EntitlementOwnerID: "ssl-owner",
		ChargeTypeCode: string(catalog.ChargeTypeRecurring),
		PluginID:       "ssl-provider", PluginReleaseVersion: "1.0.0", ContractVersion: "v1",
		ConfigurationSnapshotNo: "SCF_customer", PriceConfigurationSnapshotNo: "CFG_price",
		DeclarationDigest: strings.Repeat("a", 64),
	}

	t.Run("trial cannot use ordinary paid renewal", func(t *testing.T) {
		trial := base
		trial.RenewalModeCode = "local_entitlement"
		trial.ChargeTypeCode = string(catalog.ChargeTypeTrial)
		if err := validateEntitlementQuoteEligibility(&trial); !bizerr.Is(err, CodeCapabilityUnavailable) {
			t.Fatalf("ordinary trial renewal returned %v", err)
		}
	})

	t.Run("phone one-time delivery is rejected", func(t *testing.T) {
		phone := base
		phone.PeriodUnitCode = "once"
		phone.RenewalModeCode = "unavailable"
		if err := validateEntitlementQuoteEligibility(&phone); !bizerr.Is(err, CodeConflict) {
			t.Fatalf("one-time phone renewal returned %v", err)
		}
	})

	t.Run("SSL local long-term entitlement is accepted", func(t *testing.T) {
		ssl := base
		ssl.RenewalModeCode = "local_entitlement"
		if err := validateEntitlementQuoteEligibility(&ssl); err != nil {
			t.Fatalf("local SSL entitlement rejected: %v", err)
		}
	})

	t.Run("server provider-confirmed entitlement is accepted", func(t *testing.T) {
		server := base
		server.EntitlementOwnerID = "idc-resource"
		server.PluginID = "server-connector"
		server.RenewalModeCode = "provider_confirmed"
		server.RenewalMethod = "renew"
		if err := validateEntitlementQuoteEligibility(&server); err != nil {
			t.Fatalf("provider-confirmed server entitlement rejected: %v", err)
		}
	})

	t.Run("missing frozen business owner fails closed", func(t *testing.T) {
		unknown := base
		unknown.RenewalModeCode = "local_entitlement"
		unknown.EntitlementOwnerID = ""
		if err := validateEntitlementQuoteEligibility(&unknown); !bizerr.Is(err, CodeCapabilityUnavailable) {
			t.Fatalf("missing owner returned %v", err)
		}
	})
}

func TestRenewalPriceComponentKeepsGenericDimensionSeparateFromLegacyOption(t *testing.T) {
	t.Parallel()
	dimensionNo := "cpd_0123456789abcdef0123456789abcdef"
	service := &serviceImpl{random: bytes.NewReader(make([]byte, 64))}
	components, err := service.buildRenewalPriceComponents(
		"RPS_0123456789abcdef0123456789abcdef",
		"ent_0123456789abcdef0123456789abcdef",
		"CFG_0123456789abcdef0123456789abcdef",
		[]persistedQuoteCharge{{
			QuoteChargeNo: "QCH_0123456789abcdef0123456789abcdef",
			Charge: catalog.PurchaseQuoteCharge{
				KindCode: "option", ConfigurationPriceDimensionNo: dimensionNo,
				Quantity: 1, ChargeAmountMinor: 50, PricedAmountMinor: 50,
				PriceSourceDigest: strings.Repeat("a", 64),
			},
		}},
		[]commerce.PricingChargeAmount{{PeriodicAmountMinor: 50}}, nil, 0,
	)
	if err != nil || len(components) != 1 {
		t.Fatalf("build renewal components=%+v err=%v", components, err)
	}
	if components[0].ConfigurationPriceDimension != dimensionNo || components[0].OptionNo != "" ||
		components[0].ConfigurationSnapshotNo != "CFG_0123456789abcdef0123456789abcdef" {
		t.Fatalf("generic dimension leaked into legacy option: %+v", components[0])
	}
}
