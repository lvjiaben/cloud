-- 001: IDC resource channel, routing, task, and observation foundation
--
-- Configured channels and pools use recoverable soft deletion. Secret
-- bindings, resource mappings, tasks, and upstream observations retain
-- explicit lifecycle history and are never soft-deleted.

-- Purpose: Persists business records for plugin_idc_resource_channel.
-- 用途：持久化 plugin_idc_resource_channel 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_channel (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "channel_no" VARCHAR(64) NOT NULL,
    "provider_id" VARCHAR(128) NOT NULL,
    "provider_plugin_id" VARCHAR(128) NOT NULL,
    "display_name" VARCHAR(128) NOT NULL,
    "endpoint" VARCHAR(2048) NOT NULL,
    "non_secret_settings" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'disabled',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT ck_plugin_idc_resource_channel_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_channel_provider CHECK (
        length("provider_id") > 0 AND length("provider_plugin_id") > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_channel_endpoint CHECK (
        "endpoint" = btrim("endpoint") AND left(lower("endpoint"), 8) = 'https://'
    ),
    CONSTRAINT ck_plugin_idc_resource_channel_settings CHECK (
        jsonb_typeof("non_secret_settings") = 'object'
    ),
    CONSTRAINT ck_plugin_idc_resource_channel_status CHECK (
        "status_code" IN ('disabled', 'enabled', 'degraded', 'maintenance', 'deleted')
    ),
    CONSTRAINT ck_plugin_idc_resource_channel_delete CHECK (
        ("status_code" = 'deleted' AND "deleted_at" IS NOT NULL) OR
        ("status_code" <> 'deleted' AND "deleted_at" IS NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_channel_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_channel_no UNIQUE ("channel_no")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_channel_status
    ON plugin_idc_resource_channel (
        "tenant_id", "status_code", "provider_id", "updated_at" DESC, "id"
    ) WHERE "deleted_at" IS NULL;

COMMENT ON TABLE plugin_idc_resource_channel IS 'Configured upstream Provider channel with non-secret settings only';
COMMENT ON COLUMN plugin_idc_resource_channel."channel_no" IS 'Opaque resource-owner channel reference';
COMMENT ON COLUMN plugin_idc_resource_channel."provider_id" IS 'Provider descriptor identifier declared through resourcecap SPI';
COMMENT ON COLUMN plugin_idc_resource_channel."provider_plugin_id" IS 'Source plugin that owns the Provider factory';
COMMENT ON COLUMN plugin_idc_resource_channel."endpoint" IS 'Administrator-approved HTTPS API origin without credentials';
COMMENT ON COLUMN plugin_idc_resource_channel."non_secret_settings" IS 'Adapter settings validated to exclude credentials and secret references';
COMMENT ON COLUMN plugin_idc_resource_channel."deleted_at" IS 'Recoverable configuration deletion time; historical records remain referenced';

-- Purpose: Persists business records for plugin_idc_resource_channel_secret_binding.
-- 用途：持久化 plugin_idc_resource_channel_secret_binding 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_channel_secret_binding (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "binding_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "binding_name" VARCHAR(128) NOT NULL,
    "secret_ref" VARCHAR(256) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "bound_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "superseded_at" TIMESTAMPTZ,
    "revoked_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_binding_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_binding_name CHECK (
        "binding_name" = btrim("binding_name") AND length("binding_name") > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_binding_ref CHECK (
        "secret_ref" = btrim("secret_ref") AND length("secret_ref") > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_binding_status CHECK (
        "status_code" IN ('active', 'superseded', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_resource_binding_lifecycle CHECK (
        ("status_code" = 'active' AND "superseded_at" IS NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'superseded' AND "superseded_at" IS NOT NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_binding_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_binding_no UNIQUE ("binding_no"),
    CONSTRAINT fk_plugin_idc_resource_binding_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_binding_active
    ON plugin_idc_resource_channel_secret_binding (
        "tenant_id", "channel_no", "binding_name"
    ) WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_binding_channel
    ON plugin_idc_resource_channel_secret_binding (
        "tenant_id", "channel_no", "status_code", "binding_name", "id"
    );

COMMENT ON TABLE plugin_idc_resource_channel_secret_binding IS 'Owner-only channel binding to a core-encrypted opaque secret reference';
COMMENT ON COLUMN plugin_idc_resource_channel_secret_binding."secret_ref" IS 'Sensitive routing metadata from secretcap Handle.OpaqueRef; never returned, logged, or audited';
COMMENT ON COLUMN plugin_idc_resource_channel_secret_binding."binding_name" IS 'Adapter-defined credential slot such as username or password';

-- Purpose: Persists business records for plugin_idc_resource_pool.
-- 用途：持久化 plugin_idc_resource_pool 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_pool (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "pool_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "display_name" VARCHAR(128) NOT NULL,
    "resource_kind" VARCHAR(24) NOT NULL DEFAULT 'virtual_server',
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'disabled',
    "routing_priority" INT NOT NULL DEFAULT 100,
    "routing_weight" INT NOT NULL DEFAULT 100,
    "selector" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT ck_plugin_idc_resource_pool_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_pool_kind CHECK (
        "resource_kind" IN ('virtual_server', 'bare_metal')
    ),
    CONSTRAINT ck_plugin_idc_resource_pool_status CHECK (
        "status_code" IN ('disabled', 'enabled', 'draining', 'maintenance', 'deleted')
    ),
    CONSTRAINT ck_plugin_idc_resource_pool_routing CHECK (
        "routing_priority" BETWEEN 0 AND 100000 AND "routing_weight" BETWEEN 0 AND 10000
    ),
    CONSTRAINT ck_plugin_idc_resource_pool_selector CHECK (jsonb_typeof("selector") = 'object'),
    CONSTRAINT ck_plugin_idc_resource_pool_delete CHECK (
        ("status_code" = 'deleted' AND "deleted_at" IS NOT NULL) OR
        ("status_code" <> 'deleted' AND "deleted_at" IS NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_pool_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_pool_no UNIQUE ("pool_no"),
    CONSTRAINT uk_plugin_idc_resource_pool_channel_no UNIQUE ("channel_no", "pool_no"),
    CONSTRAINT fk_plugin_idc_resource_pool_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_pool_route
    ON plugin_idc_resource_pool (
        "tenant_id", "resource_kind", "status_code", "routing_priority", "routing_weight" DESC, "id"
    ) WHERE "deleted_at" IS NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_pool_channel
    ON plugin_idc_resource_pool (
        "tenant_id", "channel_no", "status_code", "routing_priority", "id"
    ) WHERE "deleted_at" IS NULL;

COMMENT ON TABLE plugin_idc_resource_pool IS 'Routing pool bound to one upstream channel and one normalized resource kind';
COMMENT ON COLUMN plugin_idc_resource_pool."selector" IS 'Non-secret normalized placement selector evaluated in set-based routing';
COMMENT ON COLUMN plugin_idc_resource_pool."routing_weight" IS 'Relative selection weight within the same routing priority';

-- Purpose: Persists business records for plugin_idc_resource_mapping.
-- 用途：持久化 plugin_idc_resource_mapping 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_mapping (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "mapping_no" VARCHAR(64) NOT NULL,
    "local_resource_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "pool_no" VARCHAR(64) NOT NULL,
    "external_resource_ref" VARCHAR(256) NOT NULL DEFAULT '',
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'provisioning',
    "bound_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "released_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_mapping_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_mapping_status CHECK (
        "status_code" IN ('provisioning', 'active', 'quarantined', 'releasing', 'released')
    ),
    CONSTRAINT ck_plugin_idc_resource_mapping_external CHECK (
        ("status_code" = 'provisioning') OR length("external_resource_ref") > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_mapping_release CHECK (
        ("status_code" = 'released' AND "released_at" IS NOT NULL) OR
        ("status_code" <> 'released' AND "released_at" IS NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_mapping_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_mapping_no UNIQUE ("mapping_no"),
    CONSTRAINT fk_plugin_idc_resource_mapping_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_resource_mapping_pool FOREIGN KEY ("channel_no", "pool_no")
        REFERENCES plugin_idc_resource_pool ("channel_no", "pool_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_mapping_local
    ON plugin_idc_resource_mapping ("tenant_id", "local_resource_no")
    WHERE "status_code" <> 'released';
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_mapping_external
    ON plugin_idc_resource_mapping ("tenant_id", "channel_no", "external_resource_ref")
    WHERE "status_code" <> 'released' AND "external_resource_ref" <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_mapping_pool
    ON plugin_idc_resource_mapping (
        "tenant_id", "pool_no", "status_code", "updated_at" DESC, "id"
    );

COMMENT ON TABLE plugin_idc_resource_mapping IS 'Resource-owner authoritative mapping from a local service resource to one upstream object';
COMMENT ON COLUMN plugin_idc_resource_mapping."local_resource_no" IS 'Opaque core-owned resource number; no cross-plugin database foreign key is used';
COMMENT ON COLUMN plugin_idc_resource_mapping."external_resource_ref" IS 'Opaque Provider resource reference, never interpreted as a local identity';

-- Purpose: Persists business records for plugin_idc_resource_capability_snapshot.
-- 用途：持久化 plugin_idc_resource_capability_snapshot 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_capability_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "adapter_version" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'collecting',
    "item_count" INT NOT NULL DEFAULT 0,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_cap_snapshot_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_cap_snapshot_status CHECK (
        "status_code" IN ('collecting', 'complete', 'failed', 'superseded')
    ),
    CONSTRAINT ck_plugin_idc_resource_cap_snapshot_count CHECK ("item_count" >= 0),
    CONSTRAINT ck_plugin_idc_resource_cap_snapshot_complete CHECK (
        ("status_code" = 'collecting' AND "completed_at" IS NULL) OR
        ("status_code" <> 'collecting' AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT uk_plugin_idc_resource_cap_snapshot_no UNIQUE ("snapshot_no"),
    CONSTRAINT fk_plugin_idc_resource_cap_snapshot_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_cap_snapshot_latest
    ON plugin_idc_resource_capability_snapshot (
        "tenant_id", "channel_no", "status_code", "observed_at" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_resource_capability_snapshot_item.
-- 用途：持久化 plugin_idc_resource_capability_snapshot_item 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_capability_snapshot_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(64) NOT NULL,
    "capability_code" VARCHAR(128) NOT NULL,
    "support_code" VARCHAR(16) NOT NULL,
    "constraints" JSONB NOT NULL DEFAULT '[]'::JSONB,
    "unsupported_reason" VARCHAR(512) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_cap_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_cap_item_support CHECK (
        "support_code" IN ('unsupported', 'conditional', 'supported')
    ),
    CONSTRAINT ck_plugin_idc_resource_cap_item_constraints CHECK (
        jsonb_typeof("constraints") = 'array'
    ),
    CONSTRAINT ck_plugin_idc_resource_cap_item_reason CHECK (
        ("support_code" = 'unsupported' AND length("unsupported_reason") > 0) OR
        ("support_code" <> 'unsupported' AND "unsupported_reason" = '')
    ),
    CONSTRAINT uk_plugin_idc_resource_cap_item UNIQUE ("snapshot_no", "capability_code"),
    CONSTRAINT fk_plugin_idc_resource_cap_item_snapshot FOREIGN KEY ("snapshot_no")
        REFERENCES plugin_idc_resource_capability_snapshot ("snapshot_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_cap_item_lookup
    ON plugin_idc_resource_capability_snapshot_item (
        "tenant_id", "snapshot_no", "support_code", "capability_code"
    );

COMMENT ON TABLE plugin_idc_resource_capability_snapshot IS 'Immutable header for one Provider capability discovery observation';
COMMENT ON TABLE plugin_idc_resource_capability_snapshot_item IS 'Field-bounded capability descriptors belonging to one immutable snapshot';

-- Purpose: Persists business records for plugin_idc_resource_task.
-- 用途：持久化 plugin_idc_resource_task 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_task (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(64) NOT NULL,
    "request_id_digest" CHAR(64) NOT NULL,
    "capability_code" VARCHAR(128) NOT NULL,
    "local_resource_no" VARCHAR(64) NOT NULL DEFAULT '',
    "channel_no" VARCHAR(64) NOT NULL,
    "pool_no" VARCHAR(64),
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "priority" SMALLINT NOT NULL DEFAULT 100,
    "sanitized_parameters" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "attempt_count" INT NOT NULL DEFAULT 0,
    "max_attempts" INT NOT NULL DEFAULT 8,
    "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "locked_by" VARCHAR(128) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "next_observe_at" TIMESTAMPTZ,
    "accepted_at" TIMESTAMPTZ,
    "finished_at" TIMESTAMPTZ,
    "failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "failure_summary" VARCHAR(1024) NOT NULL DEFAULT '',
    "result_unknown" BOOLEAN NOT NULL DEFAULT FALSE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_task_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_task_digest CHECK ("request_id_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_resource_task_status CHECK (
        "status_code" IN (
            'pending', 'submitting', 'accepted', 'running', 'retry_wait',
            'succeeded', 'failed', 'result_unknown', 'manual_takeover', 'canceled'
        )
    ),
    CONSTRAINT ck_plugin_idc_resource_task_parameters CHECK (
        jsonb_typeof("sanitized_parameters") = 'object'
    ),
    CONSTRAINT ck_plugin_idc_resource_task_attempt CHECK (
        "attempt_count" >= 0 AND "max_attempts" > 0 AND "attempt_count" <= "max_attempts"
    ),
    CONSTRAINT ck_plugin_idc_resource_task_unknown CHECK (
        ("status_code" = 'result_unknown' AND "result_unknown") OR
        ("status_code" <> 'result_unknown' AND NOT "result_unknown")
    ),
    CONSTRAINT ck_plugin_idc_resource_task_finish CHECK (
        ("status_code" IN ('succeeded', 'failed', 'manual_takeover', 'canceled') AND "finished_at" IS NOT NULL) OR
        ("status_code" NOT IN ('succeeded', 'failed', 'manual_takeover', 'canceled') AND "finished_at" IS NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_task_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_task_no UNIQUE ("task_no"),
    CONSTRAINT uk_plugin_idc_resource_task_request UNIQUE (
        "tenant_id", "channel_no", "request_id_digest"
    ),
    CONSTRAINT uk_plugin_idc_resource_task_channel_no UNIQUE ("task_no", "channel_no"),
    CONSTRAINT fk_plugin_idc_resource_task_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_resource_task_pool FOREIGN KEY ("channel_no", "pool_no")
        REFERENCES plugin_idc_resource_pool ("channel_no", "pool_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_claim
    ON plugin_idc_resource_task (
        "tenant_id", "status_code", "available_at", "priority", "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_lease
    ON plugin_idc_resource_task ("tenant_id", "status_code", "lease_expires_at", "id")
    WHERE "lease_expires_at" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_observe
    ON plugin_idc_resource_task (
        "tenant_id", "status_code", "next_observe_at", "priority", "id"
    )
    WHERE "next_observe_at" IS NOT NULL AND
          "status_code" IN ('accepted', 'running', 'result_unknown');
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_resource
    ON plugin_idc_resource_task (
        "tenant_id", "local_resource_no", "created_at" DESC, "id" DESC
    ) WHERE "local_resource_no" <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_channel
    ON plugin_idc_resource_task (
        "tenant_id", "channel_no", "status_code", "created_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_resource_task IS 'Owner-side asynchronous Provider operation with idempotency, lease, and unknown-result protection';
COMMENT ON COLUMN plugin_idc_resource_task."request_id_digest" IS 'SHA-256 digest of the caller idempotency key; raw keys are not persisted';
COMMENT ON COLUMN plugin_idc_resource_task."sanitized_parameters" IS 'Validated non-secret operation parameters; credential handles and plaintext are forbidden';
COMMENT ON COLUMN plugin_idc_resource_task."result_unknown" IS 'Hard replay fence requiring observation or manual reconciliation';

-- Purpose: Persists business records for plugin_idc_resource_task_secret_binding.
-- 用途：持久化 plugin_idc_resource_task_secret_binding 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_task_secret_binding (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "binding_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "binding_name" VARCHAR(128) NOT NULL,
    "binding_direction" VARCHAR(8) NOT NULL,
    "secret_ref" VARCHAR(256) NOT NULL,
    "output_policy" VARCHAR(32),
    "expires_at" TIMESTAMPTZ,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "revealed_at" TIMESTAMPTZ,
    "revoked_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_task_secret_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_task_secret_name CHECK (
        "binding_name" = btrim("binding_name") AND length("binding_name") > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_task_secret_ref CHECK (
        "secret_ref" = btrim("secret_ref") AND length("secret_ref") > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_task_secret_direction CHECK (
        "binding_direction" IN ('input', 'output')
    ),
    CONSTRAINT ck_plugin_idc_resource_task_secret_policy CHECK (
        (
            "binding_direction" = 'input' AND "output_policy" IS NULL AND "expires_at" IS NULL
        ) OR
        (
            "binding_direction" = 'output' AND "output_policy" = 'one_time_credential' AND
            "expires_at" IS NULL
        ) OR
        (
            "binding_direction" = 'output' AND "output_policy" = 'short_lived_ticket' AND
            "expires_at" IS NOT NULL
        )
    ),
    CONSTRAINT ck_plugin_idc_resource_task_secret_status CHECK (
        "status_code" IN ('active', 'revealed', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_resource_task_secret_lifecycle CHECK (
        ("status_code" = 'active' AND "revealed_at" IS NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'revealed' AND "binding_direction" = 'output' AND
            "revealed_at" IS NOT NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_task_secret_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_task_secret_binding_no UNIQUE ("binding_no"),
    CONSTRAINT fk_plugin_idc_resource_task_secret_task FOREIGN KEY ("task_no")
        REFERENCES plugin_idc_resource_task ("task_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_task_secret_active
    ON plugin_idc_resource_task_secret_binding (
        "tenant_id", "task_no", "binding_direction", "binding_name"
    ) WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_secret_task
    ON plugin_idc_resource_task_secret_binding (
        "tenant_id", "task_no", "binding_direction", "status_code", "binding_name", "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_secret_expiry
    ON plugin_idc_resource_task_secret_binding (
        "tenant_id", "status_code", "expires_at", "id"
    ) WHERE "binding_direction" = 'output' AND "expires_at" IS NOT NULL;

COMMENT ON TABLE plugin_idc_resource_task_secret_binding IS 'Owner-only opaque secret references required or produced by an asynchronous resource task';
COMMENT ON COLUMN plugin_idc_resource_task_secret_binding."secret_ref" IS 'Sensitive secretcap routing reference excluded from task JSON, logs, audit, and API responses';
COMMENT ON COLUMN plugin_idc_resource_task_secret_binding."output_policy" IS 'One-time credential or short-lived ticket delivery policy for output bindings';

-- Purpose: Persists business records for plugin_idc_resource_upstream_task_binding.
-- 用途：持久化 plugin_idc_resource_upstream_task_binding 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_upstream_task_binding (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "binding_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "step_no" INT NOT NULL DEFAULT 1,
    "channel_no" VARCHAR(64) NOT NULL,
    "external_task_ref" VARCHAR(256) NOT NULL,
    "external_resource_ref" VARCHAR(256) NOT NULL DEFAULT '',
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'accepted',
    "last_progress" SMALLINT,
    "last_observed_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_upstream_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_upstream_step CHECK ("step_no" > 0),
    CONSTRAINT ck_plugin_idc_resource_upstream_ref CHECK (length("external_task_ref") > 0),
    CONSTRAINT ck_plugin_idc_resource_upstream_status CHECK (
        "status_code" IN ('accepted', 'running', 'succeeded', 'failed', 'terminated', 'canceled', 'unknown')
    ),
    CONSTRAINT ck_plugin_idc_resource_upstream_progress CHECK (
        "last_progress" IS NULL OR "last_progress" BETWEEN 0 AND 100
    ),
    CONSTRAINT uk_plugin_idc_resource_upstream_binding_no UNIQUE ("binding_no"),
    CONSTRAINT uk_plugin_idc_resource_upstream_step UNIQUE ("task_no", "step_no"),
    CONSTRAINT uk_plugin_idc_resource_upstream_ref UNIQUE (
        "tenant_id", "channel_no", "external_task_ref"
    ),
    CONSTRAINT fk_plugin_idc_resource_upstream_task FOREIGN KEY ("task_no", "channel_no")
        REFERENCES plugin_idc_resource_task ("task_no", "channel_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_resource_upstream_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_upstream_observe
    ON plugin_idc_resource_upstream_task_binding (
        "tenant_id", "status_code", "last_observed_at", "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_upstream_resource
    ON plugin_idc_resource_upstream_task_binding (
        "tenant_id", "channel_no", "external_resource_ref", "last_observed_at" DESC
    ) WHERE "external_resource_ref" <> '';

COMMENT ON TABLE plugin_idc_resource_upstream_task_binding IS 'Exact ownership mapping between a local task step and one upstream task reference';

-- Purpose: Persists business records for plugin_idc_resource_provider_capacity_observation.
-- 用途：持久化 plugin_idc_resource_provider_capacity_observation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provider_capacity_observation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "observation_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "capacity_kind" VARCHAR(24) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'collecting',
    "observation_fingerprint" CHAR(64) NOT NULL DEFAULT '',
    "item_count" INT NOT NULL DEFAULT 0,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_capacity_kind CHECK (
        "capacity_kind" IN ('region', 'line', 'plan', 'image', 'stock')
    ),
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_status CHECK (
        "status_code" IN ('collecting', 'complete', 'failed', 'superseded')
    ),
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_fingerprint CHECK (
        "observation_fingerprint" = '' OR "observation_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_count CHECK ("item_count" >= 0),
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_complete CHECK (
        ("status_code" = 'collecting' AND "completed_at" IS NULL) OR
        ("status_code" <> 'collecting' AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT uk_plugin_idc_resource_provider_capacity_observation_no UNIQUE ("observation_no"),
    CONSTRAINT uk_plugin_idc_resource_provider_capacity_observation_kind UNIQUE (
        "observation_no", "capacity_kind"
    ),
    CONSTRAINT fk_plugin_idc_resource_provider_capacity_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provider_capacity_latest
    ON plugin_idc_resource_provider_capacity_observation (
        "tenant_id", "channel_no", "capacity_kind", "status_code", "observed_at" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_resource_provider_capacity_observation_item.
-- 用途：持久化 plugin_idc_resource_provider_capacity_observation_item 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provider_capacity_observation_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "observation_no" VARCHAR(64) NOT NULL,
    "capacity_kind" VARCHAR(24) NOT NULL,
    "external_ref" VARCHAR(512) NOT NULL,
    "projection" JSONB NOT NULL,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_item_kind CHECK (
        "capacity_kind" IN ('region', 'line', 'plan', 'image', 'stock')
    ),
    CONSTRAINT ck_plugin_idc_resource_provider_capacity_item_projection CHECK (
        jsonb_typeof("projection") = 'object'
    ),
    CONSTRAINT uk_plugin_idc_resource_provider_capacity_item UNIQUE (
        "observation_no", "capacity_kind", "external_ref"
    ),
    CONSTRAINT fk_plugin_idc_resource_provider_capacity_item_observation FOREIGN KEY (
        "observation_no", "capacity_kind"
    ) REFERENCES plugin_idc_resource_provider_capacity_observation (
        "observation_no", "capacity_kind"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provider_capacity_item_lookup
    ON plugin_idc_resource_provider_capacity_observation_item (
        "tenant_id", "observation_no", "capacity_kind", "external_ref", "id"
    );

COMMENT ON TABLE plugin_idc_resource_provider_capacity_observation IS 'Bounded immutable Provider capacity observation header';
COMMENT ON TABLE plugin_idc_resource_provider_capacity_observation_item IS 'Field-whitelisted Provider capacity observation rows inserted in batches';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation_item."projection" IS 'Normalized non-secret projection; raw Provider responses are forbidden';

-- Purpose: Persists business records for plugin_idc_resource_financial_snapshot.
-- 用途：持久化 plugin_idc_resource_financial_snapshot 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_financial_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "snapshot_kind" VARCHAR(16) NOT NULL,
    "external_resource_ref" VARCHAR(256) NOT NULL DEFAULT '',
    "currency_code" CHAR(3),
    "amount" NUMERIC(24, 8),
    "expires_at" TIMESTAMPTZ,
    "projection" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_financial_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_financial_kind CHECK (
        "snapshot_kind" IN ('cost', 'balance', 'expiry')
    ),
    CONSTRAINT ck_plugin_idc_resource_financial_projection CHECK (
        jsonb_typeof("projection") = 'object'
    ),
    CONSTRAINT ck_plugin_idc_resource_financial_shape CHECK (
        (
            "snapshot_kind" IN ('cost', 'balance') AND "amount" IS NOT NULL AND "amount" >= 0 AND
            "currency_code" ~ '^[A-Z]{3}$' AND "expires_at" IS NULL
        ) OR
        (
            "snapshot_kind" = 'expiry' AND length("external_resource_ref") > 0 AND
            "amount" IS NULL AND "currency_code" IS NULL AND "expires_at" IS NOT NULL
        )
    ),
    CONSTRAINT uk_plugin_idc_resource_financial_snapshot_no UNIQUE ("snapshot_no"),
    CONSTRAINT fk_plugin_idc_resource_financial_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_financial_latest
    ON plugin_idc_resource_financial_snapshot (
        "tenant_id", "channel_no", "snapshot_kind", "observed_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_expiry_resource
    ON plugin_idc_resource_financial_snapshot (
        "tenant_id", "channel_no", "external_resource_ref", "expires_at", "id"
    ) WHERE "snapshot_kind" = 'expiry';

COMMENT ON TABLE plugin_idc_resource_financial_snapshot IS 'Immutable Provider-reported cost, balance, or upstream expiry observation';
COMMENT ON COLUMN plugin_idc_resource_financial_snapshot."amount" IS 'Non-negative Provider amount in the exact ISO currency; never converted implicitly';

-- Purpose: Persists business records for plugin_idc_resource_health_snapshot.
-- 用途：持久化 plugin_idc_resource_health_snapshot 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_health_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "health_code" VARCHAR(16) NOT NULL,
    "latency_ms" INT,
    "consecutive_failures" INT NOT NULL DEFAULT 0,
    "circuit_open" BOOLEAN NOT NULL DEFAULT FALSE,
    "capability_observed_at" TIMESTAMPTZ,
    "provider_capacity_observed_at" TIMESTAMPTZ,
    "summary_code" VARCHAR(128) NOT NULL DEFAULT '',
    "observed_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_health_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_health_code CHECK (
        "health_code" IN ('healthy', 'degraded', 'unavailable', 'disabled', 'unknown')
    ),
    CONSTRAINT ck_plugin_idc_resource_health_latency CHECK (
        "latency_ms" IS NULL OR "latency_ms" BETWEEN 0 AND 3600000
    ),
    CONSTRAINT ck_plugin_idc_resource_health_failures CHECK ("consecutive_failures" >= 0),
    CONSTRAINT uk_plugin_idc_resource_health_snapshot_no UNIQUE ("snapshot_no"),
    CONSTRAINT fk_plugin_idc_resource_health_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_health_latest
    ON plugin_idc_resource_health_snapshot (
        "tenant_id", "channel_no", "observed_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_health_status
    ON plugin_idc_resource_health_snapshot (
        "tenant_id", "health_code", "observed_at" DESC, "channel_no"
    );

COMMENT ON TABLE plugin_idc_resource_health_snapshot IS 'Immutable bounded channel health observation for routing and operator diagnostics';
COMMENT ON COLUMN plugin_idc_resource_health_snapshot."summary_code" IS 'Stable non-sensitive machine code; raw upstream errors are forbidden';
