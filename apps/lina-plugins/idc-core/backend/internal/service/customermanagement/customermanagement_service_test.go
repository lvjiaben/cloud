// This file verifies management authorization, bounded validation, and query ordering.

package customermanagement

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const managementTestCustomerNo = "CUS_0123456789abcdef0123456789abcdef"

type managementTestBizCtx struct {
	current bizctxcap.CurrentContext
}

func (b managementTestBizCtx) Current(context.Context) bizctxcap.CurrentContext {
	return b.current
}

type managementTestPrincipal struct {
	audience *principal.Audience
	err      error
	calls    int
}

func (p *managementTestPrincipal) Current(context.Context) (*principal.Audience, error) {
	p.calls++
	return p.audience, p.err
}

func (*managementTestPrincipal) EnsureCurrentStaff(context.Context) (*principal.Audience, error) {
	return nil, nil
}

func (*managementTestPrincipal) RedeemCustomerOnboarding(
	context.Context,
	int,
	string,
	string,
	int,
) (*principal.CustomerBinding, error) {
	return nil, nil
}

func (*managementTestPrincipal) OnboardingStatus(context.Context) (*principal.OnboardingState, error) {
	return &principal.OnboardingState{}, nil
}

type managementTestAuthz struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
	err           error
	batchCalls    int
	platformCalls int
}

func (*managementTestAuthz) BatchGetPermissions(
	context.Context,
	[]authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}

func (a *managementTestAuthz) BatchHasPermissions(
	context.Context,
	[]authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	a.batchCalls++
	if a.err != nil {
		return nil, a.err
	}
	return a.grants, nil
}

func (*managementTestAuthz) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}

func (a *managementTestAuthz) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	a.platformCalls++
	if a.err != nil {
		return false, a.err
	}
	return a.platformAdmin, nil
}

func (*managementTestAuthz) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return nil
}

type managementTestPersistence struct {
	page         *Page
	segments     *SegmentPage
	overview     *Overview
	err          error
	calls        int
	input        normalizedSearch
	segmentInput normalizedSegmentSearch
}

func (p *managementTestPersistence) searchSegments(
	_ context.Context,
	input normalizedSegmentSearch,
) (*SegmentPage, error) {
	p.calls++
	p.segmentInput = input
	return p.segments, p.err
}

func (p *managementTestPersistence) search(
	_ context.Context,
	input normalizedSearch,
) (*Page, error) {
	p.calls++
	p.input = input
	return p.page, p.err
}

func (p *managementTestPersistence) getOverview(context.Context, string) (*Overview, error) {
	p.calls++
	return p.overview, p.err
}

func TestSearchAuthorizesBeforeBoundedPersistence(t *testing.T) {
	store := &managementTestPersistence{page: &Page{Customers: []Summary{}}}
	principalSvc := &managementTestPrincipal{audience: &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	authorization := &managementTestAuthz{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permissionCustomerList): true,
	}}
	service, err := newService(
		managementTestBizCtx{current: bizctxcap.CurrentContext{
			UserID: 71, DataScope: 2, PlatformBypass: true,
		}},
		principalSvc,
		authorization,
		store,
		Config{MaximumPageSize: 40, MaximumResponseBytes: 4096},
	)
	if err != nil {
		t.Fatalf("create customer management service: %v", err)
	}
	page, err := service.Search(context.Background(), SearchInput{
		NamePrefix: "  Example   CUSTOMER ", Type: customer.TypeEnterprise,
		Status:  customer.StatusActive,
		GroupNo: "GRP_11111111111111111111111111111111",
		TagNo:   "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Limit:   12,
	})
	if err != nil || page != store.page {
		t.Fatalf("search page=%+v err=%v", page, err)
	}
	if principalSvc.calls != 1 || authorization.batchCalls != 1 ||
		authorization.platformCalls != 1 || store.calls != 1 {
		t.Fatalf("unexpected dependency calls: principal=%+v authz=%+v store=%+v", principalSvc, authorization, store)
	}
	if store.input.namePrefix != "example customer" || store.input.typeCode != customer.TypeEnterprise ||
		store.input.status != customer.StatusActive ||
		store.input.groupNo != "GRP_11111111111111111111111111111111" ||
		store.input.tagNo != "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || store.input.limit != 12 {
		t.Fatalf("unexpected normalized search: %+v", store.input)
	}
}

