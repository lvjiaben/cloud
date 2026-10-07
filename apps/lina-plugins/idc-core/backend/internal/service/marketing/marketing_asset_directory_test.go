// This file verifies asset-directory authorization, cursor binding, and typed projection validation.

package marketing

import (
	"bytes"
	"context"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
)

type assetDirectoryAuthorizerStub struct {
	assetListCalls, assetQueryCalls, pointsListCalls, pointsQueryCalls, grantListCalls, grantQueryCalls int
	err                                                                                                 error
}

func (s *assetDirectoryAuthorizerStub) AuthorizeAssetList(context.Context) (int, error) {
	s.assetListCalls++
	return 7, s.err
}
func (s *assetDirectoryAuthorizerStub) AuthorizeAssetQuery(context.Context) (int, error) {
	s.assetQueryCalls++
	return 7, s.err
}
func (s *assetDirectoryAuthorizerStub) AuthorizePointsList(context.Context) (int, error) {
	s.pointsListCalls++
	return 7, s.err
}
func (s *assetDirectoryAuthorizerStub) AuthorizePointsQuery(context.Context) (int, error) {
	s.pointsQueryCalls++
	return 7, s.err
}
func (s *assetDirectoryAuthorizerStub) AuthorizeGrantList(context.Context) (int, error) {
	s.grantListCalls++
	return 7, s.err
}
func (s *assetDirectoryAuthorizerStub) AuthorizeGrantQuery(context.Context) (int, error) {
	s.grantQueryCalls++
	return 7, s.err
}

func TestMarketingAssetDirectoryAuthorizesBeforeInputValidation(t *testing.T) {
	t.Parallel()
	authorizer := &assetDirectoryAuthorizerStub{err: bizerr.NewCode(CodeManagementDirectoryNotAccessible)}
	reader, err := NewAssetDirectoryReader(newMarketingDirectoryTestDatabase(t), authorizer, bytes.Repeat([]byte{0x71}, 32))
	if err != nil {
		t.Fatalf("create asset directory: %v", err)
	}
	if _, err = reader.SearchAssets(context.Background(), PromotionAssetSearchInput{Limit: 1000}); err == nil || authorizer.assetListCalls != 1 {
		t.Fatalf("asset authorization ordering calls=%d err=%v", authorizer.assetListCalls, err)
	}
	if _, err = reader.GetPointsAccount(context.Background(), "invalid value"); err == nil || authorizer.pointsQueryCalls != 1 {
		t.Fatalf("points authorization ordering calls=%d err=%v", authorizer.pointsQueryCalls, err)
	}
	if _, err = reader.GetValueGrant(context.Background(), "invalid value"); err == nil || authorizer.grantQueryCalls != 1 {
		t.Fatalf("grant authorization ordering calls=%d err=%v", authorizer.grantQueryCalls, err)
	}
}

