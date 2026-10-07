// This helper seeds append-only, non-secret resource-owner facts for the
// explicit real-backend ZJMFCloud browser integration mode.

import { randomBytes } from "node:crypto";

import {
  execPgSQLStatements,
  pgEscapeLiteral,
  queryPgScalar,
} from "../../../../../../hack/tests/support/postgres";

export interface RealResourceFixture {
  channelName: string;
  channelNo: string;
  evidenceNo: string;
  externalResourceRef: string;
  externalTaskRef: string;
  taskNo: string;
}

function literal(value: string) {
  return `'${pgEscapeLiteral(value)}'`;
}

export function realResourceIntegrationEnabled() {
  return process.env.LINAPRO_IDC_ZJMFCLOUD_REAL_E2E === "1";
}

export function seedRealResourceFixture(): RealResourceFixture {
  const suffix = randomBytes(16).toString("hex");
  const fixture: RealResourceFixture = {
    channelName: "ZJMFCloud E2E",
    channelNo: `zjmf-e2e-${suffix.slice(0, 16)}`,
    evidenceNo: `evidence_${suffix}`,
    externalResourceRef: `resource-${suffix.slice(0, 12)}`,
    externalTaskRef: `task-${suffix.slice(0, 12)}`,
    taskNo: `task_${suffix}`,
  };
  const requestDigest = randomBytes(32).toString("hex");

  execPgSQLStatements([
    `INSERT INTO plugin_idc_resource_channel
      (channel_no,provider_id,provider_plugin_id,display_name,endpoint,
       non_secret_settings,status_code,created_by,updated_by)
     VALUES (${literal(fixture.channelNo)},'zjmfcloud','idc-zjmfcloud',
       ${literal(fixture.channelName)},'https://zjmfcloud-e2e.invalid','{}'::jsonb,'enabled',1,1);`,
    `INSERT INTO plugin_idc_resource_task
      (task_no,request_id_digest,capability_code,local_resource_no,channel_no,
       status_code,priority,sanitized_parameters,owner_metadata,attempt_count,
       max_attempts,next_observe_at,result_unknown,version)
     VALUES (${literal(fixture.taskNo)},${literal(requestDigest)},'server.start',
       ${literal(`resource-${suffix}`)},${literal(fixture.channelNo)},
       'result_unknown',100,'{}'::jsonb,'{}'::jsonb,1,8,
       CURRENT_TIMESTAMP + INTERVAL '30 seconds',TRUE,3);`,
    `INSERT INTO plugin_idc_resource_upstream_task_binding
      (binding_no,task_no,step_no,channel_no,external_task_ref,
       external_resource_ref,status_code,last_progress,last_observed_at)
     VALUES (${literal(`binding-${suffix}`)},${literal(fixture.taskNo)},1,
       ${literal(fixture.channelNo)},${literal(fixture.externalTaskRef)},
       ${literal(fixture.externalResourceRef)},'running',40,CURRENT_TIMESTAMP);`,
    `INSERT INTO plugin_idc_resource_task_evidence
      (evidence_no,task_no,source_kind,evidence_kind,reference,summary,observed_at)
     VALUES (${literal(fixture.evidenceNo)},${literal(fixture.taskNo)},
       'provider_task','task_observation',${literal(fixture.externalTaskRef)},
       'task.running',CURRENT_TIMESTAMP);`,
  ]);
  return fixture;
}

export function countRealCapacityRequests(channelNo: string) {
  return Number(
    queryPgScalar(
      `SELECT COUNT(*) FROM plugin_idc_resource_provider_capacity_observation
       WHERE channel_no=${literal(channelNo)} AND capacity_kind='image'`,
    ),
  );
}

export function realTaskStatus(taskNo: string) {
  return queryPgScalar(
    `SELECT status_code FROM plugin_idc_resource_task
     WHERE task_no=${literal(taskNo)}`,
  );
}
