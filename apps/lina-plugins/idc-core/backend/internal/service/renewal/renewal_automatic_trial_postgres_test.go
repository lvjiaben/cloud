// This file verifies that trial services never enter automatic paid renewal,
// even when an old or externally created policy has auto-renew enabled.

package renewal

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestTrialCannotEnterAutomaticRenewalPostgreSQL(t *testing.T) {
	ctx, database := newRenewalPostgreSQLDatabase(t)
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Temporary projections retain all production column types while isolating
		// this admission test from payment fixtures and persistent business data.
		for _, table := range []string{
			"plugin_idc_core_customer_subject", "plugin_idc_core_service_entitlement",
			"plugin_idc_core_service_renewal_policy", "plugin_idc_core_service_renewal_attempt",
			"plugin_idc_core_business_operation", "plugin_idc_core_service_price_snapshot",
		} {
			if _, err := database.Exec(ctx, "CREATE TEMP TABLE "+table+" ON COMMIT DROP AS SELECT * FROM "+table+" WITH NO DATA"); err != nil {
				return err
			}
		}
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,status_code) VALUES (0,'CUS_trial','active');
INSERT INTO plugin_idc_core_service_renewal_policy
 (tenant_id,policy_no,entitlement_no,policy_version,auto_renew_enabled,renewal_cycles,lead_seconds,
  maximum_attempts,retry_interval_seconds,failure_notification_enabled,configured_by,content_digest)
 VALUES (0,'RPL_trial','ENT_trial',1,TRUE,1,3600,3,300,TRUE,1,repeat('a',64));
INSERT INTO plugin_idc_core_service_entitlement
 (tenant_id,entitlement_no,customer_no,currency_code,status_code,charge_type_code,period_unit_code,
  period_count,renewal_mode_code,expires_at,version,current_renewal_policy_no,current_renewal_policy_version)
 VALUES (0,'ENT_trial','CUS_trial','CNY','active','trial','day',1,'local_entitlement',
  CURRENT_TIMESTAMP-INTERVAL '1 hour',1,'RPL_trial',1);
`); err != nil {
			return err
		}
		service := &serviceImpl{database: database, principal: &renewalPostgreSQLPrincipal{audience: &principal.Audience{
			Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "CUS_trial", UserID: 1,
		}}}
		for _, enabled := range []bool{true, false} {
			policy, err := service.ConfigurePolicyCurrentCustomer(ctx, PolicyInput{
				EntitlementNo: "ENT_trial", ExpectedEntitlementVersion: 1, AutoRenewEnabled: enabled,
				RenewalCycles: 1, LeadSeconds: 3600, MaximumAttempts: 3, RetryIntervalSeconds: 300,
				FailureNotificationEnabled: true,
			})
			if !bizerr.Is(err, CodeCapabilityUnavailable) || policy != nil {
				t.Fatalf("trial policy enabled=%v accepted: policy=%+v err=%v", enabled, policy, err)
			}
		}
		target, err := loadAutomaticTarget(ctx, database, "ENT_trial")
		if err != nil || target != nil {
			t.Fatalf("trial automatic target=%+v err=%v", target, err)
		}
		attempt, err := service.AttemptDueService(ctx, "ENT_trial")
		if !bizerr.Is(err, CodeConflict) || attempt != nil {
			t.Fatalf("trial automatic attempt=%+v err=%v", attempt, err)
		}
		batch, err := service.RunAutomaticRenewals(ctx, 10)
		if err != nil || batch == nil || batch.Selected != 0 || batch.Processed != 0 {
			t.Fatalf("trial entered automatic batch: batch=%+v err=%v", batch, err)
		}
		count, err := database.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_service_renewal_policy")
		if err != nil || count.Int() != 1 {
			t.Fatalf("trial policy was changed: count=%v err=%v", count, err)
		}
		// The same current policy and due service still work for a paid recurring
		// entitlement. This proves the guard does not disable all automatic targets.
		if _, err := database.Exec(ctx, "UPDATE plugin_idc_core_service_entitlement SET charge_type_code='recurring'"); err != nil {
			return err
		}
		target, err = loadAutomaticTarget(ctx, database, "ENT_trial")
		if err != nil || target == nil || !target.Due || target.CustomerNo != "CUS_trial" {
			t.Fatalf("paid recurring target=%+v err=%v", target, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
