// This file applies one complete explicit Tencent template snapshot without per-row database queries.

package electroniccontracttemplate

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	templateSyncStagingTable          = "idc_compliance_contract_template_sync_input"
	templateComponentSyncStagingTable = "idc_compliance_contract_template_component_sync_input"
	templateRecipientSyncStagingTable = "idc_compliance_contract_template_recipient_sync_input"
)

type syncCounters struct {
	NewVersions   int64 `json:"new_versions"`
	MarkedOffline int64 `json:"marked_offline"`
}

type syncStagingInput struct {
	TemplateNo             string  `orm:"template_no"`
	ProviderTemplateRef    string  `orm:"provider_template_ref"`
	Name                   string  `orm:"name"`
	ProviderVersion        *string `orm:"provider_version"`
	ProviderStateCode      string  `orm:"provider_state_code"`
	ProviderTimeMS         int64   `orm:"provider_time_ms"`
	ProviderSnapshotDigest string  `orm:"provider_snapshot_digest"`
	CustomerSignatureCount int     `orm:"customer_signature_count"`
	CustomerSealCount      int     `orm:"customer_seal_count"`
	MerchantSignatureCount int     `orm:"merchant_signature_count"`
	MerchantSealCount      int     `orm:"merchant_seal_count"`
}

type componentSyncStagingInput struct {
	ProviderTemplateRef string  `orm:"provider_template_ref"`
	ComponentRef        string  `orm:"component_ref"`
	Name                string  `orm:"name"`
	TypeCode            string  `orm:"type_code"`
	Required            bool    `orm:"required"`
	RecipientRef        *string `orm:"recipient_ref"`
}

type recipientSyncStagingInput struct {
	ProviderTemplateRef string `orm:"provider_template_ref"`
	RecipientRef        string `orm:"recipient_ref"`
	SequenceNo          int    `orm:"sequence_no"`
	TypeCode            string `orm:"type_code"`
	RoleName            string `orm:"role_name"`
	RequireSign         bool   `orm:"require_sign"`
	RoutingOrder        *int64 `orm:"routing_order"`
	IsPromoter          bool   `orm:"is_promoter"`
	SignComponentCount  int    `orm:"sign_component_count"`
}

