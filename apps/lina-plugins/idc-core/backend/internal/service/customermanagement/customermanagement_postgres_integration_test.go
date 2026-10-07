// This file verifies bounded customer management projections against PostgreSQL.

package customermanagement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const managementIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

// TestDAONumberedCustomerPagesCountsAndFilters checks real SQL pagination,
// exact filtered totals, a far empty page and tenant isolation in its own schema.
func TestDAONumberedCustomerPagesCountsAndFilters(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	created := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	for index := 1; index <= 25; index++ {
		insertManagementCustomer(t, ctx, fmt.Sprintf("CUS_%032x", index), customer.TypeIndividual,
			customer.VerificationUnverified, "Literal_% Customer", created.Add(-time.Duration(index)*time.Second))
	}
	store := newDAOPersistence()
	for _, scenario := range []struct{ page, size, count int }{{1, 10, 10}, {2, 10, 10}, {3, 10, 5}, {50, 10, 0}, {1, 100, 25}} {
		result, err := store.search(ctx, normalizedSearch{page: scenario.page, limit: scenario.size})
		if err != nil || result.Total != 25 || len(result.Customers) != scenario.count {
			t.Fatalf("scenario=%+v result=%+v err=%v", scenario, result, err)
		}
		if scenario.count > 0 && result.Customers[0].CustomerNo != fmt.Sprintf("CUS_%032x", (scenario.page-1)*scenario.size+1) {
			t.Fatalf("wrong numeric offset: %+v", result)
		}
	}
	filtered, err := store.search(ctx, normalizedSearch{page: 1, limit: 10, namePrefix: "absent"})
	if err != nil || filtered.Total != 0 || len(filtered.Customers) != 0 {
		t.Fatalf("empty filtered total: %+v %v", filtered, err)
	}
}

