package notificationpolicy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type preferenceBizCtxStub struct {
	current bizctxcap.CurrentContext
}

func (s preferenceBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return s.current
}

type preferencePrincipalStub struct {
	audience *principal.Audience
}

func (s preferencePrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

func TestCurrentMarketingConsentReturnsOnlyCurrentPolicyEvidencePostgreSQL(t *testing.T) {
	ctx, database := newPreferenceProjectionTestDatabase(t)

	const (
		customerNo = "CUS_preference_projection"
		contactNo  = "CON_preference_projection"
	)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,customer_type_code,status_code,display_name,preferred_locale) VALUES (0,?,'individual','active','Preference projection','zh-CN')`, customerNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact (tenant_id,contact_no,customer_no,display_name,email,email_lookup_digest,email_verified_at,status_code) VALUES (0,?,?,?,'customer@example.test',?,CURRENT_TIMESTAMP,'active')`, contactNo, customerNo, "Preference contact", strings.Repeat("1", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_notification_preference_profile (tenant_id,customer_no,version) VALUES (0,?,7)`, customerNo); err != nil {
		t.Fatal(err)
	}
	insertConsentEvent := func(eventNo, action, reference, version, locale, digest string) {
		t.Helper()
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_channel_consent_event (tenant_id,event_no,customer_no,contact_no,channel_code,action_code,policy_reference,policy_version,locale,source_code,evidence_digest,idempotency_key_digest,request_fingerprint) VALUES (0,?,?,?,'email',?,?,?,?, 'notify',?,?,?)`, eventNo, customerNo, contactNo, action, reference, version, locale, digest, digest, digest); err != nil {
			t.Fatal(err)
		}
	}
	insertConsentEvent("MCE_preference_stale", "granted", "policy/stale", "v1", "zh-CN", strings.Repeat("2", 64))
	insertConsentEvent("MCE_preference_current", "withdrawn", "policy/current", "v2", "en-US", strings.Repeat("3", 64))
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_channel_consent (tenant_id,customer_no,contact_no,channel_code,status_code,current_event_no,consent_occurred_at,version) VALUES (0,?,?,'email','opted_out','MCE_preference_current',CURRENT_TIMESTAMP,2)`, customerNo, contactNo); err != nil {
		t.Fatal(err)
	}

	service, err := NewPreferenceCommander(
		database,
		preferenceBizCtxStub{current: bizctxcap.CurrentContext{UserID: 41, TenantID: 0}},
		preferencePrincipalStub{audience: &principal.Audience{
			UserID: 41, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
			CustomerNo: customerNo, Version: 3,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Current(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.MarketingConsents) != 1 {
		t.Fatalf("unexpected marketing consent count: %d", len(snapshot.MarketingConsents))
	}
	consent := snapshot.MarketingConsents[0]
	if consent.PolicyReference != "policy/current" || consent.PolicyVersion != "v2" || consent.Locale != "en-US" || consent.Status != "opted_out" || consent.Version != 2 {
		t.Fatalf("current authoritative consent evidence was not projected: %+v", consent)
	}
}

func TestPendingActivationContactIsAvailableOnlyForActiveSecurityInvitationPostgreSQL(t *testing.T) {
	ctx, database := newPreferenceProjectionTestDatabase(t)
	const (
		customerNo = "CUS_pending_activation"
		contactNo  = "CON_pending_activation"
	)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,customer_type_code,status_code,display_name,preferred_locale) VALUES (0,?,'individual','pending_activation','Pending activation','zh-CN')`, customerNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact (tenant_id,contact_no,customer_no,display_name,email,email_lookup_digest,status_code,preferred_locale,version) VALUES (0,?,?,?,'pending@example.test',?,'active','zh-CN',1)`, contactNo, customerNo, "Pending contact", strings.Repeat("5", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_activation_invitation (tenant_id,invitation_no,customer_no,contact_no,status_code,expires_at) VALUES (0,'INV_pending_activation',?,?,'active',CURRENT_TIMESTAMP + INTERVAL '1 hour')`, customerNo, contactNo); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewContactResolver(database)
	if err != nil {
		t.Fatal(err)
	}
	var destination string
	err = resolver.ResolveBatch(ctx, []notificationcap.ContactRequest{{CustomerNo: customerNo, ContactNo: contactNo, Channel: notificationcap.ChannelEmail, Topic: notificationcap.TopicSecurity}}, func(_ int, _ notificationcap.ContactProjection, raw []byte) error {
		destination = string(raw)
		return nil
	})
	if err != nil || destination != "pending@example.test" {
		t.Fatalf("destination=%q err=%v", destination, err)
	}
	if err = resolver.ResolveBatch(ctx, []notificationcap.ContactRequest{{CustomerNo: customerNo, ContactNo: contactNo, Channel: notificationcap.ChannelEmail, Topic: notificationcap.TopicSupport}}, func(int, notificationcap.ContactProjection, []byte) error { return nil }); err == nil {
		t.Fatal("pending unverified contact unexpectedly resolved for a non-security notification")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_customer_activation_invitation SET status_code='expired' WHERE invitation_no='INV_pending_activation'`); err != nil {
		t.Fatal(err)
	}
	if err = resolver.ResolveBatch(ctx, []notificationcap.ContactRequest{{CustomerNo: customerNo, ContactNo: contactNo, Channel: notificationcap.ChannelEmail, Topic: notificationcap.TopicSecurity}}, func(int, notificationcap.ContactProjection, []byte) error { return nil }); err == nil {
		t.Fatal("pending unverified contact unexpectedly resolved after invitation expiry")
	}
}

func newPreferenceProjectionTestDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run notification preference PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_notification_pref_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close preference projection database: %v", closeErr)
		}
		if strings.HasPrefix(schema, "linapro_notification_pref_") {
			if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
				t.Errorf("drop preference projection schema: %v", dropErr)
			}
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close preference projection admin database: %v", closeErr)
		}
	})
	databaseValue, err := database.GetValue(ctx, `SELECT current_database()`)
	if err != nil {
		t.Fatal(err)
	}
	searchPathValue, err := database.GetValue(ctx, `SHOW search_path`)
	if err != nil {
		t.Fatal(err)
	}
	databaseName, searchPath := databaseValue.String(), searchPathValue.String()
	if databaseName == "" || searchPath != schema {
		t.Fatalf("unexpected isolated database target: database=%q search_path=%q", databaseName, searchPath)
	}
	if _, err = database.Exec(ctx, `
		CREATE TABLE plugin_idc_core_customer_subject (
			customer_no VARCHAR(40) PRIMARY KEY,
			tenant_id INT NOT NULL,
			customer_type_code VARCHAR(16) NOT NULL,
			status_code VARCHAR(32) NOT NULL,
			display_name VARCHAR(256) NOT NULL,
			preferred_locale VARCHAR(16) NOT NULL
		);
		CREATE TABLE plugin_idc_core_customer_contact (
			contact_no VARCHAR(40) PRIMARY KEY,
			tenant_id INT NOT NULL,
			customer_no VARCHAR(40) NOT NULL,
			display_name VARCHAR(256) NOT NULL,
			email VARCHAR(320) NOT NULL,
			email_lookup_digest CHAR(64) NOT NULL,
			email_verified_at TIMESTAMPTZ,
			phone_country_code VARCHAR(8) NOT NULL DEFAULT '',
			phone VARCHAR(32) NOT NULL DEFAULT '',
			phone_verified_at TIMESTAMPTZ,
			preferred_locale VARCHAR(16) NOT NULL DEFAULT 'zh-CN',
			version BIGINT NOT NULL DEFAULT 1,
			status_code VARCHAR(16) NOT NULL
		);
		CREATE TABLE plugin_idc_core_customer_identity_binding (
			tenant_id INT NOT NULL,
			customer_no VARCHAR(40) NOT NULL,
			sys_user_id BIGINT NOT NULL,
			status_code VARCHAR(16) NOT NULL
		);
		CREATE TABLE plugin_idc_core_customer_activation_invitation (
			tenant_id INT NOT NULL,
			invitation_no VARCHAR(48) PRIMARY KEY,
			customer_no VARCHAR(40) NOT NULL,
			contact_no VARCHAR(40) NOT NULL,
			status_code VARCHAR(16) NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE plugin_idc_core_customer_contact_preference (
			id BIGSERIAL PRIMARY KEY,
			tenant_id INT NOT NULL,
			customer_no VARCHAR(40) NOT NULL,
			contact_no VARCHAR(40) NOT NULL,
			topic_code VARCHAR(32) NOT NULL,
			channel_code VARCHAR(24) NOT NULL,
			enabled BOOLEAN NOT NULL,
			version BIGINT NOT NULL
		);
		CREATE TABLE plugin_idc_core_notification_preference_profile (
			tenant_id BIGINT NOT NULL,
			customer_no VARCHAR(40) PRIMARY KEY,
			version BIGINT NOT NULL
		);
		CREATE TABLE plugin_idc_core_marketing_channel_consent_event (
			tenant_id INT NOT NULL,
			event_no VARCHAR(40) PRIMARY KEY,
			customer_no VARCHAR(40) NOT NULL,
			contact_no VARCHAR(40) NOT NULL,
			channel_code VARCHAR(24) NOT NULL,
			action_code VARCHAR(16) NOT NULL,
			policy_reference VARCHAR(256) NOT NULL,
			policy_version VARCHAR(64) NOT NULL,
			locale VARCHAR(16) NOT NULL,
			source_code VARCHAR(24) NOT NULL,
			evidence_digest CHAR(64) NOT NULL,
			idempotency_key_digest CHAR(64) NOT NULL,
			request_fingerprint CHAR(64) NOT NULL
		);
		CREATE TABLE plugin_idc_core_marketing_channel_consent (
			tenant_id INT NOT NULL,
			customer_no VARCHAR(40) NOT NULL,
			contact_no VARCHAR(40) NOT NULL,
			channel_code VARCHAR(24) NOT NULL,
			status_code VARCHAR(16) NOT NULL,
			current_event_no VARCHAR(40) NOT NULL,
			consent_occurred_at TIMESTAMPTZ NOT NULL,
			version BIGINT NOT NULL
		);
		CREATE TABLE plugin_idc_core_marketing_quiet_hours (
			tenant_id INT NOT NULL,
			customer_no VARCHAR(40) NOT NULL,
			timezone_name VARCHAR(64) NOT NULL,
			start_minute SMALLINT NOT NULL,
			end_minute SMALLINT NOT NULL,
			enabled BOOLEAN NOT NULL,
			version BIGINT NOT NULL
		)
	`); err != nil {
		t.Fatal(err)
	}
	return ctx, database
}
