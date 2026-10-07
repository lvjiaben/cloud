// This file verifies customer-profile transaction and replay invariants against PostgreSQL.

package customer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/audit"
)

const customerIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type transactionalAuditWriter struct {
	mutex       sync.Mutex
	transitions []audit.TransitionInput
	err         error
}

func (w *transactionalAuditWriter) AppendTransition(
	ctx context.Context,
	input audit.TransitionInput,
) (*audit.Event, error) {
	if gdb.TXFromCtx(ctx, dao.CustomerSubject.Group()) == nil {
		return nil, errors.New("audit transition was not called inside customer transaction")
	}
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.err != nil {
		return nil, w.err
	}
	w.transitions = append(w.transitions, input)
	return &audit.Event{CustomerNo: input.CustomerNo, OperationCode: input.OperationCode}, nil
}

func (*transactionalAuditWriter) RecordObservation(
	context.Context,
	audit.ObservationInput,
) (*audit.Event, error) {
	return nil, errors.New("unexpected audit observation")
}

func (w *transactionalAuditWriter) transitionSnapshot() []audit.TransitionInput {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return append([]audit.TransitionInput(nil), w.transitions...)
}

func TestDAOUpdateOwnBasicsConvergesConcurrentReplay(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)

	writer := &transactionalAuditWriter{}
	persistence := &daoPersistence{
		audit: writer,
		now:   func() time.Time { return time.Date(2026, 8, 6, 12, 0, 0, 123456000, time.UTC) },
	}
	input, err := normalizeUpdate(UpdateOwnBasicsInput{
		CustomerNo: testCustomerNo, DisplayName: "Updated Customer", CountryCode: "US",
		PreferredLocale: "en-US", ExpectedVersion: 1, IdempotencyKey: "concurrent-request",
	})
	if err != nil {
		t.Fatalf("normalize integration input: %v", err)
	}

	const callers = 16
	var waitGroup sync.WaitGroup
	start := make(chan struct{})
	results := make([]*Profile, callers)
	errorsOut := make([]error, callers)
	for index := range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results[index], errorsOut[index] = persistence.updateOwnBasics(ctx, 81, input)
		}()
	}
	close(start)
	waitGroup.Wait()
	for index := range callers {
		if errorsOut[index] != nil {
			t.Fatalf("concurrent replay %d failed: %v", index, errorsOut[index])
		}
		if results[index] == nil || results[index].Subject.Version != 2 ||
			results[index].Subject.DisplayName != "Updated Customer" {
			t.Fatalf("concurrent replay %d returned %+v", index, results[index])
		}
	}

	transitions := writer.transitionSnapshot()
	if len(transitions) != 1 {
		t.Fatalf("audit transitions=%d, want exactly one", len(transitions))
	}
	encoded, err := json.Marshal(transitions[0])
	if err != nil {
		t.Fatalf("marshal audit transition: %v", err)
	}
	if strings.Contains(string(encoded), "Updated Customer") ||
		strings.Contains(string(encoded), "Customer Before") {
		t.Fatalf("audit evidence contains raw display name: %s", encoded)
	}
	assertCustomerIntegrationCount(t, ctx, dao.IdempotencyRecord.Table(), "scope", idempotencyScopeUpdateOwn, 1)

	var subject *entity.CustomerSubject
	if err = dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: testCustomerNo,
	}).Scan(&subject); err != nil {
		t.Fatalf("load updated subject: %v", err)
	}
	if subject == nil || subject.Version != 2 || subject.DisplayName != "Updated Customer" ||
		subject.SearchName != "updated customer" || subject.UpdatedBy != 81 {
		t.Fatalf("unexpected updated subject: %+v", subject)
	}
}

