// This file defines bounded PostgreSQL candidate projection and atomic lifecycle task insertion.

package lifecycle

const lifecycleCandidatesSQL = `
WITH due_fact AS (
    SELECT service.id AS service_id,
           service.service_no, service.customer_no, service.currency_code,
           policy.policy_no, policy.policy_version,
           'reminder'::VARCHAR(24) AS action_code,
           rule.reminder_sequence,
           service.version AS expected_service_version,
           service.entitlement_expires_at AS expected_entitlement_expires_at,
           service.entitlement_expires_at
               + rule.relative_seconds * INTERVAL '1 second' AS due_at
    FROM plugin_idc_core_service_instance service
    INNER JOIN plugin_idc_core_service_lifecycle_policy policy
        ON policy.tenant_id = service.tenant_id
       AND policy.policy_no = service.current_lifecycle_policy_no
       AND policy.policy_version = service.current_lifecycle_policy_version
       AND policy.service_no = service.service_no
    INNER JOIN plugin_idc_core_service_lifecycle_reminder_rule rule
        ON rule.tenant_id = policy.tenant_id
       AND rule.policy_no = policy.policy_no
       AND rule.policy_version = policy.policy_version
       AND rule.service_no = service.service_no
    WHERE service.tenant_id = 0
      AND service.entitlement_status_code IN ('active', 'grace', 'expired')
      AND service.entitlement_expires_at IS NOT NULL
      AND service.entitlement_expires_at
          + rule.relative_seconds * INTERVAL '1 second' <= CURRENT_TIMESTAMP

    UNION ALL

    SELECT service.id, service.service_no, service.customer_no, service.currency_code,
           policy.policy_no, policy.policy_version, action.action_code,
           NULL::INT, service.version, service.entitlement_expires_at,
           service.entitlement_expires_at + action.relative_seconds * INTERVAL '1 second'
    FROM plugin_idc_core_service_instance service
    INNER JOIN plugin_idc_core_service_lifecycle_policy policy
        ON policy.tenant_id = service.tenant_id
       AND policy.policy_no = service.current_lifecycle_policy_no
       AND policy.policy_version = service.current_lifecycle_policy_version
       AND policy.service_no = service.service_no
    CROSS JOIN LATERAL (
        VALUES
            ('enter_grace'::VARCHAR(24), 0::BIGINT,
                service.entitlement_status_code = 'active'),
            ('mark_expired'::VARCHAR(24), policy.grace_seconds,
                service.entitlement_status_code IN ('active', 'grace')),
            ('suspend'::VARCHAR(24), COALESCE(policy.suspend_after_seconds, 0),
                policy.suspend_enabled
                AND service.entitlement_status_code IN ('active', 'grace', 'expired')
                AND service.runtime_status_code IN ('running', 'stopped')),
            ('terminate'::VARCHAR(24), COALESCE(policy.terminate_after_seconds, 0),
                policy.terminate_enabled
                AND service.entitlement_status_code IN ('active', 'grace', 'expired')
                AND service.runtime_status_code IN ('running', 'stopped', 'suspended'))
    ) action(action_code, relative_seconds, eligible)
    WHERE service.tenant_id = 0
      AND service.entitlement_expires_at IS NOT NULL
      AND action.eligible
      AND service.entitlement_expires_at
          + action.relative_seconds * INTERVAL '1 second' <= CURRENT_TIMESTAMP

    UNION ALL

    SELECT service.id, service.service_no, service.customer_no, service.currency_code,
           policy.policy_no, policy.policy_version, 'resume'::VARCHAR(24),
           NULL::INT, service.version, service.entitlement_expires_at,
           CURRENT_TIMESTAMP
    FROM plugin_idc_core_service_instance service
    INNER JOIN plugin_idc_core_service_lifecycle_policy policy
        ON policy.tenant_id = service.tenant_id
       AND policy.policy_no = service.current_lifecycle_policy_no
       AND policy.policy_version = service.current_lifecycle_policy_version
       AND policy.service_no = service.service_no
    WHERE service.tenant_id = 0
      AND service.entitlement_status_code = 'active'
      AND service.runtime_status_code = 'suspended'
      AND service.entitlement_expires_at > CURRENT_TIMESTAMP
      AND EXISTS (
          SELECT 1
          FROM plugin_idc_core_service_lifecycle_task prior
          WHERE prior.tenant_id = service.tenant_id
            AND prior.service_no = service.service_no
            AND prior.action_code = 'suspend'
            AND prior.status_code = 'succeeded'
            AND prior.business_operation_no IS NOT NULL
            AND prior.business_operation_no = service.last_business_operation_no
      )
      AND NOT EXISTS (
          SELECT 1
          FROM plugin_idc_core_service_lifecycle_task irreversible
          WHERE irreversible.tenant_id = service.tenant_id
            AND irreversible.service_no = service.service_no
            AND irreversible.action_code = 'terminate'
            AND irreversible.status_code IN (
                'claimed', 'dispatched', 'succeeded', 'manual_review'
            )
      )
)
SELECT fact.service_no, fact.customer_no, fact.currency_code,
       fact.policy_no, fact.policy_version, fact.action_code,
       fact.reminder_sequence, fact.expected_service_version,
       fact.expected_entitlement_expires_at, fact.due_at
FROM due_fact fact
WHERE NOT EXISTS (
    SELECT 1
    FROM plugin_idc_core_service_lifecycle_task task
    WHERE task.tenant_id = 0
      AND task.service_no = fact.service_no
      AND task.policy_no = fact.policy_no
      AND task.policy_version = fact.policy_version
      AND task.expected_entitlement_expires_at = fact.expected_entitlement_expires_at
      AND task.action_code = fact.action_code
      AND COALESCE(task.reminder_sequence, 0) = COALESCE(fact.reminder_sequence, 0)
)
ORDER BY fact.due_at ASC, fact.service_id ASC, fact.action_code ASC,
         COALESCE(fact.reminder_sequence, 0) ASC
LIMIT ?`

