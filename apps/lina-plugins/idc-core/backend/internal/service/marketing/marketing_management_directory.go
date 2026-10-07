// This file defines bounded staff-only segment and campaign management reads.

package marketing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	permissionMarketingSegmentList   = "idc:admin:marketing:segment:list"
	permissionMarketingSegmentQuery  = "idc:admin:marketing:segment:query"
	permissionMarketingCampaignList  = "idc:admin:marketing:campaign:list"
	permissionMarketingCampaignQuery = "idc:admin:marketing:campaign:query"

	defaultMarketingManagementPageSize  = 25
	maximumMarketingManagementPageSize  = 100
	maximumMarketingManagementPageBytes = 1024 * 1024
	maximumSegmentConditionCount        = 256
	maximumSegmentConditionValueCount   = 4096
	maximumSegmentSnapshotCount         = 20
	maximumCampaignScopeCount           = 512
	maximumCampaignBenefitCount         = 512
)

// ManagementDirectoryAuthorizer authorizes each directory read independently.
type ManagementDirectoryAuthorizer interface {
	AuthorizeSegmentList(ctx context.Context) (int, error)
	AuthorizeSegmentQuery(ctx context.Context) (int, error)
	AuthorizeCampaignList(ctx context.Context) (int, error)
	AuthorizeCampaignQuery(ctx context.Context) (int, error)
}

// ManagementDirectoryReader provides bounded segment and campaign queues and details.
type ManagementDirectoryReader interface {
	SearchSegments(context.Context, SegmentManagementSearchInput) (*SegmentManagementPage, error)
	GetSegment(context.Context, string) (*SegmentManagementDetail, error)
	SearchCampaigns(context.Context, CampaignManagementSearchInput) (*CampaignManagementPage, error)
	GetCampaign(context.Context, string) (*CampaignManagementDetail, error)
}

type SegmentManagementSearchInput struct {
	Page       int
	NamePrefix string
	Mode       string
	Status     string
	Cursor     string
	Limit      int
}

type CampaignManagementSearchInput struct {
	Page            int
	NamePrefix      string
	Type            string
	Status          string
	TargetSegmentNo string
	Cursor          string
	Limit           int
}

