package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestMultiProviderSMSMigrationPostgreSQLIsIdempotentAndBackfillsTencentTemplates(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(notificationIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL notification migration tests", notificationIntegrationDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := newNotificationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteNotificationIdentifier(schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteNotificationIdentifier(schema)+" CASCADE")
		_ = admin.Close(ctx)
	}()
	database, err := gdb.New(gdb.ConfigNode{Link: appendNotificationDSN(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped PostgreSQL: %v", err)
	}
	defer database.Close(ctx)

	if _, err = database.Exec(ctx, readNotificationSQL(t, filepath.Join("manifest", "sql", "001-notification-foundation.sql"))); err != nil {
		t.Fatalf("apply notification foundation: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_notify_template_version
 (tenant_id,template_no,version,event_code,channel_code,locale_code,body_template,
  provider_template_id,provider_sms_variable_order,provider_wechat_data_mapping,status_code,content_digest,created_by)
 VALUES (0,'NTF_SMS_OLD_ZH',1,'service.expiry_reminder','sms','zh-CN','old body',
  '2299001','["service_no","expires_at"]'::jsonb,'{}'::jsonb,'active',repeat('a',64),1)`); err != nil {
		t.Fatalf("insert pre-migration Tencent template: %v", err)
	}

	migration := readNotificationSQL(t, filepath.Join("manifest", "sql", "006-multi-provider-sms.sql"))
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, migration); err != nil {
			t.Fatalf("apply multi-provider SMS migration attempt %d: %v", attempt, err)
		}
	}
	provider, err := database.GetValue(ctx, `SELECT provider_code
 FROM plugin_idc_notify_template_version
 WHERE tenant_id=0 AND template_no='NTF_SMS_OLD_ZH' AND version=1`)
	if err != nil || provider.String() != "tencent_cloud_sms_v20210111" {
		t.Fatalf("old Tencent template was not bound to its provider: provider=%q err=%v", provider.String(), err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_notify_template_version
 SET provider_code='unsupported_sms' WHERE tenant_id=0 AND template_no='NTF_SMS_OLD_ZH'`); err == nil {
		t.Fatal("SMS template accepted an unsupported provider")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_notify_template_version
 SET provider_sms_data_mapping='[]'::jsonb WHERE tenant_id=0 AND template_no='NTF_SMS_OLD_ZH'`); err == nil {
		t.Fatal("SMS named parameter mapping accepted a non-object value")
	}
}