func TestDAOUpdateOwnBasicsRollsBackWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	cause := errors.New("audit unavailable")
	persistence := &daoPersistence{audit: &transactionalAuditWriter{err: cause}, now: time.Now}
	input, err := normalizeUpdate(UpdateOwnBasicsInput{
		CustomerNo: testCustomerNo, DisplayName: "Must Roll Back", CountryCode: "US",
		PreferredLocale: "en-US", ExpectedVersion: 1, IdempotencyKey: "rollback-request",
	})
	if err != nil {
		t.Fatalf("normalize rollback input: %v", err)
	}
	if _, err = persistence.updateOwnBasics(ctx, 81, input); !errors.Is(err, cause) {
		t.Fatalf("expected retained audit failure, got %v", err)
	}

	var subject *entity.CustomerSubject
	if err = dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: testCustomerNo,
	}).Scan(&subject); err != nil {
		t.Fatalf("load rolled-back subject: %v", err)
	}
	if subject == nil || subject.Version != 1 || subject.DisplayName != "Customer Before" {
		t.Fatalf("customer update escaped rollback: %+v", subject)
	}
	assertCustomerIntegrationCount(t, ctx, dao.IdempotencyRecord.Table(), "scope", idempotencyScopeUpdateOwn, 0)
}

func TestDAOUpdateOwnBasicsRejectsChangedSemanticsUnderSameKey(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	persistence := &daoPersistence{audit: &transactionalAuditWriter{}, now: time.Now}
	first, err := normalizeUpdate(UpdateOwnBasicsInput{
		CustomerNo: testCustomerNo, DisplayName: "First Update", CountryCode: "CN",
		PreferredLocale: "zh-CN", ExpectedVersion: 1, IdempotencyKey: "same-key",
	})
	if err != nil {
		t.Fatalf("normalize first input: %v", err)
	}
	if _, err = persistence.updateOwnBasics(ctx, 81, first); err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	changed, err := normalizeUpdate(UpdateOwnBasicsInput{
		CustomerNo: testCustomerNo, DisplayName: "Different Update", CountryCode: "CN",
		PreferredLocale: "zh-CN", ExpectedVersion: 2, IdempotencyKey: "same-key",
	})
	if err != nil {
		t.Fatalf("normalize changed input: %v", err)
	}
	if _, err = persistence.updateOwnBasics(ctx, 81, changed); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed semantics returned %v, want idempotency conflict", err)
	}
	profile, err := persistence.load(ctx, testCustomerNo)
	if err != nil || profile == nil || profile.Subject.Version != 2 || profile.Subject.DisplayName != "First Update" {
		t.Fatalf("changed replay mutated profile: profile=%+v err=%v", profile, err)
	}
}

func TestDAOProfileProjectionNeverReturnsRawEnterpriseIdentifier(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeEnterprise, StatusActive)
	rawIdentifier := "91310000RAW-SENSITIVE"
	if _, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: testCustomerNo,
	}).Data(do.CustomerEnterpriseProfile{
		RegistrationIdentifier:       rawIdentifier,
		RegistrationIdentifierMasked: "9131************",
		RegistrationLookupDigest:     strings.Repeat("a", 64),
	}).Update(); err != nil {
		t.Fatalf("seed enterprise identifier: %v", err)
	}
	profile, err := (&daoPersistence{}).load(ctx, testCustomerNo)
	if err != nil || profile == nil || profile.Enterprise == nil {
		t.Fatalf("load enterprise projection: profile=%+v err=%v", profile, err)
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal enterprise projection: %v", err)
	}
	if strings.Contains(string(encoded), rawIdentifier) ||
		profile.Enterprise.RegistrationIdentifierMasked != "9131************" {
		t.Fatalf("unsafe enterprise projection: %s", encoded)
	}
}