type SegmentManagementSummary struct {
	SegmentNo              string
	SegmentCode            string
	NameZhCN               string
	NameEnUS               string
	Mode                   string
	RefreshIntervalMinutes *int
	Status                 string
	Version                int64
	ConditionCount         int64
	ActiveMemberCount      int64
	LatestSnapshotNo       string
	LatestSnapshotStatus   string
	LatestSnapshotMembers  int64
	LatestSnapshotAt       *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type SegmentManagementPage struct {
	Total      int
	Segments   []SegmentManagementSummary
	NextCursor string
}

type SegmentCondition struct {
	ConditionNo       string
	OrGroupNo         int
	SortOrder         int
	Fact              string
	Operator          string
	ValueKind         string
	ValueText         string
	ValueIntegerLower *int64
	ValueIntegerUpper *int64
	ValueBoolean      *bool
	CurrencyCode      string
	Values            []SegmentConditionValue
}

type SegmentConditionValue struct {
	Text    string
	Integer *int64
}

type SegmentSnapshotSummary struct {
	SnapshotNo     string
	SegmentVersion int64
	Status         string
	MemberCount    int64
	StartedAt      time.Time
	CompletedAt    *time.Time
}

type SegmentManagementDetail struct {
	SegmentManagementSummary
	DescriptionZhCN string
	DescriptionEnUS string
	Conditions      []SegmentCondition
	Snapshots       []SegmentSnapshotSummary
}

type CampaignManagementSummary struct {
	CampaignNo            string
	CampaignCode          string
	NameZhCN              string
	NameEnUS              string
	Type                  string
	Status                string
	TargetSegmentNo       string
	TargetSnapshotNo      string
	Priority              int
	StackPolicy           string
	GlobalQuotaLimit      *int64
	GlobalReservedCount   int64
	GlobalConsumedCount   int64
	PerCustomerQuotaLimit *int
	IsNewcomerExclusive   bool
	ScopeCount            int64
	BenefitCount          int64
	ReservationCount      int64
	StartsAt              time.Time
	EndsAt                time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CampaignManagementPage struct {
	Total      int
	Campaigns  []CampaignManagementSummary
	NextCursor string
}

type CampaignScope struct {
	Kind          string
	MatchMode     string
	ReferenceCode string
}

type CampaignBenefit struct {
	BenefitNo            string
	Type                 string
	CurrencyCode         string
	AmountMinor          *int64
	RateBasisPoints      *int
	MaximumDiscountMinor *int64
	MinimumSpendMinor    *int64
	PointsQuantity       *int64
	PurchasePhase        string
	CycleScope           string
	CycleCount           *int
	Status               string
	Version              int64
}

type CampaignManagementDetail struct {
	CampaignManagementSummary
	DescriptionZhCN string
	DescriptionEnUS string
	StackGroup      string
	Version         int64
	Scopes          []CampaignScope
	Benefits        []CampaignBenefit
}

type managementDirectoryReaderImpl struct {
	database   gdb.DB
	authorizer ManagementDirectoryAuthorizer
	cursor     *marketingManagementCursorCodec
}

var _ ManagementDirectoryReader = (*managementDirectoryReaderImpl)(nil)

func NewManagementDirectoryReader(
	database gdb.DB,
	authorizer ManagementDirectoryAuthorizer,
	cursorKey []byte,
) (ManagementDirectoryReader, error) {
	if isNilMarketingManagementDependency(database) || isNilMarketingManagementDependency(authorizer) {
		return nil, gerror.New("marketing management directory requires database and authorization")
	}
	cursor, err := newMarketingManagementCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &managementDirectoryReaderImpl{database: database, authorizer: authorizer, cursor: cursor}, nil
}

type normalizedSegmentManagementSearch struct {
	page       int
	namePrefix string
	likePrefix string
	mode       string
	status     string
	limit      int
	cursor     *marketingManagementCursorBoundary
	digest     [sha256.Size]byte
}

type normalizedCampaignManagementSearch struct {
	page            int
	namePrefix      string
	likePrefix      string
	typeCode        string
	status          string
	targetSegmentNo string
	limit           int
	cursor          *marketingManagementCursorBoundary
	digest          [sha256.Size]byte
}

func (s *managementDirectoryReaderImpl) SearchSegments(ctx context.Context, input SegmentManagementSearchInput) (*SegmentManagementPage, error) {
	if _, err := s.authorizer.AuthorizeSegmentList(ctx); err != nil {
		return nil, err
	}
	normalized, err := normalizeSegmentManagementSearch(input, s.cursor)
	if err != nil {
		return nil, err
	}
	page, err := s.searchSegments(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	if err = validateSegmentManagementPage(page); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return page, nil
}

func (s *managementDirectoryReaderImpl) SearchCampaigns(ctx context.Context, input CampaignManagementSearchInput) (*CampaignManagementPage, error) {
	if _, err := s.authorizer.AuthorizeCampaignList(ctx); err != nil {
		return nil, err
	}
	normalized, err := normalizeCampaignManagementSearch(input, s.cursor)
	if err != nil {
		return nil, err
	}
	page, err := s.searchCampaigns(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	if err = validateCampaignManagementPage(page); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementDirectoryUnavailable)
	}
	return page, nil
}

func normalizeSegmentManagementSearch(input SegmentManagementSearchInput, codec *marketingManagementCursorCodec) (normalizedSegmentManagementSearch, error) {
	name := normalizeMarketingNamePrefix(input.NamePrefix)
	if !validMarketingNamePrefix(name) || !validSegmentMode(input.Mode) || !validSegmentStatus(input.Status) {
		return normalizedSegmentManagementSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	limit, valid := normalizeMarketingPageLimit(input.Limit)
	if !valid || codec == nil || !validMarketingNumberedPage(input.Page, limit, input.Cursor) {
		return normalizedSegmentManagementSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	result := normalizedSegmentManagementSearch{page: input.Page, namePrefix: name, likePrefix: escapeMarketingLikePrefix(name), mode: input.Mode, status: input.Status, limit: limit}
	result.digest = sha256.Sum256([]byte("segment\x00" + name + "\x00" + input.Mode + "\x00" + input.Status + "\x00" + strconv.Itoa(limit)))
	if input.Cursor != "" {
		cursor, err := codec.decode(input.Cursor, result.digest)
		if err != nil {
			return normalizedSegmentManagementSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
		}
		result.cursor = cursor
	}
	return result, nil
}

func normalizeCampaignManagementSearch(input CampaignManagementSearchInput, codec *marketingManagementCursorCodec) (normalizedCampaignManagementSearch, error) {
	name := normalizeMarketingNamePrefix(input.NamePrefix)
	if !validMarketingNamePrefix(name) || !validCampaignType(input.Type) || !validCampaignStatus(input.Status) ||
		(input.TargetSegmentNo != "" && !identifierPattern.MatchString(input.TargetSegmentNo)) {
		return normalizedCampaignManagementSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	limit, valid := normalizeMarketingPageLimit(input.Limit)
	if !valid || codec == nil || !validMarketingNumberedPage(input.Page, limit, input.Cursor) {
		return normalizedCampaignManagementSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
	}
	result := normalizedCampaignManagementSearch{page: input.Page, namePrefix: name, likePrefix: escapeMarketingLikePrefix(name), typeCode: input.Type, status: input.Status, targetSegmentNo: input.TargetSegmentNo, limit: limit}
	result.digest = sha256.Sum256([]byte("campaign\x00" + name + "\x00" + input.Type + "\x00" + input.Status + "\x00" + input.TargetSegmentNo + "\x00" + strconv.Itoa(limit)))
	if input.Cursor != "" {
		cursor, err := codec.decode(input.Cursor, result.digest)
		if err != nil {
			return normalizedCampaignManagementSearch{}, bizerr.NewCode(CodeManagementDirectoryInvalid)
		}
		result.cursor = cursor
	}
	return result, nil
}

func normalizeMarketingNamePrefix(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func validMarketingNamePrefix(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= 128
}
func normalizeMarketingPageLimit(value int) (int, bool) {
	if value == 0 {
		value = defaultMarketingManagementPageSize
	}
	return value, value >= 1 && value <= maximumMarketingManagementPageSize
}
func validSegmentMode(value string) bool {
	return value == "" || value == "static" || value == "dynamic"
}
func validSegmentStatus(value string) bool {
	return value == "" || value == "draft" || value == "active" || value == "paused" || value == "archived"
}
func validCampaignType(value string) bool {
	switch value {
	case "", "discount", "coupon", "voucher", "points", "recharge_bonus", "cashback", "customer_price":
		return true
	default:
		return false
	}
}
func validCampaignStatus(value string) bool {
	switch value {
	case "", "draft", "scheduled", "active", "paused", "ended", "archived":
		return true
	default:
		return false
	}
}
func escapeMarketingLikePrefix(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value) + "%"
}

func validateSegmentManagementPage(page *SegmentManagementPage) error {
	if page == nil || page.Segments == nil || len(page.Segments) > maximumMarketingManagementPageSize || len(page.NextCursor) > maximumMarketingManagementCursorLength {
		return gerror.New("marketing segment page is invalid")
	}
	for _, item := range page.Segments {
		if !validSegmentSummary(item) {
			return gerror.New("marketing segment projection is inconsistent")
		}
	}
	return validateMarketingManagementPayload(page)
}

func validateCampaignManagementPage(page *CampaignManagementPage) error {
	if page == nil || page.Campaigns == nil || len(page.Campaigns) > maximumMarketingManagementPageSize || len(page.NextCursor) > maximumMarketingManagementCursorLength {
		return gerror.New("marketing campaign page is invalid")
	}
	for _, item := range page.Campaigns {
		if !validCampaignSummary(item) {
			return gerror.New("marketing campaign projection is inconsistent")
		}
	}
	return validateMarketingManagementPayload(page)
}

func validSegmentSummary(item SegmentManagementSummary) bool {
	return identifierPattern.MatchString(item.SegmentNo) && item.SegmentCode != "" && item.NameZhCN != "" && item.NameEnUS != "" &&
		validSegmentMode(item.Mode) && item.Mode != "" && validSegmentStatus(item.Status) && item.Status != "" && item.Version > 0 &&
		nonNegativeMarketingOverviewValues(item.ConditionCount, item.ActiveMemberCount, item.LatestSnapshotMembers) &&
		((item.Mode == "static" && item.RefreshIntervalMinutes == nil) ||
			(item.Mode == "dynamic" && item.RefreshIntervalMinutes != nil && *item.RefreshIntervalMinutes >= 5 && *item.RefreshIntervalMinutes <= 10080)) &&
		validLatestSegmentSnapshot(item) && !item.CreatedAt.IsZero() && !item.UpdatedAt.Before(item.CreatedAt)
}

func validCampaignSummary(item CampaignManagementSummary) bool {
	return identifierPattern.MatchString(item.CampaignNo) && item.CampaignCode != "" && item.NameZhCN != "" && item.NameEnUS != "" &&
		validCampaignType(item.Type) && item.Type != "" && validCampaignStatus(item.Status) && item.Status != "" &&
		item.EndsAt.After(item.StartsAt) && item.Priority >= 0 && item.Priority <= 1000000 && validCampaignStackPolicy(item.StackPolicy) &&
		(item.TargetSegmentNo == "" || identifierPattern.MatchString(item.TargetSegmentNo)) &&
		(item.TargetSnapshotNo == "" || (item.TargetSegmentNo != "" && identifierPattern.MatchString(item.TargetSnapshotNo))) &&
		(item.PerCustomerQuotaLimit == nil || (*item.PerCustomerQuotaLimit >= 1 && *item.PerCustomerQuotaLimit <= 1000000)) &&
		!item.CreatedAt.IsZero() && !item.UpdatedAt.Before(item.CreatedAt) &&
		nonNegativeMarketingOverviewValues(item.GlobalReservedCount, item.GlobalConsumedCount, item.ScopeCount, item.BenefitCount, item.ReservationCount) &&
		((item.GlobalQuotaLimit == nil && item.GlobalReservedCount == 0 && item.GlobalConsumedCount == 0) ||
			(item.GlobalQuotaLimit != nil && *item.GlobalQuotaLimit > 0 && item.GlobalReservedCount+item.GlobalConsumedCount <= *item.GlobalQuotaLimit))
}

func validLatestSegmentSnapshot(item SegmentManagementSummary) bool {
	if item.LatestSnapshotNo == "" {
		return item.LatestSnapshotStatus == "" && item.LatestSnapshotMembers == 0 && item.LatestSnapshotAt == nil
	}
	return identifierPattern.MatchString(item.LatestSnapshotNo) && item.LatestSnapshotAt != nil &&
		validSegmentSnapshotStatus(item.LatestSnapshotStatus)
}

func validSegmentSnapshotStatus(value string) bool {
	return value == "building" || value == "complete" || value == "failed" || value == "superseded"
}

func validCampaignStackPolicy(value string) bool {
	return value == "exclusive" || value == "stackable" || value == "same_group_exclusive"
}

func validateMarketingManagementPayload(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return gerror.Wrap(err, "measure marketing management response")
	}
	if len(payload) > maximumMarketingManagementPageBytes {
		return gerror.New("marketing management response exceeds byte limit")
	}
	return nil
}