func TestSearchDenialsNeverReachPersistence(t *testing.T) {
	tests := []struct {
		name          string
		current       bizctxcap.CurrentContext
		audience      *principal.Audience
		authorization *managementTestAuthz
	}{
		{
			name: "department scope", current: bizctxcap.CurrentContext{UserID: 71, DataScope: 3},
			audience: activeStaffAudience(), authorization: grantedManagementAuthz(),
		},
		{
			name: "delegated identity", current: bizctxcap.CurrentContext{UserID: 71, DataScope: 2, ActingUserID: 9},
			audience: activeStaffAudience(), authorization: grantedManagementAuthz(),
		},
		{
			name: "customer audience", current: bizctxcap.CurrentContext{UserID: 71, DataScope: 2},
			audience:      &principal.Audience{UserID: 71, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: managementTestCustomerNo},
			authorization: grantedManagementAuthz(),
		},
		{
			name: "missing exact grant", current: bizctxcap.CurrentContext{UserID: 71, DataScope: 2},
			audience: activeStaffAudience(), authorization: &managementTestAuthz{},
		},
		{
			name: "authorization unavailable", current: bizctxcap.CurrentContext{UserID: 71, DataScope: 2},
			audience: activeStaffAudience(), authorization: &managementTestAuthz{err: errors.New("authz unavailable")},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &managementTestPersistence{}
			service, err := newService(
				managementTestBizCtx{current: test.current},
				&managementTestPrincipal{audience: test.audience},
				test.authorization,
				store,
				Config{},
			)
			if err != nil {
				t.Fatalf("create service: %v", err)
			}
			if _, err = service.Search(context.Background(), SearchInput{}); !bizerr.Is(err, customer.CodeNotAccessible) {
				t.Fatalf("search returned %v, want inaccessible", err)
			}
			if store.calls != 0 {
				t.Fatalf("denied search reached persistence: %+v", store)
			}
		})
	}
}

func TestSearchValidationAndResponseBounds(t *testing.T) {
	service := authorizedManagementTestService(t, &managementTestPersistence{
		page: &Page{Customers: []Summary{}},
	})
	invalid := []SearchInput{
		{NamePrefix: strings.Repeat("界", maximumNamePrefixRunes+1)},
		{Type: customer.Type("unknown")},
		{Status: customer.Status("unknown")},
		{GroupNo: "GRP_invalid"},
		{TagNo: "TAG_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{Limit: absoluteMaximumPageSize + 1},
		{Cursor: "invalid"},
	}
	for index, input := range invalid {
		if _, err := service.Search(context.Background(), input); !bizerr.Is(err, customer.CodeProfileInvalid) {
			t.Fatalf("input %d returned %v, want invalid", index, err)
		}
	}

	store := &managementTestPersistence{page: &Page{Customers: []Summary{{
		CustomerNo:  managementTestCustomerNo,
		DisplayName: strings.Repeat("x", 4096),
	}}}}
	service = authorizedManagementTestService(t, store)
	if _, err := service.Search(context.Background(), SearchInput{}); !bizerr.Is(err, customer.CodeUnavailable) {
		t.Fatalf("oversized response returned %v", err)
	}
}

func TestGetOverviewRequiresExactQueryAuthorityBeforeTargetValidation(t *testing.T) {
	validOverview := &Overview{
		Customer: Summary{CustomerNo: managementTestCustomerNo},
		Groups:   SegmentPreview{Items: []SegmentSummary{}},
		Tags:     SegmentPreview{Items: []SegmentSummary{}},
	}
	store := &managementTestPersistence{overview: validOverview}
	service, err := newService(
		managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 71, DataScope: 2}},
		&managementTestPrincipal{audience: activeStaffAudience()},
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCustomerQuery): true,
		}},
		store,
		Config{},
	)
	if err != nil {
		t.Fatalf("create overview service: %v", err)
	}
	overview, err := service.GetOverview(context.Background(), managementTestCustomerNo)
	if err != nil || overview != validOverview || store.calls != 1 {
		t.Fatalf("overview=%+v store=%+v err=%v", overview, store, err)
	}

	store = &managementTestPersistence{}
	service, err = newService(
		managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 71, DataScope: 2}},
		&managementTestPrincipal{audience: activeStaffAudience()},
		grantedManagementAuthz(),
		store,
		Config{},
	)
	if err != nil {
		t.Fatalf("create list-only overview service: %v", err)
	}
	if _, err = service.GetOverview(context.Background(), "invalid"); !bizerr.Is(err, customer.CodeNotAccessible) || store.calls != 0 {
		t.Fatalf("list-only invalid overview reached persistence: store=%+v err=%v", store, err)
	}
}