func TestMarketingAssetDirectoryCursorIsBoundToResourceAndFilters(t *testing.T) {
	t.Parallel()
	codec, err := newMarketingManagementCursorCodec(bytes.Repeat([]byte{0x72}, 32))
	if err != nil {
		t.Fatal(err)
	}
	asset, err := normalizePromotionAssetSearch(PromotionAssetSearchInput{Type: "coupon", Currency: "CNY", Limit: 25}, codec)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := codec.encode(marketingManagementCursorBoundary{UpdatedAtMicroseconds: time.Now().UnixMicro(), ID: 44}, asset.digest)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := normalizePromotionAssetSearch(PromotionAssetSearchInput{Type: "coupon", Currency: "USD", Limit: 25}, codec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = codec.decode(cursor, changed.digest); err == nil {
		t.Fatal("asset cursor crossed currency filter")
	}
	points, err := normalizePointsAccountSearch(PointsAccountSearchInput{Limit: 25}, codec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = codec.decode(cursor, points.digest); err == nil {
		t.Fatal("asset cursor crossed into points accounts")
	}
}

func TestNormalizeMarketingAssetDirectoryRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	codec, err := newMarketingManagementCursorCodec(bytes.Repeat([]byte{0x73}, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []PromotionAssetSearchInput{{Type: "gift"}, {Status: "deleted"}, {Currency: "rmb"}, {CustomerNo: "bad value"}, {Limit: 101}, {Cursor: "broken"}} {
		if _, normalizeErr := normalizePromotionAssetSearch(input, codec); normalizeErr == nil {
			t.Fatalf("accepted invalid asset input: %+v", input)
		}
	}
	for _, input := range []PointsAccountSearchInput{{CustomerNo: "bad value"}, {Limit: -1}, {Cursor: "broken"}} {
		if _, normalizeErr := normalizePointsAccountSearch(input, codec); normalizeErr == nil {
			t.Fatalf("accepted invalid points input: %+v", input)
		}
	}
	for _, input := range []ValueGrantSearchInput{{Type: "gift"}, {Status: "deleted"}, {Currency: "CN"}, {CampaignNo: "bad value"}, {Limit: 101}, {Cursor: "broken"}} {
		if _, normalizeErr := normalizeValueGrantSearch(input, codec); normalizeErr == nil {
			t.Fatalf("accepted invalid grant input: %+v", input)
		}
	}
	if _, err = NewAssetDirectoryReader(nil, &assetDirectoryAuthorizerStub{}, bytes.Repeat([]byte{1}, 32)); err == nil {
		t.Fatal("nil database was accepted")
	}
	if _, err = NewAssetDirectoryReader(newMarketingDirectoryTestDatabase(t), nil, bytes.Repeat([]byte{1}, 32)); err == nil {
		t.Fatal("nil authorizer was accepted")
	}
	var typedNilAuthorizer *assetDirectoryAuthorizerStub
	if _, err = NewAssetDirectoryReader(newMarketingDirectoryTestDatabase(t), typedNilAuthorizer, bytes.Repeat([]byte{1}, 32)); err == nil {
		t.Fatal("typed nil authorizer was accepted")
	}
}

func TestMarketingAssetDirectoryProjectionKeepsMoneyAndPointsDistinct(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	face := int64(500)
	asset := PromotionAssetSummary{
		AssetNo: "asset-1", CampaignNo: "campaign-1", BenefitNo: "benefit-1", BenefitType: "voucher", Type: "voucher",
		CurrencyCode: "CNY", FaceAmountMinor: &face, UsageLimit: 1, Status: "issued",
		ValidFrom: now, ValidUntil: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	if !validPromotionAssetSummary(asset) {
		t.Fatal("valid monetary asset was rejected")
	}
	asset.BenefitType = "percentage_discount"
	if validPromotionAssetSummary(asset) {
		t.Fatal("voucher asset with percentage benefit was accepted")
	}
	asset.BenefitType = "voucher"
	asset.CustomerNo = "customer-1"
	if validPromotionAssetSummary(asset) {
		t.Fatal("unclaimed issued asset with an owner was accepted")
	}
	asset.CustomerNo = ""
	asset.CurrencyCode = ""
	if validPromotionAssetSummary(asset) {
		t.Fatal("monetary asset without explicit currency was accepted")
	}
	account := PointsAccountSummary{CustomerNo: "customer-1", Version: 1, CreatedAt: now, UpdatedAt: now}
	if !validPointsAccountSummary(account) {
		t.Fatal("valid non-monetary points account was rejected")
	}
	grant := ValueGrantSummary{
		GrantNo: "grant-1", Type: "recharge_bonus", CampaignNo: "campaign-1", BenefitNo: "benefit-1", CustomerNo: "customer-1",
		CurrencyCode: "CNY", SourceReference: "payment-1", OriginalAmountMinor: 500, PendingAmountMinor: 500, Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}
	if !validValueGrantSummary(grant) {
		t.Fatal("valid non-withdrawable recharge bonus was rejected")
	}
	grant.Withdrawable = true
	if validValueGrantSummary(grant) {
		t.Fatal("withdrawable recharge bonus was accepted")
	}
	grant.Withdrawable = false
	grant.Status = "available"
	grant.PendingAmountMinor = 0
	grant.AvailableAmountMinor = grant.OriginalAmountMinor
	if validValueGrantSummary(grant) {
		t.Fatal("posted grant without a financial reference was accepted")
	}
}

func TestMarketingAssetDirectoryRejectsMalformedLedgerRows(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	lot := PointsLotSummary{
		LotNo: "lot-1", SourceType: "order", SourceReference: "order-1", OriginalPoints: 100,
		AvailablePoints: 100, Status: "available", CreatedAt: now,
	}
	if !validPointsLot(lot) {
		t.Fatal("valid points lot was rejected")
	}
	lot.SourceType = "unknown"
	if validPointsLot(lot) {
		t.Fatal("points lot with unknown source was accepted")
	}
	pointsEntry := PointsEntrySummary{
		EntryNo: "points-entry-1", LotNo: "lot-1", Type: "reverse", AvailableDelta: -1,
		SourceType: "refund", SourceReference: "refund-1", CreatedAt: now,
	}
	if validPointsEntry(pointsEntry) {
		t.Fatal("reverse points entry without an original entry was accepted")
	}
	valueEntry := ValueEntrySummary{
		EntryNo: "value-entry-1", Type: "unknown", AvailableDeltaMinor: 1,
		SourceType: "order", SourceReference: "order-1", CreatedAt: now,
	}
	if validValueEntry(valueEntry) {
		t.Fatal("granted-value entry with an unknown type was accepted")
	}
}