func TestDAOSearchProjectsVerificationLevelTagsAndStableCursor(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	created := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	individualNo := "CUS_11111111111111111111111111111111"
	enterpriseNo := "CUS_22222222222222222222222222222222"
	insertManagementCustomer(t, ctx, individualNo, customer.TypeIndividual,
		customer.VerificationVerified, "literal_% customer", created)
	insertManagementCustomer(t, ctx, enterpriseNo, customer.TypeEnterprise,
		customer.VerificationPending, "Enterprise Customer", created.Add(-time.Minute))
	insertManagementLevelAndTags(t, ctx, individualNo)

	store := newDAOPersistence()
	first, err := store.search(ctx, normalizedSearch{limit: 1})
	if err != nil {
		t.Fatalf("search first management page: %v", err)
	}
	if len(first.Customers) != 1 || first.NextCursor == "" {
		t.Fatalf("first page=%+v", first)
	}
	firstCustomer := first.Customers[0]
	if firstCustomer.CustomerNo != individualNo ||
		firstCustomer.Verification != customer.VerificationVerified ||
		firstCustomer.Level == nil || firstCustomer.Level.Code != "gold" ||
		firstCustomer.TagCount != 2 {
		t.Fatalf("unexpected first projection: %+v", firstCustomer)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal management projection: %v", err)
	}
	if strings.Contains(string(encoded), "Protected Legal Name") || strings.Contains(string(encoded), strings.Repeat("a", 64)) {
		t.Fatalf("management projection leaked legal identity: %s", encoded)
	}

	secondInput := normalizedSearch{limit: 1}
	secondInput.cursor, err = decodeCustomerCursor(first.NextCursor, secondInput)
	if err != nil {
		t.Fatalf("decode second management cursor: %v", err)
	}
	second, err := store.search(ctx, secondInput)
	if err != nil {
		t.Fatalf("search second management page: %v", err)
	}
	if len(second.Customers) != 1 || second.NextCursor != "" ||
		second.Customers[0].CustomerNo != enterpriseNo ||
		second.Customers[0].Verification != customer.VerificationPending ||
		second.Customers[0].Level != nil || second.Customers[0].TagCount != 0 {
		t.Fatalf("unexpected second projection: %+v", second)
	}
	tamperedBoundary := *secondInput.cursor
	tamperedBoundary.InternalID += 100000
	tampered := normalizedSearch{limit: 1, cursor: &tamperedBoundary}
	if _, err = store.search(ctx, tampered); err == nil ||
		!strings.Contains(err.Error(), "validate management customer cursor boundary") {
		t.Fatalf("non-authoritative cursor boundary returned %v", err)
	}

	literal, err := store.search(ctx, normalizedSearch{namePrefix: "literal_%", limit: 10})
	if err != nil {
		t.Fatalf("search literal wildcard prefix: %v", err)
	}
	if len(literal.Customers) != 1 || literal.Customers[0].CustomerNo != individualNo {
		t.Fatalf("literal prefix escaped incorrectly: %+v", literal.Customers)
	}
	byGroup, err := store.search(ctx, normalizedSearch{
		groupNo: "GRP_11111111111111111111111111111111", limit: 10,
	})
	if err != nil || len(byGroup.Customers) != 1 || byGroup.Customers[0].CustomerNo != individualNo {
		t.Fatalf("group filter projection=%+v err=%v", byGroup, err)
	}
	byTag, err := store.search(ctx, normalizedSearch{
		tagNo: "TAG_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", limit: 10,
	})
	if err != nil || len(byTag.Customers) != 1 || byTag.Customers[0].CustomerNo != individualNo {
		t.Fatalf("tag filter projection=%+v err=%v", byTag, err)
	}
	missingSegment, err := store.search(ctx, normalizedSearch{
		tagNo: "TAG_cccccccccccccccccccccccccccccccc", limit: 10,
	})
	if err != nil || len(missingSegment.Customers) != 0 {
		t.Fatalf("missing tag filter projection=%+v err=%v", missingSegment, err)
	}
	groupCandidates, err := store.searchSegments(ctx, normalizedSegmentSearch{
		kind: SegmentKindGroup, locale: SegmentLocaleZhCN, namePrefix: "重点", limit: 10,
	})
	if err != nil || len(groupCandidates.Items) != 1 ||
		groupCandidates.Items[0].Number != "GRP_11111111111111111111111111111111" {
		t.Fatalf("group candidates=%+v err=%v", groupCandidates, err)
	}
	tagFirst, err := store.searchSegments(ctx, normalizedSegmentSearch{
		kind: SegmentKindTag, locale: SegmentLocaleEnUS, namePrefix: "Tag", limit: 1,
	})
	if err != nil || len(tagFirst.Items) != 1 || tagFirst.NextCursor == "" {
		t.Fatalf("first tag candidates=%+v err=%v", tagFirst, err)
	}
	tagSecondInput := normalizedSegmentSearch{
		kind: SegmentKindTag, locale: SegmentLocaleEnUS, namePrefix: "Tag", limit: 1,
	}
	tagSecondInput.cursor, err = decodeSegmentCursor(tagFirst.NextCursor, tagSecondInput)
	if err != nil {
		t.Fatalf("decode tag candidate cursor: %v", err)
	}
	tagSecond, err := store.searchSegments(ctx, tagSecondInput)
	if err != nil || len(tagSecond.Items) != 1 || tagSecond.NextCursor != "" ||
		tagSecond.Items[0].Number == tagFirst.Items[0].Number {
		t.Fatalf("second tag candidates=%+v err=%v", tagSecond, err)
	}
	assignmentInput := normalizedAssignmentSearch{
		customerNo: individualNo, kind: SegmentKindTag, locale: SegmentLocaleEnUS, limit: 1,
	}
	assignmentFirst, err := searchAssignments(ctx, assignmentInput)
	if err != nil || len(assignmentFirst.Items) != 1 || assignmentFirst.NextCursor == "" {
		t.Fatalf("first customer assignments=%+v err=%v", assignmentFirst, err)
	}
	assignmentInput.cursor, err = decodeAssignmentCursor(assignmentFirst.NextCursor, assignmentInput)
	if err != nil {
		t.Fatalf("decode customer assignment cursor: %v", err)
	}
	assignmentSecond, err := searchAssignments(ctx, assignmentInput)
	if err != nil || len(assignmentSecond.Items) != 1 || assignmentSecond.NextCursor != "" ||
		assignmentSecond.Items[0].Number == assignmentFirst.Items[0].Number {
		t.Fatalf("second customer assignments=%+v err=%v", assignmentSecond, err)
	}
	foreignCursorInput := normalizedAssignmentSearch{
		customerNo: enterpriseNo, kind: SegmentKindTag, locale: SegmentLocaleEnUS, limit: 1,
	}
	if _, err = decodeAssignmentCursor(assignmentFirst.NextCursor, foreignCursorInput); err == nil {
		t.Fatal("customer assignment cursor must be bound to one customer")
	}
	overview, err := store.getOverview(ctx, individualNo)
	if err != nil {
		t.Fatalf("load management customer overview: %v", err)
	}
	if overview.Customer.CustomerNo != individualNo || len(overview.Groups.Items) != 1 ||
		overview.Groups.Items[0].Code != "priority" || len(overview.Tags.Items) != 2 ||
		overview.Counts.ActiveGroups != 1 || overview.Counts.ActiveTags != 2 {
		t.Fatalf("unexpected management customer overview: %+v", overview)
	}
	overviewJSON, err := json.Marshal(overview)
	if err != nil {
		t.Fatalf("marshal management overview: %v", err)
	}
	if strings.Contains(string(overviewJSON), "Protected Legal Name") ||
		strings.Contains(string(overviewJSON), strings.Repeat("a", 64)) {
		t.Fatalf("management overview leaked legal identity: %s", overviewJSON)
	}
}