func TestCustomerMigrationRejectsEmptyVerifiedProfiles(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	individualNo := "CUS_11111111111111111111111111111111"
	enterpriseNo := "CUS_22222222222222222222222222222222"
	insertCustomerIntegrationProfile(t, ctx, individualNo, TypeIndividual, StatusActive)
	insertCustomerIntegrationProfile(t, ctx, enterpriseNo, TypeEnterprise, StatusActive)
	verifiedAt := time.Now().UTC()

	if _, err := dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: individualNo,
	}).Data(do.CustomerIndividualProfile{
		IdentityStatusCode: string(VerificationVerified),
		IdentityCaseNo:     "CASE_EMPTY_INDIVIDUAL",
		IdentityVerifiedAt: &verifiedAt,
	}).Update(); err == nil {
		t.Fatal("empty individual legal identity must not become verified")
	}
	if _, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: enterpriseNo,
	}).Data(do.CustomerEnterpriseProfile{
		VerificationStatusCode: string(VerificationVerified),
		VerificationCaseNo:     "CASE_EMPTY_ENTERPRISE",
		VerifiedAt:             &verifiedAt,
	}).Update(); err == nil {
		t.Fatal("empty enterprise legal identity must not become verified")
	}

	if _, err := dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: individualNo,
	}).Data(do.CustomerIndividualProfile{
		LegalName:                "Verified Person",
		LegalNameMasked:          "V*** P*****",
		IdentityIdentifierMasked: "110***********001",
		IdentityLookupDigest:     strings.Repeat("a", 64),
		IdentityStatusCode:       string(VerificationVerified),
		IdentityCaseNo:           "CASE_COMPLETE_INDIVIDUAL",
		IdentityVerifiedAt:       &verifiedAt,
	}).Update(); err != nil {
		t.Fatalf("complete individual identity should become verified: %v", err)
	}
	if _, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: enterpriseNo,
	}).Data(do.CustomerEnterpriseProfile{
		LegalName:                    "Verified Enterprise",
		RegistrationIdentifier:       "91310000123456789X",
		RegistrationIdentifierMasked: "9131**********789X",
		RegistrationLookupDigest:     strings.Repeat("b", 64),
		VerificationStatusCode:       string(VerificationVerified),
		VerificationCaseNo:           "CASE_COMPLETE_ENTERPRISE",
		VerifiedAt:                   &verifiedAt,
	}).Update(); err != nil {
		t.Fatalf("complete enterprise identity should become verified: %v", err)
	}
}

func TestGeneratedDecimalProjectionPreservesNumericThirtyNine(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	exact := "123456789012345678901.123456789"
	fieldNo := "FLD_0123456789abcdef0123456789abcdef"
	valueNo := "VAL_0123456789abcdef0123456789abcdef"
	if _, err := dao.CustomerCustomField.Ctx(ctx).Data(do.CustomerCustomField{
		TenantId: idccap.DefaultTenantID, FieldNo: fieldNo, FieldKey: "exact_decimal",
		LabelZhCn: "精确小数", LabelEnUs: "Exact decimal", DataTypeCode: "decimal",
		CustomerTypeCode: "all", VisibilityCode: "admin_only", SensitivityCode: "ordinary",
		MinimumNumber: &exact, StatusCode: "active", Version: 1,
	}).Insert(); err != nil {
		t.Fatalf("insert exact decimal field: %v", err)
	}
	if _, err := dao.CustomerCustomFieldValue.Ctx(ctx).Data(do.CustomerCustomFieldValue{
		TenantId: idccap.DefaultTenantID, ValueNo: valueNo, CustomerNo: testCustomerNo,
		FieldNo: fieldNo, DecimalValue: &exact, StatusCode: "active", Version: 1,
	}).Insert(); err != nil {
		t.Fatalf("insert exact decimal value: %v", err)
	}
	var field *entity.CustomerCustomField
	if err := dao.CustomerCustomField.Ctx(ctx).Where(do.CustomerCustomField{
		TenantId: idccap.DefaultTenantID, FieldNo: fieldNo,
	}).Scan(&field); err != nil {
		t.Fatalf("load exact decimal field: %v", err)
	}
	var value *entity.CustomerCustomFieldValue
	if err := dao.CustomerCustomFieldValue.Ctx(ctx).Where(do.CustomerCustomFieldValue{
		TenantId: idccap.DefaultTenantID, ValueNo: valueNo,
	}).Scan(&value); err != nil {
		t.Fatalf("load exact decimal value: %v", err)
	}
	if field == nil || field.MinimumNumber == nil || *field.MinimumNumber != exact ||
		value == nil || value.DecimalValue == nil || *value.DecimalValue != exact {
		t.Fatalf("numeric projection lost precision: field=%+v value=%+v", field, value)
	}
}

