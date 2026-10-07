-- 006: Stable audit reservation lookup and target timeline keyset index
--
-- Audit reservation digests are generated with an installation-stable,
-- independently purposed HMAC key. They therefore remain locatable after a
-- replay-evidence key is retired without persisting a plaintext idempotency key.

COMMENT ON COLUMN plugin_idc_core_business_number."reservation_scope_digest"
    IS 'Keyed digest of the normalized caller or command scope; audit reservations use the stable installation lookup key';
COMMENT ON COLUMN plugin_idc_core_business_number."idempotency_key_digest"
    IS 'Keyed digest of the reservation idempotency key; audit reservations use the stable installation lookup key';

-- Match the target-filter keyset order exactly. The final ID key prevents an
-- incremental sort and a same-timestamp scan before LIMIT can be satisfied.
DROP INDEX IF EXISTS idx_plugin_idc_core_audit_target;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_target
    ON plugin_idc_core_business_audit_event
    ("tenant_id", "target_type", "target_no", "occurred_at" DESC, "id" DESC);