func TestSearchSegmentsUsesListAuthorityAndBindsCursorFilters(t *testing.T) {
	store := &managementTestPersistence{segments: &SegmentPage{Items: []SegmentSummary{}}}
	service := authorizedManagementTestService(t, store)
	page, err := service.SearchSegments(context.Background(), SegmentSearchInput{
		Kind: SegmentKindTag, Locale: SegmentLocaleEnUS, NamePrefix: "  Gold   Tier ", Limit: 12,
	})
	if err != nil || page != store.segments || store.calls != 1 ||
		store.segmentInput.kind != SegmentKindTag || store.segmentInput.locale != SegmentLocaleEnUS ||
		store.segmentInput.namePrefix != "Gold Tier" || store.segmentInput.limit != 12 {
		t.Fatalf("segment page=%+v store=%+v err=%v", page, store, err)
	}
	for _, input := range []SegmentSearchInput{
		{Kind: SegmentKind("unknown"), Locale: SegmentLocaleZhCN},
		{Kind: SegmentKindGroup, Locale: SegmentLocale("unknown")},
		{Kind: SegmentKindGroup, Locale: SegmentLocaleZhCN, NamePrefix: strings.Repeat("界", maximumSegmentNamePrefixRunes+1)},
		{Kind: SegmentKindGroup, Locale: SegmentLocaleZhCN, Limit: absoluteMaximumPageSize + 1},
		{Kind: SegmentKindGroup, Locale: SegmentLocaleZhCN, Cursor: "invalid"},
	} {
		if _, err = service.SearchSegments(context.Background(), input); !bizerr.Is(err, customer.CodeProfileInvalid) {
			t.Fatalf("segment input %+v returned %v, want invalid", input, err)
		}
	}

	normalized := normalizedSegmentSearch{
		kind: SegmentKindGroup, locale: SegmentLocaleZhCN, namePrefix: "重点", limit: 25,
	}
	encoded, err := encodeSegmentCursor(segmentCursorBoundary{
		internalID: 9, number: "GRP_11111111111111111111111111111111", sortName: "重点客户",
	}, normalized)
	if err != nil {
		t.Fatalf("encode valid segment cursor: %v", err)
	}
	if _, err = decodeSegmentCursor(encoded, normalized); err != nil {
		t.Fatalf("decode valid segment cursor: %v", err)
	}
	emptyNumberCursor, err := encodeSegmentCursor(segmentCursorBoundary{
		internalID: 9, sortName: "重点客户",
	}, normalized)
	if err != nil {
		t.Fatalf("encode empty-number segment cursor: %v", err)
	}
	if _, err = decodeSegmentCursor(emptyNumberCursor, normalized); err == nil {
		t.Fatal("segment cursor with an empty business number was accepted")
	}
	if _, err = projectSegmentPage([]segmentRow{{
		InternalID: 9, Code: "priority", NameZhCn: "重点客户", NameEnUs: "Priority", Status: "active",
	}}, normalized); err == nil {
		t.Fatal("segment projection with an empty business number was accepted")
	}
	changed := normalized
	changed.locale = SegmentLocaleEnUS
	if _, err = decodeSegmentCursor(encoded, changed); err == nil {
		t.Fatal("segment cursor was accepted under a different locale")
	}
	changed = normalized
	changed.namePrefix = "普通"
	if _, err = decodeSegmentCursor(encoded, changed); err == nil {
		t.Fatal("segment cursor was accepted under a different prefix")
	}
}

func TestProjectPageUsesStableDescendingBoundary(t *testing.T) {
	created := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Minute)
	rows := []*entity.CustomerSubject{
		managementCustomerRow(3, "CUS_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", created, updated),
		managementCustomerRow(2, "CUS_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", created.Add(-time.Minute), updated),
		managementCustomerRow(1, "CUS_cccccccccccccccccccccccccccccccc", created.Add(-time.Hour), updated),
	}
	input := normalizedSearch{limit: 2}
	page, err := projectPage(rows, input, testBatchProjection(rows[:2]))
	if err != nil || len(page.Customers) != 2 || page.NextCursor == "" {
		t.Fatalf("project page=%+v err=%v", page, err)
	}
	boundary, err := decodeCustomerCursor(page.NextCursor, input)
	if err != nil || boundary.InternalID != rows[1].Id ||
		boundary.CustomerNo != rows[1].CustomerNo || !boundary.CreatedAt.Equal(*rows[1].CreatedAt) {
		t.Fatalf("unexpected next cursor: %+v err=%v", boundary, err)
	}
	if page.Customers[0].CustomerNo != rows[0].CustomerNo ||
		page.Customers[1].CustomerNo != rows[1].CustomerNo {
		t.Fatalf("unexpected page order: %+v", page.Customers)
	}
}