// SyncTencent reads Tencent only after the explicit management action and commits one complete snapshot.
func (s *serviceImpl) SyncTencent(ctx context.Context) (*SyncResult, error) {
	if _, err := s.authorize(ctx, PermissionManage, templateDirectoryTarget); err != nil {
		return nil, err
	}
	snapshot, err := s.gateway.TencentTemplates(ctx)
	if err != nil {
		return nil, bizerr.NewCode(CodeProviderUnavailable)
	}
	seen := make(map[string]struct{}, len(snapshot.Items))
	staging := make([]syncStagingInput, 0, len(snapshot.Items))
	componentStaging := make([]componentSyncStagingInput, 0)
	recipientStaging := make([]recipientSyncStagingInput, 0)
	for _, item := range snapshot.Items {
		if !validProviderItem(item) {
			return nil, bizerr.NewCode(CodeProviderUnavailable)
		}
		if _, exists := seen[item.TemplateRef]; exists {
			return nil, bizerr.NewCode(CodeProviderUnavailable)
		}
		seen[item.TemplateRef] = struct{}{}
		templateDigest := sha256Hex([]byte("tencent-template-no-v1\x00" + item.TemplateRef))
		staging = append(staging, syncStagingInput{
			TemplateNo:             "ctpl_" + templateDigest[:32],
			ProviderTemplateRef:    item.TemplateRef,
			Name:                   item.Name,
			ProviderVersion:        nullString(item.Version),
			ProviderStateCode:      item.ProviderState,
			ProviderTimeMS:         item.ProviderTimeMS,
			ProviderSnapshotDigest: providerSnapshotDigest(item),
			CustomerSignatureCount: item.CustomerSignatureCount,
			CustomerSealCount:      item.CustomerSealCount,
			MerchantSignatureCount: item.MerchantSignatureCount,
			MerchantSealCount:      item.MerchantSealCount,
		})
		for _, component := range item.Components {
			componentStaging = append(componentStaging, componentSyncStagingInput{
				ProviderTemplateRef: item.TemplateRef, ComponentRef: component.ComponentRef,
				Name: component.Name, TypeCode: component.TypeCode, Required: component.Required,
				RecipientRef: nullString(component.RecipientRef),
			})
		}
		for _, recipient := range item.Recipients {
			recipientStaging = append(recipientStaging, recipientSyncStagingInput{
				ProviderTemplateRef: item.TemplateRef, RecipientRef: recipient.RecipientRef,
				SequenceNo: recipient.SequenceNo,
				TypeCode:   recipient.TypeCode, RoleName: recipient.RoleName,
				RequireSign: recipient.RequireSign, RoutingOrder: recipient.RoutingOrder,
				IsPromoter: recipient.IsPromoter, SignComponentCount: recipient.SignComponentCount,
			})
		}
	}
	syncNo, err := newSyncNo()
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	now := s.now().UTC()
	counters := syncCounters{}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, lockErr := tx.Ctx(ctx).Exec(
			`SELECT pg_advisory_xact_lock(hashtext('idc-compliance:contract-template-sync'))`,
		); lockErr != nil {
			return lockErr
		}
		if _, createErr := tx.Ctx(ctx).Exec(`
CREATE TEMP TABLE idc_compliance_contract_template_sync_input (
    template_no VARCHAR(64) NOT NULL,
    provider_template_ref VARCHAR(256) NOT NULL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    provider_version VARCHAR(32),
    provider_state_code VARCHAR(24) NOT NULL,
    provider_time_ms BIGINT NOT NULL,
    provider_snapshot_digest CHAR(64) NOT NULL UNIQUE,
    customer_signature_count INT NOT NULL,
    customer_seal_count INT NOT NULL,
    merchant_signature_count INT NOT NULL,
    merchant_seal_count INT NOT NULL
) ON COMMIT DROP`); createErr != nil {
			return createErr
		}
		if _, createErr := tx.Ctx(ctx).Exec(`
CREATE TEMP TABLE idc_compliance_contract_template_component_sync_input (
    provider_template_ref VARCHAR(256) NOT NULL,
    component_ref VARCHAR(128) NOT NULL,
    name VARCHAR(100) NOT NULL,
    type_code VARCHAR(64) NOT NULL,
    required BOOLEAN NOT NULL,
    recipient_ref VARCHAR(128),
    PRIMARY KEY (provider_template_ref, component_ref)
) ON COMMIT DROP`); createErr != nil {
			return createErr
		}
		if _, createErr := tx.Ctx(ctx).Exec(`
CREATE TEMP TABLE idc_compliance_contract_template_recipient_sync_input (
    provider_template_ref VARCHAR(256) NOT NULL,
    recipient_ref VARCHAR(128) NOT NULL,
    sequence_no INT NOT NULL,
    type_code VARCHAR(16) NOT NULL,
    role_name VARCHAR(100) NOT NULL,
    require_sign BOOLEAN NOT NULL,
    routing_order BIGINT,
    is_promoter BOOLEAN NOT NULL,
    sign_component_count INT NOT NULL,
    PRIMARY KEY (provider_template_ref, recipient_ref)
) ON COMMIT DROP`); createErr != nil {
			return createErr
		}
		if len(staging) > 0 {
			if _, insertErr := tx.Ctx(ctx).Model(templateSyncStagingTable).
				Data(staging).Batch(500).Insert(); insertErr != nil {
				return insertErr
			}
		}
		if len(componentStaging) > 0 {
			if _, insertErr := tx.Ctx(ctx).Model(templateComponentSyncStagingTable).
				Data(componentStaging).Batch(500).Insert(); insertErr != nil {
				return insertErr
			}
		}
		if len(recipientStaging) > 0 {
			if _, insertErr := tx.Ctx(ctx).Model(templateRecipientSyncStagingTable).
				Data(recipientStaging).Batch(500).Insert(); insertErr != nil {
				return insertErr
			}
		}
		if _, upsertErr := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_contract_template
    (tenant_id, template_no, origin_code, provider_code, provider_template_ref,
     provider_state_code, provider_last_seen_sync_no, provider_updated_at)
SELECT ?, input.template_no, 'tencent_synced', 'tencent_essbasic',
       input.provider_template_ref, input.provider_state_code, ?, ?
FROM idc_compliance_contract_template_sync_input AS input
ON CONFLICT (provider_code, provider_template_ref) DO UPDATE
SET provider_state_code = EXCLUDED.provider_state_code,
    provider_last_seen_sync_no = EXCLUDED.provider_last_seen_sync_no,
    provider_updated_at = EXCLUDED.provider_updated_at`,
			idccap.DefaultTenantID, syncNo, now); upsertErr != nil {
			return upsertErr
		}
		// A newly observed provider snapshot becomes a separate local immutable
		// version and stops the old provider version from being used for new contracts.
		if _, disableErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version AS version
SET status_code = 'disabled', lock_version = lock_version + 1,
    updated_at = CURRENT_TIMESTAMP
FROM plugin_idc_compliance_contract_template AS template
JOIN idc_compliance_contract_template_sync_input AS input
  ON input.provider_template_ref = template.provider_template_ref
WHERE version.tenant_id = ?
  AND version.tenant_id = template.tenant_id
  AND version.template_no = template.template_no
  AND version.status_code = 'enabled'
  AND version.provider_snapshot_digest IS DISTINCT FROM input.provider_snapshot_digest`,
			idccap.DefaultTenantID); disableErr != nil {
			return disableErr
		}
		insertResult, insertErr := tx.Ctx(ctx).Exec(`
WITH candidates AS (
    SELECT template.template_no, input.name, input.provider_version,
           input.provider_snapshot_digest, input.customer_signature_count,
           input.customer_seal_count, input.merchant_signature_count,
           input.merchant_seal_count,
           COALESCE(MAX(version.version_no), 0) + 1 AS next_version_no
    FROM idc_compliance_contract_template_sync_input AS input
    JOIN plugin_idc_compliance_contract_template AS template
      ON template.tenant_id = ?
     AND template.provider_code = 'tencent_essbasic'
     AND template.provider_template_ref = input.provider_template_ref
    LEFT JOIN plugin_idc_compliance_contract_template_version AS version
      ON version.tenant_id = template.tenant_id AND version.template_no = template.template_no
    WHERE NOT EXISTS (
        SELECT 1
        FROM plugin_idc_compliance_contract_template_version AS existing
        WHERE existing.tenant_id = template.tenant_id
          AND existing.template_no = template.template_no
          AND existing.provider_snapshot_digest = input.provider_snapshot_digest
    )
    GROUP BY template.template_no, input.name, input.provider_version,
             input.provider_snapshot_digest, input.customer_signature_count,
             input.customer_seal_count, input.merchant_signature_count,
             input.merchant_seal_count
)
INSERT INTO plugin_idc_compliance_contract_template_version
    (tenant_id, template_no, version_no, name, scope_code, signing_order_code, language_code,
     status_code, provider_version, provider_snapshot_digest,
     provider_customer_signature_count, provider_customer_seal_count,
     provider_merchant_signature_count, provider_merchant_seal_count, lock_version)
SELECT ?, candidate.template_no, candidate.next_version_no, candidate.name,
	   NULL AS scope_code, NULL AS signing_order_code, NULL AS language_code,
       'needs_configuration', candidate.provider_version,
       candidate.provider_snapshot_digest, candidate.customer_signature_count,
       candidate.customer_seal_count, candidate.merchant_signature_count,
       candidate.merchant_seal_count, 1
FROM candidates AS candidate`, idccap.DefaultTenantID, idccap.DefaultTenantID)
		if insertErr != nil {
			return insertErr
		}
		newVersions, insertErr := insertResult.RowsAffected()
		if insertErr != nil {
			return insertErr
		}
		counters.NewVersions = newVersions
		if _, projectionErr := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_ctpl_provider_recipient
    (tenant_id, template_no, version_no, recipient_ref, sequence_no, type_code, role_name,
     require_sign, routing_order, is_promoter, sign_component_count)
