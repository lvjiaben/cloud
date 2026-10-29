-- Purpose: Personal snapshots use the secret manager, while stored byte size belongs to enterprise uploads.
-- 用途：个人快照由密钥管理器保存，上传字节数仅约束企业快照，保留所有现有材料和处置证据。
ALTER TABLE plugin_idc_compliance_verification_case
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_verification_snapshot_size;
ALTER TABLE plugin_idc_compliance_verification_case
    ADD CONSTRAINT ck_plugin_idc_compliance_verification_snapshot_size CHECK (
        ("subject_type_code" = 'personal' AND "snapshot_stored_byte_size" = 0) OR
        ("subject_type_code" = 'enterprise' AND (
            ("legal_profile_snapshot_ref" IS NULL AND "snapshot_stored_byte_size" = 0) OR
            ("legal_profile_snapshot_ref" IS NOT NULL AND "snapshot_stored_byte_size" > 0)
        ))
    );