func TestDAOSearchFailsClosedWhenVerificationProjectionIsMissing(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	created := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	customerNo := "CUS_33333333333333333333333333333333"
	insertManagementCustomer(t, ctx, customerNo, customer.TypeIndividual,
		customer.VerificationUnverified, "Incomplete Customer", created)
	if _, err := dao.CustomerIndividualProfile.Ctx(ctx).
		Where(dao.CustomerIndividualProfile.Columns().CustomerNo, customerNo).Delete(); err != nil {
		t.Fatalf("remove verification projection fixture: %v", err)
	}
	if _, err := newDAOPersistence().search(ctx, normalizedSearch{limit: 10}); err == nil ||
		!strings.Contains(err.Error(), "verification projection is incomplete") {
		t.Fatalf("missing verification projection returned %v", err)
	}
}

func insertManagementCustomer(
	t *testing.T,
	ctx context.Context,
	customerNo string,
	typeCode customer.Type,
	verification customer.VerificationStatus,
	displayName string,
	createdAt time.Time,
) {
	t.Helper()
	if _, err := dao.CustomerSubject.Ctx(ctx).Data(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		CustomerTypeCode: string(typeCode), StatusCode: string(customer.StatusActive),
		DisplayName: displayName, SearchName: strings.ToLower(displayName),
		CountryCode: "CN", PreferredLocale: "zh-CN", Version: 1,
		CreatedBy: 71, UpdatedBy: 71, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	}).Insert(); err != nil {
		t.Fatalf("insert management customer: %v", err)
	}
	switch typeCode {
	case customer.TypeIndividual:
		profile := do.CustomerIndividualProfile{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
			CustomerTypeCode: string(typeCode), IdentityStatusCode: string(verification),
			CreatedAt: &createdAt, UpdatedAt: &createdAt,
		}
		if verification == customer.VerificationVerified {
			profile.LegalName = "Protected Legal Name"
			profile.LegalNameMasked = "P*** N***"
			profile.IdentityIdentifierMasked = "110***********1234"
			profile.IdentityLookupDigest = strings.Repeat("a", 64)
			profile.IdentityCaseNo = "CASE_VERIFIED_1"
			profile.IdentityVerifiedAt = &createdAt
		}
		if _, err := dao.CustomerIndividualProfile.Ctx(ctx).Data(profile).Insert(); err != nil {
			t.Fatalf("insert management individual profile: %v", err)
		}
	case customer.TypeEnterprise:
		if _, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Data(do.CustomerEnterpriseProfile{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
			CustomerTypeCode: string(typeCode), RegistrationCountryCode: "CN",
			VerificationStatusCode: string(verification), CreatedAt: &createdAt, UpdatedAt: &createdAt,
		}).Insert(); err != nil {
			t.Fatalf("insert management enterprise profile: %v", err)
		}
	default:
		t.Fatalf("unsupported management customer type %q", typeCode)
	}
}

func insertManagementLevelAndTags(t *testing.T, ctx context.Context, customerNo string) {
	t.Helper()
	if _, err := dao.CustomerLevel.Ctx(ctx).Data(do.CustomerLevel{
		TenantId: idccap.DefaultTenantID, LevelNo: "LVL_11111111111111111111111111111111",
		LevelCode: "gold", NameZhCn: "金牌", NameEnUs: "Gold", Priority: 100,
		StatusCode: "active", Version: 1,
	}).Insert(); err != nil {
		t.Fatalf("insert management customer level: %v", err)
	}
	if _, err := dao.CustomerLevelAssignment.Ctx(ctx).Data(do.CustomerLevelAssignment{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		LevelNo: "LVL_11111111111111111111111111111111", SourceCode: "manual",
		StatusCode: "active", AssignedBy: 71,
	}).Insert(); err != nil {
		t.Fatalf("assign management customer level: %v", err)
	}
	if _, err := dao.CustomerGroup.Ctx(ctx).Data(do.CustomerGroup{
		TenantId: idccap.DefaultTenantID, GroupNo: "GRP_11111111111111111111111111111111",
		GroupCode: "priority", NameZhCn: "重点客户", NameEnUs: "Priority",
		StatusCode: "active", Version: 1,
	}).Insert(); err != nil {
		t.Fatalf("insert management customer group: %v", err)
	}
	if _, err := dao.CustomerGroupAssignment.Ctx(ctx).Data(do.CustomerGroupAssignment{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		GroupNo: "GRP_11111111111111111111111111111111", SourceCode: "manual",
		StatusCode: "active", AssignedBy: 71,
	}).Insert(); err != nil {
		t.Fatalf("assign management customer group: %v", err)
	}
	for index, suffix := range []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} {
		tagNo := "TAG_" + suffix
		if _, err := dao.CustomerTag.Ctx(ctx).Data(do.CustomerTag{
			TenantId: idccap.DefaultTenantID, TagNo: tagNo,
			TagCode: fmt.Sprintf("tag_%d", index), NameZhCn: fmt.Sprintf("标签%d", index),
			NameEnUs: fmt.Sprintf("Tag %d", index), ColorToken: "default",
			StatusCode: "active", Version: 1,
		}).Insert(); err != nil {
			t.Fatalf("insert management customer tag: %v", err)
		}
		if _, err := dao.CustomerTagAssignment.Ctx(ctx).Data(do.CustomerTagAssignment{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo, TagNo: tagNo,
			SourceCode: "manual", StatusCode: "active", AssignedBy: 71,
		}).Insert(); err != nil {
			t.Fatalf("assign management customer tag: %v", err)
		}
	}
}

func configureManagementIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(managementIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL management integration tests", managementIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create management integration admin database: %v", err)
	}
	schema := newManagementIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteManagementPostgreSQLIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated management schema: %v", err)
	}
	originalConfig := gdb.GetAllConfig()
	var scopedDB gdb.DB
	t.Cleanup(func() {
		if scopedDB != nil {
			if closeErr := scopedDB.Close(ctx); closeErr != nil {
				t.Errorf("close scoped management database: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore management database config: %v", restoreErr)
		}
		if !strings.HasPrefix(schema, "linapro_management_test_") {
			t.Errorf("refusing to drop unexpected management schema %q", schema)
		} else if _, dropErr := adminDB.Exec(ctx,
			"DROP SCHEMA "+quoteManagementPostgreSQLIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop isolated management schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close management admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: appendManagementDSNParameter(
			dsn, "search_path="+url.QueryEscape(schema),
		)}},
	}); err != nil {
		t.Fatalf("configure management integration database: %v", err)
	}
	scopedDB = g.DB()
	if _, err = scopedDB.Exec(ctx, managementDictionaryFixtureSQL); err != nil {
		t.Fatalf("create management dictionary fixtures: %v", err)
	}
	for _, migrationName := range []string{
		"001-core-foundation.sql",
		"003-customer-account.sql",
		"005-audit-evidence-sealing.sql",
		"006-audit-replay-lookup-and-target-index.sql",
		"008-audit-keyset-filter-indexes.sql",
		"009-audit-customer-link.sql",
		"070-customer-short-code.sql",
	} {
		migration, readErr := os.ReadFile(managementMigrationPath(t, migrationName))
		if readErr != nil {
			t.Fatalf("read management migration %s: %v", migrationName, readErr)
		}
		if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute management migration %s: %v", migrationName, err)
		}
		if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("repeat management migration %s idempotently: %v", migrationName, err)
		}
	}
}

const managementDictionaryFixtureSQL = `
CREATE TABLE sys_dict_type (
    "id" INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "name" VARCHAR(128) NOT NULL DEFAULT '',
    "type" VARCHAR(128) NOT NULL DEFAULT '',
    "status" SMALLINT NOT NULL DEFAULT 1,
    "is_builtin" SMALLINT NOT NULL DEFAULT 0,
    "allow_tenant_override" BOOL NOT NULL DEFAULT FALSE,
    "remark" VARCHAR(512) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ,
    "updated_at" TIMESTAMPTZ,
    "deleted_at" TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_management_test_dict_type ON sys_dict_type ("tenant_id", "type");
CREATE TABLE sys_dict_data (
    "id" INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "dict_type" VARCHAR(128) NOT NULL DEFAULT '',
    "label" VARCHAR(128) NOT NULL DEFAULT '',
    "value" VARCHAR(128) NOT NULL DEFAULT '',
    "sort" INT NOT NULL DEFAULT 0,
    "tag_style" VARCHAR(128) NOT NULL DEFAULT '',
    "css_class" VARCHAR(128) NOT NULL DEFAULT '',
    "status" SMALLINT NOT NULL DEFAULT 1,
    "is_builtin" SMALLINT NOT NULL DEFAULT 0,
    "remark" VARCHAR(512) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ,
    "updated_at" TIMESTAMPTZ,
    "deleted_at" TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_management_test_dict_data ON sys_dict_data ("tenant_id", "dict_type", "value");
`

func newManagementIntegrationSchema(t *testing.T) string {
	t.Helper()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate management schema suffix: %v", err)
	}
	return "linapro_management_test_" + hex.EncodeToString(suffix[:])
}

func quoteManagementPostgreSQLIdentifier(value string) string {
	for _, character := range value {
		if character == '_' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' {
			continue
		}
		panic(fmt.Sprintf("unsafe PostgreSQL identifier %q", value))
	}
	return `"` + value + `"`
}

func appendManagementDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func managementMigrationPath(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve management integration test path")
	}
	return filepath.Clean(filepath.Join(
		filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", name,
	))
}
