// This file verifies marketing directory cursor, validation, and authorization ordering.

package marketing

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

type marketingDirectoryAuthorizerStub struct {
	segmentListCalls   int
	segmentQueryCalls  int
	campaignListCalls  int
	campaignQueryCalls int
	err                error
}

func (s *marketingDirectoryAuthorizerStub) AuthorizeSegmentList(context.Context) (int, error) {
	s.segmentListCalls++
	return 7, s.err
}
func (s *marketingDirectoryAuthorizerStub) AuthorizeSegmentQuery(context.Context) (int, error) {
	s.segmentQueryCalls++
	return 7, s.err
}
func (s *marketingDirectoryAuthorizerStub) AuthorizeCampaignList(context.Context) (int, error) {
	s.campaignListCalls++
	return 7, s.err
}
func (s *marketingDirectoryAuthorizerStub) AuthorizeCampaignQuery(context.Context) (int, error) {
	s.campaignQueryCalls++
	return 7, s.err
}

func TestMarketingManagementCursorIsOpaqueTamperEvidentAndFilterBound(t *testing.T) {
	t.Parallel()
	codec, err := newMarketingManagementCursorCodec(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatalf("create cursor codec: %v", err)
	}
	normalized, err := normalizeSegmentManagementSearch(SegmentManagementSearchInput{NamePrefix: "loyal", Status: "active", Limit: 25}, codec)
	if err != nil {
		t.Fatalf("normalize segment search: %v", err)
	}
	encoded, err := codec.encode(marketingManagementCursorBoundary{UpdatedAtMicroseconds: 1767225600123456, ID: 987654321}, normalized.digest)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	decoded, err := codec.decode(encoded, normalized.digest)
	if err != nil || decoded.ID != 987654321 || decoded.UpdatedAtMicroseconds != 1767225600123456 {
		t.Fatalf("decode cursor=%+v err=%v", decoded, err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode cursor transport: %v", err)
	}
	if strings.Contains(string(payload), "987654321") || strings.Contains(string(payload), "loyal") {
		t.Fatal("cursor exposed database or filter values")
	}
	payload[len(payload)-1] ^= 1
	if _, err = codec.decode(base64.RawURLEncoding.EncodeToString(payload), normalized.digest); err == nil {
		t.Fatal("tampered cursor was accepted")
	}
	changed, err := normalizeSegmentManagementSearch(SegmentManagementSearchInput{NamePrefix: "loyal", Status: "paused", Limit: 25}, codec)
	if err != nil {
		t.Fatalf("normalize changed search: %v", err)
	}
	if _, err = codec.decode(encoded, changed.digest); err == nil {
		t.Fatal("cursor was reusable across filters")
	}
	campaign, err := normalizeCampaignManagementSearch(CampaignManagementSearchInput{NamePrefix: "loyal", Status: "active", Limit: 25}, codec)
	if err != nil {
		t.Fatalf("normalize campaign search: %v", err)
	}
	if _, err = codec.decode(encoded, campaign.digest); err == nil {
		t.Fatal("segment cursor was reusable for campaigns")
	}
}

func TestMarketingManagementDirectoryAuthorizesBeforeValidation(t *testing.T) {
	t.Parallel()
	database := newMarketingDirectoryTestDatabase(t)
	authorizer := &marketingDirectoryAuthorizerStub{err: bizerr.NewCode(CodeManagementDirectoryNotAccessible)}
	reader, err := NewManagementDirectoryReader(database, authorizer, bytes.Repeat([]byte{0x21}, 32))
	if err != nil {
		t.Fatalf("create directory reader: %v", err)
	}
	if _, err = reader.SearchSegments(context.Background(), SegmentManagementSearchInput{Limit: 1000}); err == nil || authorizer.segmentListCalls != 1 {
		t.Fatalf("segment authorization ordering calls=%d err=%v", authorizer.segmentListCalls, err)
	}
	if _, err = reader.GetCampaign(context.Background(), "invalid value"); err == nil || authorizer.campaignQueryCalls != 1 {
		t.Fatalf("campaign authorization ordering calls=%d err=%v", authorizer.campaignQueryCalls, err)
	}
}

func TestNormalizeMarketingManagementSearchRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	database := newMarketingDirectoryTestDatabase(t)
	codec, err := newMarketingManagementCursorCodec(bytes.Repeat([]byte{0x11}, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []SegmentManagementSearchInput{{Limit: -1}, {Limit: 101}, {Mode: "computed"}, {Status: "deleted"}, {Cursor: "broken"}, {NamePrefix: strings.Repeat("界", 129)}} {
		if _, normalizeErr := normalizeSegmentManagementSearch(input, codec); normalizeErr == nil {
			t.Fatalf("accepted invalid segment input: %+v", input)
		}
	}
	for _, input := range []CampaignManagementSearchInput{{Limit: -1}, {Type: "gift"}, {Status: "deleted"}, {TargetSegmentNo: "bad value"}, {Cursor: "broken"}} {
		if _, normalizeErr := normalizeCampaignManagementSearch(input, codec); normalizeErr == nil {
			t.Fatalf("accepted invalid campaign input: %+v", input)
		}
	}
	if _, err = NewManagementDirectoryReader(nil, &marketingDirectoryAuthorizerStub{}, bytes.Repeat([]byte{1}, 32)); err == nil {
		t.Fatal("nil database was accepted")
	}
	if _, err = NewManagementDirectoryReader(database, nil, bytes.Repeat([]byte{1}, 32)); err == nil {
		t.Fatal("nil authorizer was accepted")
	}
	if _, err = NewManagementDirectoryReader(database, &marketingDirectoryAuthorizerStub{}, []byte("short")); err == nil {
		t.Fatal("short cursor key was accepted")
	}
}

func TestMarketingManagementProjectionValidationRejectsInconsistentTypedValues(t *testing.T) {
	t.Parallel()
	lower, upper := int64(100), int64(200)
	condition := SegmentCondition{
		ConditionNo:       "cond-1",
		OrGroupNo:         1,
		Fact:              "paid_amount",
		Operator:          "between",
		ValueKind:         "currency_amount",
		ValueIntegerLower: &lower,
		ValueIntegerUpper: &upper,
		CurrencyCode:      "CNY",
		Values:            []SegmentConditionValue{},
	}
	if !validSegmentCondition(condition) {
		t.Fatal("valid typed currency condition was rejected")
	}
	condition.CurrencyCode = ""
	if validSegmentCondition(condition) {
		t.Fatal("currency condition without currency was accepted")
	}

	amount := int64(500)
	benefit := CampaignBenefit{
		BenefitNo:     "benefit-1",
		Type:          "voucher",
		CurrencyCode:  "CNY",
		AmountMinor:   &amount,
		PurchasePhase: "both",
		CycleScope:    "first",
		Status:        "active",
		Version:       1,
	}
	if !validCampaignBenefit(benefit) {
		t.Fatal("valid fixed amount benefit was rejected")
	}
	points := int64(50)
	benefit.PointsQuantity = &points
	if validCampaignBenefit(benefit) {
		t.Fatal("money benefit carrying points was accepted")
	}

	now := time.Now().UTC()
	globalLimit := int64(10)
	perCustomerLimit := 1
	campaign := CampaignManagementSummary{
		CampaignNo:            "campaign-1",
		CampaignCode:          "campaign.code",
		NameZhCN:              "活动",
		NameEnUS:              "Campaign",
		Type:                  "voucher",
		Status:                "active",
		Priority:              1,
		StackPolicy:           "exclusive",
		GlobalQuotaLimit:      &globalLimit,
		PerCustomerQuotaLimit: &perCustomerLimit,
		StartsAt:              now.Add(-time.Hour),
		EndsAt:                now.Add(time.Hour),
		CreatedAt:             now.Add(-2 * time.Hour),
		UpdatedAt:             now,
	}
	if !validCampaignSummary(campaign) {
		t.Fatal("valid campaign summary was rejected")
	}
	campaign.GlobalQuotaLimit = nil
	campaign.GlobalReservedCount = 1
	if validCampaignSummary(campaign) {
		t.Fatal("unlimited campaign carrying reserved quota was accepted")
	}
}

func newMarketingDirectoryTestDatabase(t *testing.T) gdb.DB {
	t.Helper()
	database, err := gdb.New(gdb.ConfigNode{Type: "pgsql", Link: "pgsql:test:test@tcp(127.0.0.1:1)/test?sslmode=disable"})
	if err != nil {
		t.Fatalf("create disconnected test database: %v", err)
	}
	return database
}
