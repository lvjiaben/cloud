// This file defines bounded staff-only promotion asset, points, and granted-value reads.

package marketing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	permissionMarketingAssetList      = "idc:admin:marketing:asset:list"
	permissionMarketingAssetQuery     = "idc:admin:marketing:asset:query"
	permissionMarketingPointsList     = "idc:admin:marketing:points:list"
	permissionMarketingPointsQuery    = "idc:admin:marketing:points:query"
	permissionMarketingGrantList      = "idc:admin:marketing:grant:list"
	permissionMarketingGrantQuery     = "idc:admin:marketing:grant:query"
	permissionMarketingAssetIssue     = "idc:admin:marketing:asset:issue"
	permissionMarketingCouponGenerate = "idc:admin:marketing:coupon-code:generate"
	permissionMarketingAssetRevoke    = "idc:admin:marketing:asset:revoke"
	permissionMarketingPointsCredit   = "idc:admin:marketing:points:credit"
	permissionMarketingPointsReverse  = "idc:admin:marketing:points:reverse"
	permissionMarketingGrantPost      = "idc:admin:marketing:grant:post"
	permissionMarketingGrantReverse   = "idc:admin:marketing:grant:reverse"
	permissionMarketingGrantReview    = "idc:admin:marketing:grant:review"

	maximumAssetReservationCount = 100
	maximumPointsLotCount        = 100
	maximumPointsEntryCount      = 200
	maximumValueEntryCount       = 200
)

type AssetDirectoryAuthorizer interface {
	AuthorizeAssetList(context.Context) (int, error)
	AuthorizeAssetQuery(context.Context) (int, error)
	AuthorizePointsList(context.Context) (int, error)
	AuthorizePointsQuery(context.Context) (int, error)
	AuthorizeGrantList(context.Context) (int, error)
	AuthorizeGrantQuery(context.Context) (int, error)
}

type AssetDirectoryReader interface {
	SearchAssets(context.Context, PromotionAssetSearchInput) (*PromotionAssetPage, error)
	GetAsset(context.Context, string) (*PromotionAssetDetail, error)
	SearchPointsAccounts(context.Context, PointsAccountSearchInput) (*PointsAccountPage, error)
	GetPointsAccount(context.Context, string) (*PointsAccountDetail, error)
	SearchValueGrants(context.Context, ValueGrantSearchInput) (*ValueGrantPage, error)
	GetValueGrant(context.Context, string) (*ValueGrantDetail, error)
}

type PromotionAssetSearchInput struct {
	Page       int
	Type       string
	Status     string
	Currency   string
	CustomerNo string
	CampaignNo string
	Cursor     string
	Limit      int
}