SELECT ?, template.template_no, version.version_no, recipient.recipient_ref,
       recipient.sequence_no, recipient.type_code, recipient.role_name, recipient.require_sign,
       recipient.routing_order, recipient.is_promoter, recipient.sign_component_count
FROM idc_compliance_contract_template_recipient_sync_input AS recipient
JOIN idc_compliance_contract_template_sync_input AS input
  ON input.provider_template_ref = recipient.provider_template_ref
JOIN plugin_idc_compliance_contract_template AS template
  ON template.tenant_id = ? AND template.provider_code = 'tencent_essbasic'
 AND template.provider_template_ref = input.provider_template_ref
JOIN plugin_idc_compliance_contract_template_version AS version
  ON version.tenant_id = template.tenant_id AND version.template_no = template.template_no
 AND version.provider_snapshot_digest = input.provider_snapshot_digest
ON CONFLICT (template_no, version_no, recipient_ref) DO NOTHING`,
			idccap.DefaultTenantID, idccap.DefaultTenantID); projectionErr != nil {
			return projectionErr
		}
		if _, projectionErr := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_ctpl_provider_component
    (tenant_id, template_no, version_no, component_ref, name, type_code, required, recipient_ref)
SELECT ?, template.template_no, version.version_no, component.component_ref,
       component.name, component.type_code, component.required, component.recipient_ref
FROM idc_compliance_contract_template_component_sync_input AS component
JOIN idc_compliance_contract_template_sync_input AS input
  ON input.provider_template_ref = component.provider_template_ref
JOIN plugin_idc_compliance_contract_template AS template
  ON template.tenant_id = ? AND template.provider_code = 'tencent_essbasic'
 AND template.provider_template_ref = input.provider_template_ref
JOIN plugin_idc_compliance_contract_template_version AS version
  ON version.tenant_id = template.tenant_id AND version.template_no = template.template_no
 AND version.provider_snapshot_digest = input.provider_snapshot_digest
ON CONFLICT (template_no, version_no, component_ref) DO NOTHING`,
			idccap.DefaultTenantID, idccap.DefaultTenantID); projectionErr != nil {
			return projectionErr
		}
		offlineResult, offlineErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template
