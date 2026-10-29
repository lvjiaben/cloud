-- Purpose: Add enterprise legal snapshots to retained-material disposition without invalidating historical targets.
-- 用途：企业法定资料快照接入现有保全和人工删除，原升级库的历史目标约束与证据继续保留。
DO $$
DECLARE
    target_table TEXT;
    target_constraint TEXT;
    original_expression TEXT;
BEGIN
    FOR target_table, target_constraint IN
        VALUES ('plugin_idc_compliance_privacy_retention_hold', 'ck_idc_compliance_privacy_hold_target_type'),
               ('plugin_idc_compliance_privacy_deletion_task', 'ck_idc_compliance_privacy_deletion_target_type')
    LOOP
        SELECT pg_get_expr(conbin, conrelid) INTO original_expression
        FROM pg_constraint
        WHERE conrelid = to_regclass(target_table) AND conname = target_constraint;
        IF original_expression IS NOT NULL AND position('enterprise_verification_case' IN original_expression) = 0 THEN
            EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', target_table, target_constraint);
            EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK ((%s) OR target_type_code = %L)',
                target_table, target_constraint, original_expression, 'enterprise_verification_case');
        END IF;
    END LOOP;
END $$;

-- A committed human approval survives provider or completion failures and can be resumed safely.
ALTER TABLE plugin_idc_compliance_privacy_deletion_task
    DROP CONSTRAINT IF EXISTS ck_idc_compliance_privacy_deletion_status;
ALTER TABLE plugin_idc_compliance_privacy_deletion_task
    ADD CONSTRAINT ck_idc_compliance_privacy_deletion_status CHECK (
        status_code IN ('pending', 'blocked', 'confirmed', 'completed')
    );
ALTER TABLE plugin_idc_compliance_privacy_deletion_task
    DROP CONSTRAINT IF EXISTS ck_idc_compliance_privacy_deletion_state;
ALTER TABLE plugin_idc_compliance_privacy_deletion_task
    ADD CONSTRAINT ck_idc_compliance_privacy_deletion_state CHECK (
        (status_code = 'pending' AND hold_reason_code = '' AND confirmed_at IS NULL AND completed_at IS NULL) OR
        (status_code = 'blocked' AND hold_reason_code <> '' AND completed_at IS NULL) OR
        (status_code = 'confirmed' AND hold_reason_code = '' AND confirmed_by_user_id > 0 AND confirmed_at IS NOT NULL AND completed_at IS NULL) OR
        (status_code = 'completed' AND hold_reason_code = '' AND confirmed_by_user_id > 0 AND confirmed_at IS NOT NULL AND completed_at IS NOT NULL)
    );