type PromotionAssetSummary struct {
	AssetNo         string
	CampaignNo      string
	BenefitNo       string
	BenefitType     string
	Type            string
	CustomerNo      string
	CurrencyCode    string
	FaceAmountMinor *int64
	RateBasisPoints *int
	UsageLimit      int
	ReservedUses    int
	ConsumedUses    int
	Status          string
	ValidFrom       time.Time
	ValidUntil      time.Time
	ClaimedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PromotionAssetReservationSummary struct {
	ReservationNo       string
	ReservedUses        int
	DiscountAmountMinor int64
	Status              string
	ExpiresAt           time.Time
	CreatedAt           time.Time
}

type PromotionAssetPage struct {
	Total      int
	Assets     []PromotionAssetSummary
	NextCursor string
}

type PromotionAssetDetail struct {
	PromotionAssetSummary
	Reservations []PromotionAssetReservationSummary
}

type PointsAccountSearchInput struct {
	Page       int
	CustomerNo string
	Cursor     string
	Limit      int
}

type PointsAccountSummary struct {
	CustomerNo      string
	AvailablePoints int64
	FrozenPoints    int64
	DebtPoints      int64
	Version         int64
	LotCount        int64
	EntryCount      int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PointsLotSummary struct {
	LotNo                   string
	CampaignNo              string
	SourceType              string
	SourceReference         string
	OriginalPoints          int64
	AvailablePoints         int64
	FrozenPoints            int64
	ConsumedPoints          int64
	ExpiredPoints           int64
	ReversedPoints          int64
	ReversalShortfallPoints int64
	ExpiresAt               *time.Time
	Status                  string
	CreatedAt               time.Time
}

type PointsEntrySummary struct {
	EntryNo           string
	LotNo             string
	Type              string
	AvailableDelta    int64
	FrozenDelta       int64
	DebtDelta         int64
	SourceType        string
	SourceReference   string
	ReversalOfEntryNo string
	CreatedAt         time.Time
}

type PointsAccountPage struct {
	Total      int
	Accounts   []PointsAccountSummary
	NextCursor string
}

type PointsAccountDetail struct {
	PointsAccountSummary
	Lots    []PointsLotSummary
	Entries []PointsEntrySummary
}

type ValueGrantSearchInput struct {
	Page       int
	Type       string
	Status     string
	Currency   string
	CustomerNo string
	CampaignNo string
	Cursor     string
	Limit      int
}

type ValueGrantSummary struct {
	GrantNo                   string
	Type                      string
	CampaignNo                string
	BenefitNo                 string
	CustomerNo                string
	CurrencyCode              string
	SourceReference           string
	OriginalAmountMinor       int64
	PendingAmountMinor        int64
	AvailableAmountMinor      int64
	FrozenAmountMinor         int64
	ConsumedAmountMinor       int64
	ExpiredAmountMinor        int64
	ReversedAmountMinor       int64
	Withdrawable              bool
	GlobalQuotaReserved       bool
	CustomerQuotaReserved     bool
	ExpiresAt                 *time.Time
	Status                    string
	FinancialPostingReference string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type ValueEntrySummary struct {
	EntryNo             string
	Type                string
	PendingDeltaMinor   int64
	AvailableDeltaMinor int64
	FrozenDeltaMinor    int64
	ConsumedDeltaMinor  int64
	ExpiredDeltaMinor   int64
	ReversedDeltaMinor  int64
	SourceType          string
	SourceReference     string
	ReversalOfEntryNo   string
	CreatedAt           time.Time
}

type ValueGrantPage struct {
	Total      int
	Grants     []ValueGrantSummary
	NextCursor string
}

type ValueGrantDetail struct {
	ValueGrantSummary
	Entries []ValueEntrySummary
}

type assetDirectoryReaderImpl struct {
	database   gdb.DB
	authorizer AssetDirectoryAuthorizer
	cursor     *marketingManagementCursorCodec
}

var _ AssetDirectoryReader = (*assetDirectoryReaderImpl)(nil)

func NewAssetDirectoryReader(database gdb.DB, authorizer AssetDirectoryAuthorizer, cursorKey []byte) (AssetDirectoryReader, error) {
	if isNilMarketingManagementDependency(database) || isNilMarketingManagementDependency(authorizer) {
		return nil, gerror.New("marketing asset directory requires database and authorization")
	}
	cursor, err := newMarketingManagementCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &assetDirectoryReaderImpl{database: database, authorizer: authorizer, cursor: cursor}, nil
}

type normalizedPromotionAssetSearch struct {
	page                                               int
	typeCode, status, currency, customerNo, campaignNo string
	limit                                              int
	cursor                                             *marketingManagementCursorBoundary
	digest                                             [sha256.Size]byte
}

type normalizedPointsAccountSearch struct {
	page       int
	customerNo string
	limit      int
	cursor     *marketingManagementCursorBoundary
	digest     [sha256.Size]byte
}

type normalizedValueGrantSearch struct {
	page                                               int
	typeCode, status, currency, customerNo, campaignNo string
	limit                                              int
	cursor                                             *marketingManagementCursorBoundary
	digest                                             [sha256.Size]byte
}

func (s *assetDirectoryReaderImpl) SearchAssets(ctx context.Context, input PromotionAssetSearchInput) (*PromotionAssetPage, error) {
	if _, err := s.authorizer.AuthorizeAssetList(ctx); err != nil {
		return nil, err
	}
	normalized, err := normalizePromotionAssetSearch(input, s.cursor)
	if err != nil {
		return nil, err
	}
	page, err := s.searchAssets(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	if err = validatePromotionAssetPage(page); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return page, nil
}

func (s *assetDirectoryReaderImpl) SearchPointsAccounts(ctx context.Context, input PointsAccountSearchInput) (*PointsAccountPage, error) {
	if _, err := s.authorizer.AuthorizePointsList(ctx); err != nil {
		return nil, err
	}
	normalized, err := normalizePointsAccountSearch(input, s.cursor)
	if err != nil {
		return nil, err
	}
	page, err := s.searchPointsAccounts(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	if err = validatePointsAccountPage(page); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return page, nil
}

func (s *assetDirectoryReaderImpl) SearchValueGrants(ctx context.Context, input ValueGrantSearchInput) (*ValueGrantPage, error) {
	if _, err := s.authorizer.AuthorizeGrantList(ctx); err != nil {
		return nil, err
	}
	normalized, err := normalizeValueGrantSearch(input, s.cursor)
	if err != nil {
		return nil, err
	}
	page, err := s.searchValueGrants(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	if err = validateValueGrantPage(page); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return page, nil
}

func normalizePromotionAssetSearch(input PromotionAssetSearchInput, codec *marketingManagementCursorCodec) (normalizedPromotionAssetSearch, error) {
	if !validPromotionAssetType(input.Type) || !validPromotionAssetStatus(input.Status) || !validOptionalCurrency(input.Currency) ||
		!validOptionalIdentifier(input.CustomerNo) || !validOptionalIdentifier(input.CampaignNo) {
		return normalizedPromotionAssetSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	limit, valid := normalizeMarketingPageLimit(input.Limit)
	if !valid || codec == nil || !validMarketingNumberedPage(input.Page, limit, input.Cursor) {
		return normalizedPromotionAssetSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	result := normalizedPromotionAssetSearch{page: input.Page, typeCode: input.Type, status: input.Status, currency: input.Currency, customerNo: input.CustomerNo, campaignNo: input.CampaignNo, limit: limit}
	result.digest = sha256.Sum256([]byte("asset\x00" + input.Type + "\x00" + input.Status + "\x00" + input.Currency + "\x00" + input.CustomerNo + "\x00" + input.CampaignNo + "\x00" + strconv.Itoa(limit)))
	if input.Cursor != "" {
		cursor, err := codec.decode(input.Cursor, result.digest)
		if err != nil {
			return normalizedPromotionAssetSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
		}
		result.cursor = cursor
	}
	return result, nil
}

func normalizePointsAccountSearch(input PointsAccountSearchInput, codec *marketingManagementCursorCodec) (normalizedPointsAccountSearch, error) {
	if !validOptionalIdentifier(input.CustomerNo) {
		return normalizedPointsAccountSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	limit, valid := normalizeMarketingPageLimit(input.Limit)
	if !valid || codec == nil || !validMarketingNumberedPage(input.Page, limit, input.Cursor) {
		return normalizedPointsAccountSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	result := normalizedPointsAccountSearch{page: input.Page, customerNo: input.CustomerNo, limit: limit}
	result.digest = sha256.Sum256([]byte("points\x00" + input.CustomerNo + "\x00" + strconv.Itoa(limit)))
	if input.Cursor != "" {
		cursor, err := codec.decode(input.Cursor, result.digest)
		if err != nil {
			return normalizedPointsAccountSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
		}
		result.cursor = cursor
	}
	return result, nil
}

func normalizeValueGrantSearch(input ValueGrantSearchInput, codec *marketingManagementCursorCodec) (normalizedValueGrantSearch, error) {
	if !validValueGrantType(input.Type) || !validValueGrantStatus(input.Status) || !validOptionalCurrency(input.Currency) ||
		!validOptionalIdentifier(input.CustomerNo) || !validOptionalIdentifier(input.CampaignNo) {
		return normalizedValueGrantSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	limit, valid := normalizeMarketingPageLimit(input.Limit)
	if !valid || codec == nil || !validMarketingNumberedPage(input.Page, limit, input.Cursor) {
		return normalizedValueGrantSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	result := normalizedValueGrantSearch{page: input.Page, typeCode: input.Type, status: input.Status, currency: input.Currency, customerNo: input.CustomerNo, campaignNo: input.CampaignNo, limit: limit}
	result.digest = sha256.Sum256([]byte("grant\x00" + input.Type + "\x00" + input.Status + "\x00" + input.Currency + "\x00" + input.CustomerNo + "\x00" + input.CampaignNo + "\x00" + strconv.Itoa(limit)))
	if input.Cursor != "" {
		cursor, err := codec.decode(input.Cursor, result.digest)
		if err != nil {
			return normalizedValueGrantSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
		}
		result.cursor = cursor
	}
	return result, nil
}

func validOptionalIdentifier(value string) bool {
	return value == "" || identifierPattern.MatchString(value)
}
func validOptionalCurrency(value string) bool {
	return value == "" || currencyPattern.MatchString(value)
}
func validPromotionAssetType(value string) bool {
	return value == "" || value == "coupon" || value == "voucher"
}
func validPromotionAssetStatus(value string) bool {
	return value == "" || value == "issued" || value == "active" || value == "exhausted" || value == "expired" || value == "revoked"
}
func validValueGrantType(value string) bool {
	return value == "" || value == "recharge_bonus" || value == "cashback"
}
func validValueGrantStatus(value string) bool {
	return value == "" || value == "pending" || value == "available" || value == "exhausted" || value == "expired" || value == "reversed" || value == "cancelled"
}

func validatePromotionAssetPage(page *PromotionAssetPage) error {
	if page == nil || page.Assets == nil || len(page.Assets) > maximumMarketingManagementPageSize || len(page.NextCursor) > maximumMarketingManagementCursorLength {
		return gerror.New("marketing promotion asset page is invalid")
	}
	for _, item := range page.Assets {
		if !validPromotionAssetSummary(item) {
			return gerror.New("marketing promotion asset projection is inconsistent")
		}
	}
	return validateAssetDirectoryPayload(page)
}

func validatePointsAccountPage(page *PointsAccountPage) error {
	if page == nil || page.Accounts == nil || len(page.Accounts) > maximumMarketingManagementPageSize || len(page.NextCursor) > maximumMarketingManagementCursorLength {
		return gerror.New("marketing points account page is invalid")
	}
	for _, item := range page.Accounts {
		if !validPointsAccountSummary(item) {
			return gerror.New("marketing points account projection is inconsistent")
		}
	}
	return validateAssetDirectoryPayload(page)
}

func validateValueGrantPage(page *ValueGrantPage) error {
	if page == nil || page.Grants == nil || len(page.Grants) > maximumMarketingManagementPageSize || len(page.NextCursor) > maximumMarketingManagementCursorLength {
		return gerror.New("marketing value grant page is invalid")
	}
	for _, item := range page.Grants {
		if !validValueGrantSummary(item) {
			return gerror.New("marketing value grant projection is inconsistent")
		}
	}
	return validateAssetDirectoryPayload(page)
}

func validPromotionAssetSummary(item PromotionAssetSummary) bool {
	return identifierPattern.MatchString(item.AssetNo) && identifierPattern.MatchString(item.CampaignNo) && identifierPattern.MatchString(item.BenefitNo) &&
		validPromotionAssetType(item.Type) && item.Type != "" && validPromotionAssetStatus(item.Status) && item.Status != "" &&
		validOptionalIdentifier(item.CustomerNo) && currencyPattern.MatchString(item.CurrencyCode) && validPromotionAssetValue(item) &&
		item.UsageLimit >= 1 && item.ReservedUses >= 0 && item.ConsumedUses >= 0 && item.ReservedUses+item.ConsumedUses <= item.UsageLimit &&
		item.ValidUntil.After(item.ValidFrom) && !item.CreatedAt.IsZero() && !item.UpdatedAt.Before(item.CreatedAt) && validPromotionAssetLifecycle(item)
}

func validPromotionAssetValue(item PromotionAssetSummary) bool {
	switch item.BenefitType {
	case "voucher":
		return item.Type == "voucher" && item.FaceAmountMinor != nil && *item.FaceAmountMinor > 0 && item.RateBasisPoints == nil
	case "fixed_discount":
		return item.Type == "coupon" && item.FaceAmountMinor != nil && *item.FaceAmountMinor > 0 && item.RateBasisPoints == nil
	case "percentage_discount":
		return item.Type == "coupon" && item.FaceAmountMinor == nil && item.RateBasisPoints != nil && *item.RateBasisPoints >= 1 && *item.RateBasisPoints <= 10000
	default:
		return false
	}
}

func validPromotionAssetLifecycle(item PromotionAssetSummary) bool {
	claimed := item.CustomerNo != "" && item.ClaimedAt != nil && !item.ClaimedAt.Before(item.CreatedAt)
	unclaimed := item.CustomerNo == "" && item.ClaimedAt == nil
	switch item.Status {
	case "issued":
		return unclaimed && item.ReservedUses == 0 && item.ConsumedUses == 0
	case "active":
		return claimed && (item.ReservedUses > 0 || item.ConsumedUses < item.UsageLimit)
	case "exhausted":
		return claimed && item.ReservedUses == 0 && item.ConsumedUses == item.UsageLimit
	case "expired", "revoked":
		return item.ReservedUses == 0 && (claimed || unclaimed)
	default:
		return false
	}
}

func validPointsAccountSummary(item PointsAccountSummary) bool {
	return identifierPattern.MatchString(item.CustomerNo) && item.AvailablePoints >= 0 && item.FrozenPoints >= 0 && item.DebtPoints >= 0 &&
		item.Version > 0 && item.LotCount >= 0 && item.EntryCount >= 0 && !item.CreatedAt.IsZero() && !item.UpdatedAt.Before(item.CreatedAt)
}

func validValueGrantSummary(item ValueGrantSummary) bool {
	return identifierPattern.MatchString(item.GrantNo) && identifierPattern.MatchString(item.CampaignNo) && identifierPattern.MatchString(item.BenefitNo) && identifierPattern.MatchString(item.CustomerNo) &&
		validValueGrantType(item.Type) && item.Type != "" && validValueGrantStatus(item.Status) && item.Status != "" && currencyPattern.MatchString(item.CurrencyCode) &&
		validBusinessReference(item.SourceReference) &&
		item.OriginalAmountMinor > 0 && nonNegativeMarketingOverviewValues(item.PendingAmountMinor, item.AvailableAmountMinor, item.FrozenAmountMinor, item.ConsumedAmountMinor, item.ExpiredAmountMinor, item.ReversedAmountMinor) &&
		item.PendingAmountMinor+item.AvailableAmountMinor+item.FrozenAmountMinor+item.ConsumedAmountMinor+item.ExpiredAmountMinor+item.ReversedAmountMinor == item.OriginalAmountMinor &&
		(item.Type != "recharge_bonus" || !item.Withdrawable) && !item.CreatedAt.IsZero() && !item.UpdatedAt.Before(item.CreatedAt) && validValueGrantLifecycle(item)
}

func validValueGrantLifecycle(item ValueGrantSummary) bool {
	switch item.Status {
	case "pending":
		return item.PendingAmountMinor == item.OriginalAmountMinor && item.FinancialPostingReference == ""
	case "available":
		return validBusinessReference(item.FinancialPostingReference) && item.PendingAmountMinor == 0 && item.AvailableAmountMinor+item.FrozenAmountMinor > 0
	case "exhausted":
		return validBusinessReference(item.FinancialPostingReference) && item.PendingAmountMinor == 0 && item.AvailableAmountMinor == 0 && item.FrozenAmountMinor == 0 && item.ConsumedAmountMinor > 0
	case "expired":
		return validBusinessReference(item.FinancialPostingReference) && item.PendingAmountMinor == 0 && item.AvailableAmountMinor == 0 && item.FrozenAmountMinor == 0 && item.ExpiredAmountMinor > 0
	case "reversed":
		return validBusinessReference(item.FinancialPostingReference) && item.PendingAmountMinor == 0 && item.AvailableAmountMinor == 0 && item.FrozenAmountMinor == 0 && item.ReversedAmountMinor > 0
	case "cancelled":
		return item.FinancialPostingReference == "" && item.PendingAmountMinor == 0 && item.AvailableAmountMinor == 0 && item.FrozenAmountMinor == 0 && item.ConsumedAmountMinor == 0 && item.ExpiredAmountMinor == 0 && item.ReversedAmountMinor == item.OriginalAmountMinor
	default:
		return false
	}
}

func validateAssetDirectoryPayload(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return gerror.Wrap(err, "measure marketing asset directory response")
	}
	if len(payload) > maximumMarketingManagementPageBytes {
		return gerror.New("marketing asset directory response exceeds byte limit")
	}
	return nil
}