func TestProjectPageRejectsEmptyPersistedEnumsAndEscapesLiteralPrefix(t *testing.T) {
	created := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Minute)
	row := managementCustomerRow(1, managementTestCustomerNo, created, updated)
	row.CustomerTypeCode = ""
	if _, err := projectPage([]*entity.CustomerSubject{row}, normalizedSearch{limit: 1}, testBatchProjection([]*entity.CustomerSubject{row})); err == nil {
		t.Fatal("empty persisted customer type was accepted")
	}
	row = managementCustomerRow(1, managementTestCustomerNo, created, updated)
	row.StatusCode = ""
	if _, err := projectPage([]*entity.CustomerSubject{row}, normalizedSearch{limit: 1}, testBatchProjection([]*entity.CustomerSubject{row})); err == nil {
		t.Fatal("empty persisted customer status was accepted")
	}
	if escaped := escapeLikeLiteral(`a_b%\\c`); escaped != `a\_b\%\\\\c` {
		t.Fatalf("escaped prefix=%q", escaped)
	}
}

func TestCustomerCursorBindsNormalizedFiltersAndLimit(t *testing.T) {
	input := normalizedSearch{
		namePrefix: "example", typeCode: customer.TypeIndividual,
		status:  customer.StatusActive,
		groupNo: "GRP_11111111111111111111111111111111",
		tagNo:   "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		limit:   25,
	}
	boundary := cursorBoundary{
		CreatedAt:  time.Date(2026, 8, 8, 12, 0, 0, 123456000, time.UTC),
		InternalID: 42, CustomerNo: managementTestCustomerNo,
	}
	encoded := encodeCustomerCursor(boundary, input)
	decoded, err := decodeCustomerCursor(encoded, input)
	if err != nil || decoded.InternalID != boundary.InternalID ||
		decoded.CustomerNo != boundary.CustomerNo || !decoded.CreatedAt.Equal(boundary.CreatedAt) {
		t.Fatalf("decoded cursor=%+v err=%v", decoded, err)
	}
	changed := input
	changed.limit++
	if _, err = decodeCustomerCursor(encoded, changed); err == nil {
		t.Fatal("cursor was accepted under a different page limit")
	}
	changed = input
	changed.status = customer.StatusRestricted
	if _, err = decodeCustomerCursor(encoded, changed); err == nil {
		t.Fatal("cursor was accepted under different filters")
	}
	changed = input
	changed.groupNo = "GRP_22222222222222222222222222222222"
	if _, err = decodeCustomerCursor(encoded, changed); err == nil {
		t.Fatal("cursor was accepted under a different group filter")
	}
	changed = input
	changed.tagNo = "TAG_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err = decodeCustomerCursor(encoded, changed); err == nil {
		t.Fatal("cursor was accepted under a different tag filter")
	}
}

func testBatchProjection(rows []*entity.CustomerSubject) batchProjection {
	projection := batchProjection{
		verificationByCustomer: make(map[string]customer.VerificationStatus, len(rows)),
		levelByCustomer:        make(map[string]*LevelSummary),
		tagCountByCustomer:     make(map[string]int, len(rows)),
	}
	for _, row := range rows {
		projection.verificationByCustomer[row.CustomerNo] = customer.VerificationUnverified
		projection.tagCountByCustomer[row.CustomerNo] = 0
	}
	return projection
}

func authorizedManagementTestService(t *testing.T, store persistence) Service {
	t.Helper()
	service, err := newService(
		managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 71, DataScope: 2}},
		&managementTestPrincipal{audience: activeStaffAudience()},
		grantedManagementAuthz(),
		store,
		Config{MaximumResponseBytes: 1024},
	)
	if err != nil {
		t.Fatalf("create authorized service: %v", err)
	}
	return service
}

func activeStaffAudience() *principal.Audience {
	return &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
}

func grantedManagementAuthz() *managementTestAuthz {
	return &managementTestAuthz{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permissionCustomerList): true,
	}}
}

func managementCustomerRow(
	id int64,
	customerNo string,
	created time.Time,
	updated time.Time,
) *entity.CustomerSubject {
	return &entity.CustomerSubject{
		Id: id, CustomerNo: customerNo, CustomerTypeCode: string(customer.TypeIndividual),
		StatusCode: string(customer.StatusActive), DisplayName: customerNo,
		CountryCode: "CN", PreferredLocale: "zh-CN", Version: 1,
		CreatedAt: &created, UpdatedAt: &updated,
	}
}
