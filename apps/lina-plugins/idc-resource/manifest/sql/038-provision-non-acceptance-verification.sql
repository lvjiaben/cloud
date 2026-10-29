-- Immutable operator facts supplement, but never replace, unknown provider history.
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provision_non_acceptance_verification (
 tenant_id BIGINT NOT NULL DEFAULT 0 CHECK(tenant_id=0),
 task_no VARCHAR(64) PRIMARY KEY REFERENCES plugin_idc_resource_task(task_no),
 request_digest CHAR(64) NOT NULL,
 verification_digest CHAR(64) NOT NULL,
 actor_user_id BIGINT NOT NULL CHECK(actor_user_id>0),
 safe_note VARCHAR(512) NOT NULL CHECK(char_length(safe_note) BETWEEN 10 AND 512),
 verified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE OR REPLACE FUNCTION plugin_idc_resource_guard_non_acceptance_verification()
RETURNS TRIGGER AS $$ BEGIN
 RAISE EXCEPTION 'operator non-acceptance verification is immutable';
END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS guard_non_acceptance_verification ON plugin_idc_resource_provision_non_acceptance_verification;
CREATE TRIGGER guard_non_acceptance_verification BEFORE UPDATE OR DELETE
 ON plugin_idc_resource_provision_non_acceptance_verification FOR EACH ROW
 EXECUTE FUNCTION plugin_idc_resource_guard_non_acceptance_verification();