SET provider_state_code = 'offline', provider_updated_at = ?
WHERE tenant_id = ? AND provider_code = 'tencent_essbasic'
  AND provider_last_seen_sync_no IS DISTINCT FROM ?
  AND provider_state_code <> 'offline'`, now, idccap.DefaultTenantID, syncNo)
		if offlineErr != nil {
			return offlineErr
		}
		markedOffline, offlineErr := offlineResult.RowsAffected()
		if offlineErr != nil {
			return offlineErr
		}
		counters.MarkedOffline = markedOffline
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.sync",
			IdempotencyKey:   syncNo, OperationCode: "compliance.contract-template.sync",
			TargetType: "electronic_contract_template_directory", TargetNo: templateDirectoryTarget,
			ResultCode: "succeeded", ReasonCode: "complete_provider_snapshot_applied",
			RiskCode: auditRiskHigh, RequiredPermission: PermissionManage,
			BeforeSnapshot: map[string]any{"provider": "tencent_essbasic"},
			AfterSnapshot: map[string]any{
				"provider_total": len(snapshot.Items), "new_versions": newVersions,
				"marked_offline": markedOffline,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return &SyncResult{
		ProviderTotal: len(snapshot.Items), NewVersions: int(counters.NewVersions),
		MarkedOffline: int(counters.MarkedOffline), SyncedAt: now,
	}, nil
}

func newSyncNo() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "cts_" + hex.EncodeToString(value), nil
}

func nullString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