const lifecycleInsertPrefix = `
WITH raw_input(
    task_no, service_no, customer_no, currency_code,
    policy_no, policy_version, action_code, reminder_sequence,
    expected_service_version, expected_entitlement_expires_at, due_at,
    idempotency_digest, content_digest
) AS (VALUES `

const lifecycleInsertSuffix = `),
input AS (
    SELECT
        task_no::VARCHAR(40) AS task_no,
        service_no::VARCHAR(40) AS service_no,
        customer_no::VARCHAR(40) AS customer_no,
        currency_code::CHAR(3) AS currency_code,
        policy_no::VARCHAR(40) AS policy_no,
        policy_version::BIGINT AS policy_version,
        action_code::VARCHAR(24) AS action_code,
        reminder_sequence::INT AS reminder_sequence,
        expected_service_version::BIGINT AS expected_service_version,
        expected_entitlement_expires_at::TIMESTAMPTZ AS expected_entitlement_expires_at,
        due_at::TIMESTAMPTZ AS due_at,
        idempotency_digest::CHAR(64) AS idempotency_digest,
        content_digest::CHAR(64) AS content_digest
    FROM raw_input
),
eligible AS (
    SELECT input.task_no, input.service_no, input.customer_no,
           input.currency_code, input.policy_no, input.policy_version,
           input.action_code, input.reminder_sequence,
           input.expected_service_version,
           input.expected_entitlement_expires_at, input.due_at,
           input.idempotency_digest, input.content_digest
    FROM input
    INNER JOIN plugin_idc_core_service_instance service
        ON service.tenant_id = 0
       AND service.service_no = input.service_no
       AND service.customer_no = input.customer_no
       AND service.currency_code = input.currency_code
       AND service.version = input.expected_service_version
       AND service.entitlement_expires_at = input.expected_entitlement_expires_at
       AND service.current_lifecycle_policy_no = input.policy_no
       AND service.current_lifecycle_policy_version = input.policy_version
    INNER JOIN plugin_idc_core_service_lifecycle_policy policy
        ON policy.tenant_id = service.tenant_id
       AND policy.policy_no = input.policy_no
       AND policy.policy_version = input.policy_version
       AND policy.service_no = service.service_no
    LEFT JOIN plugin_idc_core_service_lifecycle_reminder_rule rule
        ON rule.tenant_id = policy.tenant_id
       AND rule.policy_no = policy.policy_no
       AND rule.policy_version = policy.policy_version
       AND rule.service_no = service.service_no
       AND rule.reminder_sequence = NULLIF(input.reminder_sequence, 0)
    WHERE input.due_at <= CURRENT_TIMESTAMP
      AND CASE input.action_code
          WHEN 'reminder' THEN
              input.reminder_sequence BETWEEN 1 AND 16
              AND service.entitlement_status_code IN ('active', 'grace', 'expired')
              AND input.due_at = service.entitlement_expires_at
                  + rule.relative_seconds * INTERVAL '1 second'
          WHEN 'enter_grace' THEN
              service.entitlement_status_code = 'active'
              AND input.due_at = service.entitlement_expires_at
          WHEN 'mark_expired' THEN
              service.entitlement_status_code IN ('active', 'grace')
              AND input.due_at = service.entitlement_expires_at
                  + policy.grace_seconds * INTERVAL '1 second'
          WHEN 'suspend' THEN
              policy.suspend_enabled
              AND service.entitlement_status_code IN ('active', 'grace', 'expired')
              AND service.runtime_status_code IN ('running', 'stopped')
              AND input.due_at = service.entitlement_expires_at
                  + policy.suspend_after_seconds * INTERVAL '1 second'
          WHEN 'terminate' THEN
              policy.terminate_enabled
              AND service.entitlement_status_code IN ('active', 'grace', 'expired')
              AND service.runtime_status_code IN ('running', 'stopped', 'suspended')
              AND input.due_at = service.entitlement_expires_at
                  + policy.terminate_after_seconds * INTERVAL '1 second'
          WHEN 'resume' THEN
              input.reminder_sequence = 0
              AND service.entitlement_status_code = 'active'
              AND service.runtime_status_code = 'suspended'
              AND service.entitlement_expires_at > CURRENT_TIMESTAMP
              AND EXISTS (
                  SELECT 1
                  FROM plugin_idc_core_service_lifecycle_task prior
                  WHERE prior.tenant_id = service.tenant_id
                    AND prior.service_no = service.service_no
                    AND prior.action_code = 'suspend'
                    AND prior.status_code = 'succeeded'
                    AND prior.business_operation_no IS NOT NULL
                    AND prior.business_operation_no = service.last_business_operation_no
              )
              AND NOT EXISTS (
                  SELECT 1
                  FROM plugin_idc_core_service_lifecycle_task irreversible
                  WHERE irreversible.tenant_id = service.tenant_id
                    AND irreversible.service_no = service.service_no
                    AND irreversible.action_code = 'terminate'
                    AND irreversible.status_code IN (
                        'claimed', 'dispatched', 'succeeded', 'manual_review'
                    )
              )
          ELSE FALSE
      END
),
inserted_task AS (
INSERT INTO plugin_idc_core_service_lifecycle_task (
    tenant_id, task_no, service_no, customer_no, currency_code,
    policy_no, policy_version, action_code, reminder_sequence,
    expected_service_version, expected_entitlement_expires_at, due_at,
    status_code, idempotency_digest, content_digest
)
SELECT 0, task_no, service_no, customer_no, currency_code,
       policy_no, policy_version, action_code, NULLIF(reminder_sequence, 0),
       expected_service_version, expected_entitlement_expires_at, due_at,
       'pending', idempotency_digest, content_digest
FROM eligible
ON CONFLICT DO NOTHING
RETURNING task_no, service_no, idempotency_digest, content_digest
)
INSERT INTO plugin_idc_core_service_lifecycle_task_event (
    tenant_id, event_no, task_no, service_no, event_sequence,
    from_status_code, to_status_code, reason_code,
    actor_type_code, actor_reference, evidence_digest, content_digest
)
SELECT 0, 'LCE_' || substring(task_no FROM 5 FOR 32), task_no, service_no, 1,
       NULL, 'pending', 'lifecycle.scheduled',
       'system', 'lifecycle-scheduler', idempotency_digest, content_digest
FROM inserted_task`
