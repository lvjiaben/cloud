<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Alert, Button, Descriptions, Modal, Select, message } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { downloadFileFromBlob } from "@vben/utils";
import { pluginApiPath, requestClient } from "#/api/request";
import { formatTimestamp } from "#/utils/time";
import { useAccess } from "../plugin-access";
import { actionLabel, copy, statusLabel } from "../../../../yunsu-ssl/frontend/src/shared/copy";
import { parseCertificateDetail, type CertificateItem, type CertificateAction } from "../../../../yunsu-ssl/frontend/src/shared/model";

const { hasAccessByCodes } = useAccess();
const canAct = (action: CertificateAction) => hasAccessByCodes([`idc:admin:service-entitlement:${action === "download" ? "download" : "action"}`]);
const props = defineProps<{ serviceNo: string }>();
const locale = computed(() => preferences.app.locale === "en-US" ? "en-US" : "zh-CN");
const words = computed(() => copy[locale.value]);
const item = ref<CertificateItem>();
const busy = ref(false);
const failed = ref(false);
const method = ref("dns");
const changingMethod = ref(false);
let generation = 0;
const path = (suffix = "") => pluginApiPath("idc-core", `admin/service-entitlements/${encodeURIComponent(props.serviceNo)}${suffix}`);
async function load() {
  const current = ++generation;
  changingMethod.value = false;
  failed.value = false;
  busy.value = true;
  item.value = undefined;
  try {
    const result = parseCertificateDetail(await requestClient.get(path()));
    if (current !== generation) return;
    if (!result || result.entitlementNo !== props.serviceNo) throw new Error("invalid certificate");
    item.value = result;
  } catch {
    if (current === generation) failed.value = true;
  } finally {
    if (current === generation) busy.value = false;
  }
}
async function execute(action: CertificateAction, validationMethod?: string) {
  if (busy.value || !item.value || !canAct(action) || !item.value.availableActions.includes(action)) return;
  const current = generation;
  busy.value = true;
  try {
    const result = await requestClient.post<{ ticket?: string; fileName?: string }>(
      path(action === "download" ? "/downloads/certificate/tickets" : `/actions/${action}`),
      { idempotencyKey: crypto.randomUUID(), input: validationMethod ? { validationMethod } : {} },
    );
    if (current !== generation) return;
    if (action === "download") {
      if (!result.ticket || !/^[A-Za-z0-9_-]{32,128}$/.test(result.ticket)) throw new Error("invalid ticket");
      const source = await requestClient.download<Blob>(pluginApiPath("idc-core", `admin/service-downloads/${result.ticket}`));
      if (current !== generation) return;
      downloadFileFromBlob({ source, fileName: result.fileName || "certificate.zip" });
    } else {
      changingMethod.value = false;
      await load();
    }
  } catch {
    if (current === generation) message.error(words.value.actionFailed);
  } finally {
    if (current === generation) busy.value = false;
  }
}
function act(action: CertificateAction) {
  if (busy.value || !item.value || !canAct(action) || !item.value.availableActions.includes(action)) return;
  if (action === "update-validation") {
    method.value = item.value?.validationMethod || "dns";
    changingMethod.value = true;
  } else if (action === "cancel") {
    const current = generation;
    Modal.confirm({ title: words.value.cancelConfirm, onOk: () => current === generation ? execute(action) : undefined });
  } else void execute(action);
}
watch(() => props.serviceNo, () => { changingMethod.value = false; void load(); }, { immediate: true });
onBeforeUnmount(() => ++generation);
</script>
<template>
  <section data-testid="customer-certificate-management" class="flex flex-col gap-4">
    <Alert v-if="failed" type="error" :message="words.error"><template #action><Button @click="load">{{ words.retry }}</Button></template></Alert>
    <Button :loading="busy" @click="load">{{ words.refresh }}</Button>
    <template v-if="item">
      <Descriptions bordered :column="1">
        <Descriptions.Item :label="words.domain">{{ item.primaryDomain }}</Descriptions.Item>
        <Descriptions.Item :label="words.status">{{ statusLabel(item.statusCode, locale) }}</Descriptions.Item>
        <Descriptions.Item :label="words.method">{{ item.validationMethod === 'dns' ? words.methodDns : words.methodFile }}</Descriptions.Item>
        <Descriptions.Item :label="words.issuedAt">{{ item.issuedAt ? formatTimestamp(item.issuedAt) : '—' }}</Descriptions.Item>
        <Descriptions.Item :label="words.expiresAt">{{ item.expiresAt ? formatTimestamp(item.expiresAt) : '—' }}</Descriptions.Item>
      </Descriptions>
      <div class="flex flex-wrap gap-2"><Button v-for="action in item.availableActions.filter(canAct)" :key="action" :danger="action === 'cancel'" :loading="busy" @click="act(action)">{{ actionLabel(action, locale) }}</Button></div>
      <section v-if="changingMethod" class="flex flex-wrap gap-2">
        <Select v-model:value="method" class="min-w-48" :options="[{ value: 'dns', label: words.methodDns }, { value: 'file', label: words.methodFile }]" />
        <Button :loading="busy" @click="execute('update-validation', method)">{{ words.save }}</Button>
        <Button :disabled="busy" @click="changingMethod = false">{{ words.close }}</Button>
      </section>
      <h3>{{ words.validation }}</h3>
      <Descriptions v-for="(record, index) in item.validation" :key="index" bordered :column="1">
        <Descriptions.Item :label="words.domain">{{ record.domain }}</Descriptions.Item>
        <Descriptions.Item v-if="record.host" :label="words.host">{{ record.host }}</Descriptions.Item>
        <Descriptions.Item v-if="record.value" :label="words.value"><span class="break-all">{{ record.value }}</span></Descriptions.Item>
        <Descriptions.Item v-if="record.fileName" :label="words.fileName">{{ record.fileName }}</Descriptions.Item>
        <Descriptions.Item v-if="record.path" :label="words.path">{{ record.path }}</Descriptions.Item>
      </Descriptions>
    </template>
  </section>
</template>