func TestGeneratedDateProjectionPreservesNullAndCalendarDate(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	fieldNo := "FLD_DATE_123456789abcdef0123456789abc"
	valueNo := "VAL_DATE_123456789abcdef0123456789abc"
	if _, err := dao.CustomerCustomField.Ctx(ctx).Data(do.CustomerCustomField{
		TenantId: idccap.DefaultTenantID, FieldNo: fieldNo, FieldKey: "nullable_date",
		LabelZhCn: "可空日期", LabelEnUs: "Nullable date", DataTypeCode: "date",
		CustomerTypeCode: "all", VisibilityCode: "admin_only", SensitivityCode: "ordinary",
		StatusCode: "active", Version: 1,
	}).Insert(); err != nil {
		t.Fatalf("insert nullable date field: %v", err)
	}
	if _, err := dao.CustomerCustomFieldValue.Ctx(ctx).Data(do.CustomerCustomFieldValue{
		TenantId: idccap.DefaultTenantID, ValueNo: valueNo, CustomerNo: testCustomerNo,
		FieldNo: fieldNo, StatusCode: "active", Version: 1,
	}).Insert(); err != nil {
		t.Fatalf("insert null date value: %v", err)
	}

	load := func() *entity.CustomerCustomFieldValue {
		t.Helper()
		var value *entity.CustomerCustomFieldValue
		if err := dao.CustomerCustomFieldValue.Ctx(ctx).Where(do.CustomerCustomFieldValue{
			TenantId: idccap.DefaultTenantID, ValueNo: valueNo,
		}).Scan(&value); err != nil {
			t.Fatalf("load nullable date value: %v", err)
		}
		return value
	}
	if value := load(); value == nil || value.DateValue != nil {
		t.Fatalf("database NULL date projection=%+v, want nil", value)
	}

	exact := time.Date(2040, time.February, 29, 0, 0, 0, 0, time.UTC)
	if _, err := dao.CustomerCustomFieldValue.Ctx(ctx).Where(do.CustomerCustomFieldValue{
		TenantId: idccap.DefaultTenantID, ValueNo: valueNo,
	}).Data(do.CustomerCustomFieldValue{DateValue: &exact}).Update(); err != nil {
		t.Fatalf("update exact calendar date: %v", err)
	}
	value := load()
	if value == nil || value.DateValue == nil ||
		value.DateValue.Format(time.DateOnly) != exact.Format(time.DateOnly) {
		t.Fatalf("calendar date projection=%+v, want %s", value, exact.Format(time.DateOnly))
	}
}

func TestCustomerIdentityDigestIndexesLeadWithDigest(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	indexes := map[string][]string{
		"idx_plugin_idc_core_ind_profile_digest": {"tenant_id", "identity_lookup_digest", "customer_no"},
		"idx_plugin_idc_core_ent_profile_digest": {"tenant_id", "registration_lookup_digest", "customer_no"},
	}
	for name, columns := range indexes {
		definitionValue, err := g.DB().GetValue(ctx,
			"SELECT pg_get_indexdef(indexrelid) FROM pg_index WHERE indexrelid = ?::regclass",
			name,
		)
		if err != nil {
			t.Fatalf("load index %s definition: %v", name, err)
		}
		definition := definitionValue.String()
		positions := make([]int, len(columns))
		for index, column := range columns {
			positions[index] = strings.Index(definition, column)
			if positions[index] < 0 || index > 0 && positions[index] <= positions[index-1] {
				t.Fatalf("index %s has unexpected definition %q", name, definition)
			}
		}
		if !strings.Contains(definition, "WHERE") || !strings.Contains(definition, "<> ''") {
			t.Fatalf("index %s is not partial for nonempty digests: %q", name, definition)
		}
	}
}

func configureCustomerIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(customerIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL customer integration tests", customerIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create customer integration admin database: %v", err)
	}
	schema := newCustomerIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCustomerPostgreSQLIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated customer schema: %v", err)
	}
	originalConfig := gdb.GetAllConfig()
	var scopedDB gdb.DB
	t.Cleanup(func() {
		if scopedDB != nil {
			if closeErr := scopedDB.Close(ctx); closeErr != nil {
				t.Errorf("close scoped customer database: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore customer database config: %v", restoreErr)
		}
		if !strings.HasPrefix(schema, "linapro_customer_test_") {
			t.Errorf("refusing to drop unexpected customer schema %q", schema)
		} else if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCustomerPostgreSQLIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop isolated customer schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close customer admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: appendCustomerDSNParameter(dsn, "search_path="+url.QueryEscape(schema))}},
	}); err != nil {
		t.Fatalf("configure customer integration database: %v", err)
	}
	scopedDB = g.DB()
	if _, err = scopedDB.Exec(ctx, customerDictionaryFixtureSQL); err != nil {
		t.Fatalf("create customer dictionary fixtures: %v", err)
	}
	for _, migrationName := range []string{
		"001-core-foundation.sql", "003-customer-account.sql", "007-customer-account-workflows.sql",
	} {
		migration, readErr := os.ReadFile(customerMigrationPath(t, migrationName))
		if readErr != nil {
			t.Fatalf("read customer migration %s: %v", migrationName, readErr)
		}
		if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute customer migration %s: %v", migrationName, err)
		}
		if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("repeat customer migration %s idempotently: %v", migrationName, err)
		}
	}
	currentSchema, err := scopedDB.GetValue(ctx, "SELECT current_schema()")
	if err != nil || currentSchema.String() != schema {
		t.Fatalf("customer search path=%q want=%q err=%v", currentSchema.String(), schema, err)
	}
}

const customerDictionaryFixtureSQL = `
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
CREATE UNIQUE INDEX uk_customer_test_dict_type
    ON sys_dict_type ("tenant_id", "type");
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
CREATE UNIQUE INDEX uk_customer_test_dict_data
    ON sys_dict_data ("tenant_id", "dict_type", "value");
`

func insertCustomerIntegrationProfile(
	t *testing.T,
	ctx context.Context,
	customerNo string,
	customerType Type,
	status Status,
) {
	t.Helper()
	if _, err := dao.CustomerSubject.Ctx(ctx).Data(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		CustomerTypeCode: string(customerType), StatusCode: string(status),
		DisplayName: "Customer Before", SearchName: "customer before",
		CountryCode: "CN", PreferredLocale: "zh-CN", Version: 1,
		CreatedBy: 81, UpdatedBy: 81,
	}).Insert(); err != nil {
		t.Fatalf("insert customer subject: %v", err)
	}
	switch customerType {
	case TypeIndividual:
		if _, err := dao.CustomerIndividualProfile.Ctx(ctx).Data(do.CustomerIndividualProfile{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
			CustomerTypeCode: string(TypeIndividual), IdentityStatusCode: string(VerificationUnverified),
		}).Insert(); err != nil {
			t.Fatalf("insert individual profile: %v", err)
		}
	case TypeEnterprise:
		if _, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Data(do.CustomerEnterpriseProfile{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
			CustomerTypeCode: string(TypeEnterprise), RegistrationCountryCode: "CN",
			VerificationStatusCode: string(VerificationUnverified),
		}).Insert(); err != nil {
			t.Fatalf("insert enterprise profile: %v", err)
		}
	default:
		t.Fatalf("unsupported customer type %q", customerType)
	}
}

func assertCustomerIntegrationCount(
	t *testing.T,
	ctx context.Context,
	table string,
	column string,
	value any,
	want int,
) {
	t.Helper()
	count, err := g.DB().Model(table).Ctx(ctx).Where(column, value).Count()
	if err != nil || count != want {
		t.Fatalf("%s count=%d want=%d err=%v", table, count, want, err)
	}
}

func newCustomerIntegrationSchema(t *testing.T) string {
	t.Helper()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate customer schema suffix: %v", err)
	}
	return "linapro_customer_test_" + hex.EncodeToString(suffix[:])
}

func quoteCustomerPostgreSQLIdentifier(value string) string {
	for _, character := range value {
		if character == '_' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' {
			continue
		}
		panic(fmt.Sprintf("unsafe PostgreSQL identifier %q", value))
	}
	return `"` + value + `"`
}

func appendCustomerDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func customerMigrationPath(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve customer integration test path")
	}
	return filepath.Clean(filepath.Join(
		filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", name,
	))
}
