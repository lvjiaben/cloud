-- Phase 7.8: core-owned online refund requests, immutable impact snapshots,
-- approval evidence, source allocations, service-operation locks, and
-- payment-owner execution commands.

-- Purpose: Immutable per-currency refund approval and recent-login threshold versions.
-- 用途：保存退款策略版本相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_policy_version (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "high_amount_threshold_minor" BIGINT NOT NULL,
    "automatic_approval_enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "content_digest" CHAR(64) NOT NULL,
    "configured_by" INT NOT NULL,
    "effective_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_refund_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_policy_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_refund_policy_version CHECK ("policy_version" > 0),
    CONSTRAINT ck_idc_refund_policy_threshold CHECK ("high_amount_threshold_minor" > 0),
    CONSTRAINT ck_idc_refund_policy_actor CHECK ("configured_by" > 0),
    CONSTRAINT ck_idc_refund_policy_digest CHECK ("content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT uk_idc_refund_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_refund_policy_currency_version UNIQUE ("currency_code", "policy_version"),
    CONSTRAINT uk_idc_refund_policy_scope UNIQUE (
        "policy_no", "currency_code", "policy_version"
    )
);

CREATE INDEX IF NOT EXISTS idx_idc_refund_policy_effective
    ON plugin_idc_core_refund_policy_version (
        "tenant_id", "currency_code", "effective_at" DESC, "policy_version" DESC
    );

-- Purpose: Core-owned online refund request and current workflow projection.
-- 用途：保存退款申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "refund_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(32) NOT NULL,
    "reason_detail" VARCHAR(1000) NOT NULL DEFAULT '',
    "requested_amount_minor" BIGINT NOT NULL,
    "approval_mode_code" VARCHAR(16) NOT NULL,
    "refund_destination_code" VARCHAR(24) NOT NULL DEFAULT 'original_channel',
    "requires_service_termination" BOOLEAN NOT NULL DEFAULT FALSE,
    "has_resource_impact" BOOLEAN NOT NULL DEFAULT FALSE,
    "has_dangerous_operation" BOOLEAN NOT NULL DEFAULT FALSE,
    "has_fund_anomaly" BOOLEAN NOT NULL DEFAULT FALSE,
    "high_amount" BOOLEAN NOT NULL DEFAULT FALSE,
    "policy_no" VARCHAR(40),
    "policy_version" BIGINT,
    "impact_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "submitted_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "approved_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_refund_request_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_request_status CHECK (
        "status_code" IN (
            'pending_review', 'approved', 'awaiting_balance_consent',
            'executing', 'succeeded', 'failed', 'result_unknown',
            'manual_confirmation', 'rejected'
        )
    ),
    CONSTRAINT ck_idc_refund_request_reason CHECK (
        "reason_code" IN (
            'duplicate_purchase', 'wrong_configuration', 'service_issue',
            'no_longer_needed', 'other'
        )
        AND (
            ("reason_code" = 'other' AND length(btrim("reason_detail")) BETWEEN 1 AND 1000)
            OR ("reason_code" <> 'other' AND length("reason_detail") <= 1000)
        )
    ),
    CONSTRAINT ck_idc_refund_request_amount CHECK ("requested_amount_minor" > 0),
    CONSTRAINT ck_idc_refund_request_approval CHECK (
        "approval_mode_code" IN ('automatic', 'manual')
        AND (
            ("approval_mode_code" = 'automatic'
                AND NOT "has_resource_impact"
                AND NOT "has_dangerous_operation"
                AND NOT "has_fund_anomaly")
            OR "approval_mode_code" = 'manual'
        )
    ),
    CONSTRAINT ck_idc_refund_request_destination CHECK (
        "refund_destination_code" IN (
            'original_channel', 'same_currency_balance', 'mixed'
        )
        AND (
            "refund_destination_code" = 'original_channel'
            OR "status_code" IN (
                'approved', 'executing', 'succeeded', 'failed',
                'result_unknown', 'manual_confirmation'
            )
        )
    ),
    CONSTRAINT ck_idc_refund_request_policy CHECK (
        ("policy_no" IS NULL AND "policy_version" IS NULL
            AND "approval_mode_code" = 'manual' AND "high_amount")
        OR ("policy_no" IS NOT NULL AND "policy_version" > 0)
    ),
    CONSTRAINT ck_idc_refund_request_digest CHECK (
        "impact_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_refund_request_version CHECK ("version" > 0),
    CONSTRAINT ck_idc_refund_request_lifecycle CHECK (
        ("status_code" = 'pending_review' AND "approved_at" IS NULL AND "completed_at" IS NULL)
        OR ("status_code" IN ('approved', 'awaiting_balance_consent', 'executing',
                'result_unknown', 'manual_confirmation')
            AND "approved_at" IS NOT NULL AND "completed_at" IS NULL)
        OR ("status_code" IN ('succeeded', 'failed', 'rejected')
            AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_refund_request_no UNIQUE ("refund_no"),
    CONSTRAINT uk_idc_refund_request_owner UNIQUE (
        "refund_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_refund_request_invoice UNIQUE (
        "refund_no", "invoice_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_refund_request_idempotency UNIQUE (
        "customer_no", "idempotency_digest"
    ),
    CONSTRAINT fk_idc_refund_request_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_request_invoice FOREIGN KEY (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_request_policy FOREIGN KEY (
        "policy_no", "currency_code", "policy_version"
    ) REFERENCES plugin_idc_core_refund_policy_version (
        "policy_no", "currency_code", "policy_version"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_refund_request_customer
    ON plugin_idc_core_refund_request (
        "tenant_id", "customer_no", "submitted_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_refund_request_review
    ON plugin_idc_core_refund_request (
        "tenant_id", "status_code", "submitted_at", "id"
    ) WHERE "status_code" IN ('pending_review', 'awaiting_balance_consent', 'result_unknown', 'manual_confirmation');

-- Purpose: Immutable item or server refund amount and customer-confirmed impact snapshot.
-- 用途：保存退款申请明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_request_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "refund_item_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "line_number" INT NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "invoice_item_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "target_type_code" VARCHAR(16) NOT NULL,
    "service_no" VARCHAR(40),
    "requested_amount_minor" BIGINT NOT NULL,
    "refundable_amount_minor_snapshot" BIGINT NOT NULL,
    "impact_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_refund_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_item_line CHECK ("line_number" BETWEEN 1 AND 1000),
    CONSTRAINT ck_idc_refund_item_amount CHECK (
        "requested_amount_minor" > 0
        AND "requested_amount_minor" <= "refundable_amount_minor_snapshot"
    ),
    CONSTRAINT ck_idc_refund_item_target CHECK (
        ("target_type_code" = 'order_item' AND "service_no" IS NULL)
        OR ("target_type_code" = 'service' AND "service_no" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_refund_item_digest CHECK ("impact_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT uk_idc_refund_item_no UNIQUE ("refund_item_no"),
    CONSTRAINT uk_idc_refund_item_line UNIQUE ("refund_no", "line_number"),
    CONSTRAINT uk_idc_refund_item_scope UNIQUE (
        "refund_no", "refund_item_no", "invoice_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_refund_item_request FOREIGN KEY (
        "refund_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_refund_request (
        "refund_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_item_invoice_item FOREIGN KEY (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_item (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_item_service FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

-- Plugin upgrades replay the full SQL manifest. A database that has already
-- applied migration 087 keeps the entitlement-shaped column instead. Restore
-- this legacy projection only for the lifetime of the replay so the functions
-- below can be created; migration 087 removes it again.
ALTER TABLE plugin_idc_core_refund_request
    ADD COLUMN IF NOT EXISTS "requires_service_termination" BOOLEAN NOT NULL DEFAULT FALSE;

-- Older development databases may already contain the refund item table from
-- an earlier draft that did not include service_no. CREATE TABLE IF NOT EXISTS
-- does not reconcile columns, so add the nullable projection before indexes,
-- constraints, and runtime queries reference it. This is safe to replay.
ALTER TABLE plugin_idc_core_refund_request_item
    ADD COLUMN IF NOT EXISTS "target_type_code" VARCHAR(16) NOT NULL DEFAULT 'order_item',
    ADD COLUMN IF NOT EXISTS "service_no" VARCHAR(40);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_refund_item_target
    ON plugin_idc_core_refund_request_item (
        "tenant_id", "refund_no", "invoice_item_no", COALESCE("service_no", '')
    );

CREATE INDEX IF NOT EXISTS idx_idc_refund_item_request
    ON plugin_idc_core_refund_request_item (
        "tenant_id", "refund_no", "line_number", "id"
    );

-- Purpose: Per-server immutable downtime, data deletion, termination, and version snapshot.
-- 用途：保存退款服务影响相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_service_impact (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "impact_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "refund_item_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "service_version_snapshot" BIGINT NOT NULL,
    "entitlement_status_snapshot" VARCHAR(24) NOT NULL,
    "runtime_status_snapshot" VARCHAR(24) NOT NULL,
    "provider_task_status_snapshot" VARCHAR(24) NOT NULL,
    "requires_service_termination" BOOLEAN NOT NULL,
    "causes_downtime" BOOLEAN NOT NULL,
    "causes_data_deletion" BOOLEAN NOT NULL,
    "impact_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT pk_idc_refund_service_impact_legacy_v1 PRIMARY KEY ("id"),
    CONSTRAINT ck_idc_refund_impact_tenant_legacy_v1 CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_impact_version_legacy_v1 CHECK ("service_version_snapshot" > 0),
    CONSTRAINT ck_idc_refund_impact_termination_legacy_v1 CHECK (
        NOT "requires_service_termination"
        OR ("causes_downtime" AND "causes_data_deletion")
    ),
    CONSTRAINT ck_idc_refund_impact_digest_legacy_v1 CHECK ("impact_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT uk_idc_refund_impact_no_legacy_v1 UNIQUE ("impact_no"),
    CONSTRAINT uk_idc_refund_impact_service_legacy_v1 UNIQUE (
        "refund_no", "refund_item_no", "service_no"
    ),
    CONSTRAINT fk_idc_refund_impact_item_legacy_v1 FOREIGN KEY ("refund_item_no")
        REFERENCES plugin_idc_core_refund_request_item ("refund_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_impact_service_legacy_v1 FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_refund_impact_request_legacy_v1
    ON plugin_idc_core_refund_service_impact (
        "tenant_id", "refund_no", "refund_item_no", "service_no", "id"
    );

-- Purpose: Deterministic refund split against original payment allocations.
-- 用途：保存退款来源分配相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_source_allocation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "refund_source_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "refund_item_no" VARCHAR(40) NOT NULL,
    "payment_allocation_no" VARCHAR(40) NOT NULL,
    "payment_source_type_code" VARCHAR(24) NOT NULL,
    "payment_source_reference" VARCHAR(128) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "execution_owner_code" VARCHAR(24) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_refund_source_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_source_type CHECK (
        "payment_source_type_code" IN (
            'channel_payment', 'balance', 'credit', 'manual_receipt', 'migration'
        )
    ),
    CONSTRAINT ck_idc_refund_source_owner CHECK (
        ("payment_source_type_code" = 'channel_payment' AND "execution_owner_code" = 'payment')
        OR ("payment_source_type_code" IN ('balance', 'credit') AND "execution_owner_code" = 'core')
        OR ("payment_source_type_code" IN ('manual_receipt', 'migration') AND "execution_owner_code" = 'manual')
    ),
    CONSTRAINT ck_idc_refund_source_amount CHECK ("amount_minor" > 0),
    CONSTRAINT ck_idc_refund_source_digest CHECK ("content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT uk_idc_refund_source_no UNIQUE ("refund_source_no"),
    CONSTRAINT uk_idc_refund_source_payment UNIQUE (
        "refund_no", "refund_item_no", "payment_allocation_no"
    ),
    CONSTRAINT fk_idc_refund_source_item FOREIGN KEY ("refund_item_no")
        REFERENCES plugin_idc_core_refund_request_item ("refund_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_source_payment FOREIGN KEY ("payment_allocation_no")
        REFERENCES plugin_idc_core_billing_payment_allocation ("allocation_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_refund_source_request
    ON plugin_idc_core_refund_source_allocation (
        "tenant_id", "refund_no", "refund_item_no", "id"
    );

-- Purpose: Active refund fence that blocks conflicting service operations.
-- 用途：保存退款服务锁定相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_service_lock (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "lock_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "refund_item_no" VARCHAR(40) NOT NULL,
    "impact_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "expected_service_version" BIGINT NOT NULL,
    "released_at" TIMESTAMPTZ,
    "release_reason_code" VARCHAR(32),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT pk_idc_refund_service_lock_legacy_v1 PRIMARY KEY ("id"),
    CONSTRAINT ck_idc_refund_lock_tenant_legacy_v1 CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_lock_version_legacy_v1 CHECK ("expected_service_version" > 0),
    CONSTRAINT ck_idc_refund_lock_release_legacy_v1 CHECK (
        ("released_at" IS NULL AND "release_reason_code" IS NULL)
        OR ("released_at" IS NOT NULL AND "release_reason_code" IN (
            'rejected', 'failed', 'succeeded', 'cancelled', 'superseded'
        ))
    ),
    CONSTRAINT uk_idc_refund_lock_no_legacy_v1 UNIQUE ("lock_no"),
    CONSTRAINT uk_idc_refund_lock_impact_legacy_v1 UNIQUE ("refund_no", "impact_no"),
    CONSTRAINT fk_idc_refund_lock_impact_legacy_v1 FOREIGN KEY ("impact_no")
        REFERENCES plugin_idc_core_refund_service_impact ("impact_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_lock_service_legacy_v1 FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_refund_lock_active_service_legacy_v1
    ON plugin_idc_core_refund_service_lock ("tenant_id", "service_no")
    WHERE "released_at" IS NULL;

-- Purpose: Append-only automatic, staff, customer-consent, and execution decisions.
-- 用途：保存退款决定相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_decision (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "decision_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "sequence_no" INT NOT NULL,
    "decision_type_code" VARCHAR(32) NOT NULL,
    "from_status_code" VARCHAR(32),
    "to_status_code" VARCHAR(32) NOT NULL,
    "actor_type_code" VARCHAR(16) NOT NULL,
    "actor_id" INT NOT NULL DEFAULT 0,
    "reason_code" VARCHAR(64) NOT NULL,
    "comment_digest" CHAR(64),
    "evidence_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_refund_decision_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_decision_sequence CHECK ("sequence_no" > 0),
    CONSTRAINT ck_idc_refund_decision_type CHECK (
        "decision_type_code" IN (
            'automatic_approve', 'manual_approve', 'manual_reject',
            'request_balance_consent', 'balance_consent', 'balance_refusal',
            'execution_started', 'execution_succeeded', 'execution_failed',
            'execution_unknown', 'manual_confirmation'
        )
    ),
    CONSTRAINT ck_idc_refund_decision_actor CHECK (
        ("actor_type_code" = 'system' AND "actor_id" = 0)
        OR ("actor_type_code" IN ('customer', 'staff') AND "actor_id" > 0)
    ),
    CONSTRAINT ck_idc_refund_decision_status CHECK (
        "to_status_code" IN (
            'pending_review', 'approved', 'awaiting_balance_consent',
            'executing', 'succeeded', 'failed', 'result_unknown',
            'manual_confirmation', 'rejected'
        )
        AND ("from_status_code" IS NULL OR "from_status_code" IN (
            'pending_review', 'approved', 'awaiting_balance_consent',
            'executing', 'succeeded', 'failed', 'result_unknown',
            'manual_confirmation', 'rejected'
        ))
    ),
    CONSTRAINT ck_idc_refund_decision_digest CHECK (
        "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND ("comment_digest" IS NULL OR "comment_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT uk_idc_refund_decision_no UNIQUE ("decision_no"),
    CONSTRAINT uk_idc_refund_decision_sequence UNIQUE ("refund_no", "sequence_no"),
    CONSTRAINT fk_idc_refund_decision_request FOREIGN KEY ("refund_no")
        REFERENCES plugin_idc_core_refund_request ("refund_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_refund_decision_history
    ON plugin_idc_core_refund_decision (
        "tenant_id", "refund_no", "sequence_no", "id"
    );

-- Purpose: Core-owned durable command claimed by the matching refund execution owner.
-- 用途：保存退款执行指令相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_execution_command (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "command_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "refund_source_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "phase_code" VARCHAR(16) NOT NULL DEFAULT 'execution',
    "destination_code" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "lease_owner" VARCHAR(128),
    "lease_expires_at" TIMESTAMPTZ,
    "attempt_count" INT NOT NULL DEFAULT 0,
    "preflight_result_code" VARCHAR(48),
    "preflight_evidence_digest" CHAR(64),
    "preflight_at" TIMESTAMPTZ,
    "result_code" VARCHAR(48),
    "result_reference" VARCHAR(128),
    "result_evidence_digest" CHAR(64),
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_refund_command_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_refund_command_status CHECK (
        "status_code" IN (
            'pending', 'claimed', 'succeeded', 'failed',
            'result_unknown', 'manual_confirmation'
        )
    ),
    CONSTRAINT ck_idc_refund_command_destination CHECK (
        "destination_code" IN ('original_channel', 'same_currency_balance')
    ),
    CONSTRAINT ck_idc_refund_command_phase CHECK (
        "phase_code" IN ('preflight', 'execution')
        AND ("phase_code" <> 'preflight' OR "destination_code" = 'original_channel')
    ),
    CONSTRAINT ck_idc_refund_command_amount CHECK ("amount_minor" > 0),
    CONSTRAINT ck_idc_refund_command_attempt CHECK ("attempt_count" BETWEEN 0 AND 100),
    CONSTRAINT ck_idc_refund_command_lease CHECK (
        ("status_code" = 'claimed' AND "lease_owner" IS NOT NULL AND "lease_expires_at" IS NOT NULL)
        OR ("status_code" <> 'claimed')
    ),
    CONSTRAINT ck_idc_refund_command_result CHECK (
        ("status_code" IN ('pending', 'claimed') AND "completed_at" IS NULL
            AND "result_code" IS NULL)
        OR ("status_code" IN ('succeeded', 'failed', 'result_unknown', 'manual_confirmation')
            AND "completed_at" IS NOT NULL AND "result_code" IS NOT NULL
            AND "result_code" ~ '^[a-z][a-z0-9._-]{0,47}$')
    ),
    CONSTRAINT ck_idc_refund_command_preflight CHECK (
        ("preflight_result_code" IS NULL AND "preflight_evidence_digest" IS NULL
            AND "preflight_at" IS NULL)
        OR ("preflight_result_code" IN ('preflight_ready', 'original_channel_unsupported')
            AND "preflight_evidence_digest" ~ '^[0-9a-f]{64}$'
            AND "preflight_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_refund_command_digest CHECK (
        "request_digest" ~ '^[0-9a-f]{64}$'
        AND ("result_evidence_digest" IS NULL OR "result_evidence_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT uk_idc_refund_command_no UNIQUE ("command_no"),
    CONSTRAINT fk_idc_refund_command_request FOREIGN KEY ("refund_no")
        REFERENCES plugin_idc_core_refund_request ("refund_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_refund_command_source FOREIGN KEY ("refund_source_no")
        REFERENCES plugin_idc_core_refund_source_allocation ("refund_source_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_refund_execution_command
    ADD COLUMN IF NOT EXISTS "result_code" VARCHAR(48);

ALTER TABLE plugin_idc_core_refund_execution_command
    ADD COLUMN IF NOT EXISTS "phase_code" VARCHAR(16) NOT NULL DEFAULT 'execution',
    ADD COLUMN IF NOT EXISTS "preflight_result_code" VARCHAR(48),
    ADD COLUMN IF NOT EXISTS "preflight_evidence_digest" CHAR(64),
    ADD COLUMN IF NOT EXISTS "preflight_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_core_refund_execution_command
    DROP CONSTRAINT IF EXISTS uk_idc_refund_command_source;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_refund_command_source_destination
    ON plugin_idc_core_refund_execution_command (
        "refund_source_no", "destination_code"
    );

CREATE INDEX IF NOT EXISTS idx_idc_refund_command_claim
    ON plugin_idc_core_refund_execution_command (
        "tenant_id", "status_code", "created_at", "id"
    ) WHERE "status_code" IN ('pending', 'claimed');

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_refund_request_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'refund requests cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."refund_no", OLD."customer_no", OLD."order_no",
        OLD."invoice_no", OLD."currency_code", OLD."reason_code", OLD."reason_detail",
        OLD."requested_amount_minor", OLD."approval_mode_code",
        OLD."requires_service_termination", OLD."has_resource_impact",
        OLD."has_dangerous_operation", OLD."has_fund_anomaly", OLD."high_amount",
        OLD."policy_no", OLD."policy_version", OLD."impact_digest",
        OLD."request_digest", OLD."idempotency_digest", OLD."submitted_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."refund_no", NEW."customer_no", NEW."order_no",
        NEW."invoice_no", NEW."currency_code", NEW."reason_code", NEW."reason_detail",
        NEW."requested_amount_minor", NEW."approval_mode_code",
        NEW."requires_service_termination", NEW."has_resource_impact",
        NEW."has_dangerous_operation", NEW."has_fund_anomaly", NEW."high_amount",
        NEW."policy_no", NEW."policy_version", NEW."impact_digest",
        NEW."request_digest", NEW."idempotency_digest", NEW."submitted_at"
    ) THEN
        RAISE EXCEPTION 'refund request intent is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_refund_history_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'refund history is append-only';
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_refund_conflict_operation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW."operation_type_code" <> 'refund' AND EXISTS (
        SELECT 1
        FROM plugin_idc_core_refund_service_lock service_lock
        WHERE service_lock."tenant_id" = NEW."tenant_id"
          AND service_lock."service_no" = NEW."service_no"
          AND service_lock."released_at" IS NULL
    ) THEN
        RAISE EXCEPTION 'service has an active refund operation lock';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_refund_conflict_lifecycle_task()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_refund_service_lock service_lock
        WHERE service_lock."tenant_id" = NEW."tenant_id"
          AND service_lock."service_no" = NEW."service_no"
          AND service_lock."released_at" IS NULL
    ) THEN
        RAISE EXCEPTION 'service has an active refund operation lock';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_refund_composition()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_refund_no VARCHAR(40);
    expected_amount BIGINT;
    item_amount BIGINT;
    invalid_items BIGINT;
    invalid_impacts BIGINT;
    invalid_sources BIGINT;
    missing_locks BIGINT;
    expected_resource_impact BOOLEAN;
    expected_termination BOOLEAN;
BEGIN
    target_refund_no := COALESCE(NEW."refund_no", OLD."refund_no");

    SELECT "requested_amount_minor" INTO expected_amount
    FROM plugin_idc_core_refund_request
    WHERE "refund_no" = target_refund_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT COALESCE(SUM("requested_amount_minor"), 0)
    INTO item_amount
    FROM plugin_idc_core_refund_request_item
    WHERE "refund_no" = target_refund_no;

    SELECT COUNT(*) INTO invalid_items
    FROM plugin_idc_core_refund_request_item item
    LEFT JOIN plugin_idc_core_billing_invoice_item invoice_item
      ON invoice_item."invoice_no" = item."invoice_no"
     AND invoice_item."invoice_item_no" = item."invoice_item_no"
     AND invoice_item."order_item_no" = item."order_item_no"
     AND invoice_item."customer_no" = item."customer_no"
     AND invoice_item."currency_code" = item."currency_code"
    WHERE item."refund_no" = target_refund_no
      AND (
          invoice_item."id" IS NULL
          OR (item."target_type_code" = 'service' AND NOT EXISTS (
              SELECT 1
              FROM plugin_idc_core_service_instance service
              WHERE service."service_no" = item."service_no"
                AND service."customer_no" = item."customer_no"
                AND service."currency_code" = item."currency_code"
                AND service."order_item_no" = item."order_item_no"
          ))
      );

    SELECT COUNT(*) INTO invalid_impacts
    FROM plugin_idc_core_refund_service_impact "impact"
    JOIN plugin_idc_core_refund_request_item item
      ON item."refund_no" = "impact"."refund_no"
     AND item."refund_item_no" = "impact"."refund_item_no"
    LEFT JOIN plugin_idc_core_service_instance service
      ON service."service_no" = "impact"."service_no"
     AND service."customer_no" = "impact"."customer_no"
     AND service."currency_code" = "impact"."currency_code"
     AND service."order_item_no" = "impact"."order_item_no"
    WHERE "impact"."refund_no" = target_refund_no
      AND (
          service."id" IS NULL
          OR "impact"."order_item_no" <> item."order_item_no"
          OR "impact"."customer_no" <> item."customer_no"
          OR "impact"."currency_code" <> item."currency_code"
          OR (item."target_type_code" = 'service' AND "impact"."service_no" <> item."service_no")
          OR "impact"."service_version_snapshot" <> service."version"
          OR "impact"."entitlement_status_snapshot" <> service."entitlement_status_code"
          OR "impact"."runtime_status_snapshot" <> service."runtime_status_code"
          OR "impact"."provider_task_status_snapshot" <> service."provider_task_status_code"
      );

    SELECT COALESCE(BOOL_OR(
        "requires_service_termination" OR "causes_downtime" OR "causes_data_deletion"
    ), FALSE), COALESCE(BOOL_OR("requires_service_termination"), FALSE)
    INTO expected_resource_impact, expected_termination
    FROM plugin_idc_core_refund_service_impact
    WHERE "refund_no" = target_refund_no;

    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_refund_request_item item
        WHERE item."refund_no" = target_refund_no
          AND (
              (item."target_type_code" = 'service' AND (
                  SELECT COUNT(*)
                  FROM plugin_idc_core_refund_service_impact "impact"
                  WHERE "impact"."refund_item_no" = item."refund_item_no"
                    AND "impact"."service_no" = item."service_no"
              ) <> 1)
              OR (item."target_type_code" = 'order_item' AND (
                  SELECT COUNT(*)
                  FROM plugin_idc_core_refund_service_impact "impact"
                  WHERE "impact"."refund_item_no" = item."refund_item_no"
              ) <> (
                  SELECT COUNT(*)
                  FROM plugin_idc_core_service_instance service
                  WHERE service."customer_no" = item."customer_no"
                    AND service."currency_code" = item."currency_code"
                    AND service."order_item_no" = item."order_item_no"
              ))
          )
    ) THEN
        invalid_impacts := invalid_impacts + 1;
    END IF;

    SELECT COUNT(*) INTO invalid_sources
    FROM plugin_idc_core_refund_request_item item
    LEFT JOIN LATERAL (
        SELECT COALESCE(SUM(source."amount_minor"), 0) AS "source_amount"
        FROM plugin_idc_core_refund_source_allocation source
        JOIN plugin_idc_core_billing_payment_allocation payment
          ON payment."allocation_no" = source."payment_allocation_no"
         AND payment."invoice_no" = item."invoice_no"
         AND payment."customer_no" = item."customer_no"
         AND payment."currency_code" = item."currency_code"
         AND payment."payment_source_type_code" = source."payment_source_type_code"
         AND payment."payment_source_reference" = source."payment_source_reference"
         AND (payment."invoice_item_no" IS NULL
              OR payment."invoice_item_no" = item."invoice_item_no")
        WHERE source."refund_item_no" = item."refund_item_no"
    ) source_total ON TRUE
    WHERE item."refund_no" = target_refund_no
      AND source_total."source_amount" <> item."requested_amount_minor";

    SELECT COUNT(*) INTO missing_locks
    FROM plugin_idc_core_refund_service_impact "impact"
    JOIN plugin_idc_core_refund_request_item item
      ON item."refund_item_no" = "impact"."refund_item_no"
     AND item."refund_no" = "impact"."refund_no"
    JOIN plugin_idc_core_refund_request request
      ON request."refund_no" = "impact"."refund_no"
    LEFT JOIN plugin_idc_core_refund_service_lock service_lock
      ON service_lock."refund_no" = "impact"."refund_no"
     AND service_lock."refund_item_no" = "impact"."refund_item_no"
     AND service_lock."impact_no" = "impact"."impact_no"
     AND service_lock."service_no" = "impact"."service_no"
     AND service_lock."expected_service_version" = "impact"."service_version_snapshot"
     AND service_lock."released_at" IS NULL
    WHERE "impact"."refund_no" = target_refund_no
      AND request."status_code" IN (
          'pending_review', 'approved', 'awaiting_balance_consent',
          'executing', 'result_unknown', 'manual_confirmation'
      )
      AND service_lock."id" IS NULL;

    IF item_amount <> expected_amount OR invalid_items <> 0 OR invalid_impacts <> 0
        OR invalid_sources <> 0 OR missing_locks <> 0 THEN
        RAISE EXCEPTION 'refund composition does not match request %', target_refund_no;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_refund_request request
        WHERE request."refund_no" = target_refund_no
          AND (
              request."has_resource_impact" <> expected_resource_impact
              OR request."requires_service_termination" <> expected_termination
          )
    ) THEN
        RAISE EXCEPTION 'refund impact flags do not match request %', target_refund_no;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_refund_request_intent_guard
    ON plugin_idc_core_refund_request;
CREATE TRIGGER trg_idc_refund_request_intent_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_refund_request
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_refund_request_intent();

DROP TRIGGER IF EXISTS trg_idc_refund_conflict_operation
    ON plugin_idc_core_business_operation;
CREATE TRIGGER trg_idc_refund_conflict_operation
BEFORE INSERT ON plugin_idc_core_business_operation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_refund_conflict_operation();

DROP TRIGGER IF EXISTS trg_idc_refund_conflict_lifecycle_task
    ON plugin_idc_core_service_lifecycle_task;
CREATE TRIGGER trg_idc_refund_conflict_lifecycle_task
BEFORE INSERT ON plugin_idc_core_service_lifecycle_task
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_refund_conflict_lifecycle_task();

DROP TRIGGER IF EXISTS trg_idc_refund_policy_immutable
    ON plugin_idc_core_refund_policy_version;
CREATE TRIGGER trg_idc_refund_policy_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_refund_policy_version
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_refund_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_refund_item_immutable
    ON plugin_idc_core_refund_request_item;
CREATE TRIGGER trg_idc_refund_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_refund_request_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_refund_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_refund_source_immutable
    ON plugin_idc_core_refund_source_allocation;
CREATE TRIGGER trg_idc_refund_source_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_refund_source_allocation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_refund_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_refund_impact_immutable
    ON plugin_idc_core_refund_service_impact;
CREATE TRIGGER trg_idc_refund_impact_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_refund_service_impact
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_refund_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_refund_decision_immutable
    ON plugin_idc_core_refund_decision;
CREATE TRIGGER trg_idc_refund_decision_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_refund_decision
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_refund_history_mutation();

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_refund_request'::regclass
           AND conname = 'trg_idc_refund_request_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_refund_request'::regclass
               AND constraint_row.conname = 'trg_idc_refund_request_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_refund_request_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_refund_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_refund_request_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSIF EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_refund_request'::regclass
           AND tgname = 'trg_idc_refund_request_composition'
           AND NOT tgisinternal
    ) THEN
        -- A later migration replaces this old constraint trigger with the
        -- queue trigger below. During full-manifest replay, accept only that
        -- exact later shape instead of dropping and recreating a constraint
        -- trigger inside the same transaction.
        IF NOT EXISTS (
            SELECT 1
              FROM pg_trigger
             WHERE tgrelid = 'plugin_idc_core_refund_request'::regclass
               AND tgname = 'trg_idc_refund_request_composition'
               AND NOT tgisinternal
               AND tgfoid = to_regprocedure('plugin_idc_core_enqueue_refund_composition_validation()')
               AND tgenabled = 'O'
        ) THEN
            RAISE EXCEPTION 'trigger trg_idc_refund_request_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_refund_request_composition
        AFTER INSERT OR UPDATE ON plugin_idc_core_refund_request
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_refund_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_refund_request_item'::regclass
           AND conname = 'trg_idc_refund_item_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_refund_request_item'::regclass
               AND constraint_row.conname = 'trg_idc_refund_item_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_refund_item_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_refund_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_refund_item_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSIF EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_refund_request_item'::regclass
           AND tgname = 'trg_idc_refund_item_composition'
           AND NOT tgisinternal
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_trigger
             WHERE tgrelid = 'plugin_idc_core_refund_request_item'::regclass
               AND tgname = 'trg_idc_refund_item_composition'
               AND NOT tgisinternal
               AND tgfoid = to_regprocedure('plugin_idc_core_enqueue_refund_composition_validation()')
               AND tgenabled = 'O'
        ) THEN
            RAISE EXCEPTION 'trigger trg_idc_refund_item_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_refund_item_composition
        AFTER INSERT ON plugin_idc_core_refund_request_item
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_refund_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_refund_source_allocation'::regclass
           AND conname = 'trg_idc_refund_source_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_refund_source_allocation'::regclass
               AND constraint_row.conname = 'trg_idc_refund_source_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_refund_source_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_refund_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_refund_source_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSIF EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_refund_source_allocation'::regclass
           AND tgname = 'trg_idc_refund_source_composition'
           AND NOT tgisinternal
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_trigger
             WHERE tgrelid = 'plugin_idc_core_refund_source_allocation'::regclass
               AND tgname = 'trg_idc_refund_source_composition'
               AND NOT tgisinternal
               AND tgfoid = to_regprocedure('plugin_idc_core_enqueue_refund_composition_validation()')
               AND tgenabled = 'O'
        ) THEN
            RAISE EXCEPTION 'trigger trg_idc_refund_source_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_refund_source_composition
        AFTER INSERT ON plugin_idc_core_refund_source_allocation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_refund_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_refund_service_impact'::regclass
           AND conname = 'trg_idc_refund_impact_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_refund_service_impact'::regclass
               AND constraint_row.conname = 'trg_idc_refund_impact_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_refund_impact_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_refund_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_refund_impact_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSIF EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_refund_service_impact'::regclass
           AND tgname = 'trg_idc_refund_impact_composition'
           AND NOT tgisinternal
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_trigger
             WHERE tgrelid = 'plugin_idc_core_refund_service_impact'::regclass
               AND tgname = 'trg_idc_refund_impact_composition'
               AND NOT tgisinternal
               AND tgfoid = to_regprocedure('plugin_idc_core_enqueue_refund_composition_validation()')
               AND tgenabled = 'O'
        ) THEN
            RAISE EXCEPTION 'trigger trg_idc_refund_impact_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_refund_impact_composition
        AFTER INSERT ON plugin_idc_core_refund_service_impact
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_refund_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_refund_service_lock'::regclass
           AND conname = 'trg_idc_refund_lock_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_refund_service_lock'::regclass
               AND constraint_row.conname = 'trg_idc_refund_lock_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_refund_lock_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_refund_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_refund_lock_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSIF EXISTS (
        SELECT 1
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_refund_service_lock'::regclass
           AND tgname = 'trg_idc_refund_lock_composition'
           AND NOT tgisinternal
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_trigger
             WHERE tgrelid = 'plugin_idc_core_refund_service_lock'::regclass
               AND tgname = 'trg_idc_refund_lock_composition'
               AND NOT tgisinternal
               AND tgfoid = to_regprocedure('plugin_idc_core_enqueue_refund_composition_validation()')
               AND tgenabled = 'O'
        ) THEN
            RAISE EXCEPTION 'trigger trg_idc_refund_lock_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_refund_lock_composition
        AFTER INSERT OR UPDATE ON plugin_idc_core_refund_service_lock
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_refund_composition();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_refund_policy_version
    IS 'Immutable per-currency refund approval and recent-login threshold versions';
COMMENT ON TABLE plugin_idc_core_refund_request
    IS 'Core-owned online refund request and current workflow projection';
COMMENT ON TABLE plugin_idc_core_refund_request_item
    IS 'Immutable item or server refund amount and customer-confirmed impact snapshot';
COMMENT ON TABLE plugin_idc_core_refund_service_impact
    IS 'Per-server immutable downtime, data deletion, termination, and version snapshot';
COMMENT ON TABLE plugin_idc_core_refund_source_allocation
    IS 'Deterministic refund split against original payment allocations';
COMMENT ON TABLE plugin_idc_core_refund_service_lock
    IS 'Active refund fence that blocks conflicting service operations';
COMMENT ON TABLE plugin_idc_core_refund_decision
    IS 'Append-only automatic, staff, customer-consent, and execution decisions';
COMMENT ON TABLE plugin_idc_core_refund_execution_command
    IS 'Core-owned durable command claimed by the matching refund execution owner';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 退款状态', 'idc_refund_status', 1, 1, FALSE,
     '在线退款申请和执行状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 退款原因', 'idc_refund_reason', 1, 1, FALSE,
     '客户可选择的固定退款原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 退款审核方式', 'idc_refund_approval_mode', 1, 1, FALSE,
     '自动审核或人工审核', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 退款执行状态', 'idc_refund_execution_status', 1, 1, FALSE,
     '退款资金执行命令状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_refund_status', '等待审核', 'pending_review', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '已批准', 'approved', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '等待余额确认', 'awaiting_balance_consent', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '退款中', 'executing', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '已退款', 'succeeded', 5, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '退款失败', 'failed', 6, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '结果未知', 'result_unknown', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '人工确认', 'manual_confirmation', 8, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_status', '已拒绝', 'rejected', 9, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_reason', '重复购买', 'duplicate_purchase', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_reason', '配置选错', 'wrong_configuration', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_reason', '服务问题', 'service_issue', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_reason', '不再需要', 'no_longer_needed', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_reason', '其他', 'other', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_approval_mode', '自动审核', 'automatic', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_approval_mode', '人工审核', 'manual', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_execution_status', '等待执行', 'pending', 1, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_execution_status', '已领取', 'claimed', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_execution_status', '已成功', 'succeeded', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_execution_status', '已失败', 'failed', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_execution_status', '结果未知', 'result_unknown', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_refund_execution_status', '人工确认', 'manual_confirmation', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
