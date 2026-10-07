<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';

import type { PluginHostMountContext } from '../host-context';
import type { IPChangePolicy } from './api';

import { createTranslator } from '../runtime/i18n';
import { createIPChangePolicyAPI } from './api';
import { ipChangeMessages } from './messages';

const props = defineProps<{ context: PluginHostMountContext }>();
const api = createIPChangePolicyAPI(() => props.context);
const tr = createTranslator(() => props.context, ipChangeMessages);
const items = ref<IPChangePolicy[]>([]);
const loading = ref(false);
const saving = ref(false);
const error = ref('');
const success = ref('');
const form = reactive({
  amountMinor: 0,
  currencyCode: 'CNY',
  enabled: true,
  expectedVersion: 0,
  freeChangesPerWindow: 1,
  maxChangesPerDay: 2,
  maximumSnapshotAgeSeconds: 300,
  windowDays: 30,
});

function selectPolicy(policy?: IPChangePolicy) {
  form.expectedVersion = policy?.version ?? 0;
  form.enabled = policy?.enabled ?? true;
  form.freeChangesPerWindow = policy?.freeChangesPerWindow ?? 1;
  form.windowDays = policy?.windowDays ?? 30;
  form.maxChangesPerDay = policy?.maxChangesPerDay ?? 2;
  form.amountMinor = policy?.amountMinor ?? 0;
  form.maximumSnapshotAgeSeconds = policy?.maximumSnapshotAgeSeconds ?? 300;
}

async function load(selectCurrency = form.currencyCode) {
  loading.value = true;
  error.value = '';
  try {
    items.value = await api.list();
    selectPolicy(items.value.find((item) => item.currencyCode === selectCurrency));
  } catch {
    error.value = tr('ipChange.error.load');
  } finally {
    loading.value = false;
  }
}

async function publish() {
  if (saving.value) return;
  saving.value = true;
  error.value = '';
  success.value = '';
  try {
    const policy = await api.publish({ ...form, currencyCode: form.currencyCode.trim().toUpperCase() });
    form.currencyCode = policy.currencyCode;
    success.value = tr('ipChange.saved');
    await load(policy.currencyCode);
  } catch {
    error.value = tr('ipChange.error.save');
  } finally {
    saving.value = false;
  }
}

onMounted(() => load());
</script>

<template>
  <main class="idc-workspace" data-testid="ip-change-policy-page">
    <section class="panel workspace-toolbar">
      <div><h2>{{ tr('ipChange.title') }}</h2></div>
      <button class="button secondary" type="button" @click="load()">{{ tr('ipChange.action.refresh') }}</button>
    </section>
    <p v-if="error" class="notice danger" role="alert">{{ error }}</p>
    <p v-if="success" class="notice success" role="status">{{ success }}</p>
    <section v-if="loading && !items.length" class="panel empty-state">{{ tr('ipChange.loading') }}</section>
    <section v-else class="panel policy-card">
      <div class="form-grid">
        <label>{{ tr('ipChange.currency') }}<input v-model="form.currencyCode" maxlength="3" @change="load(form.currencyCode.trim().toUpperCase())" /></label>
        <label>{{ tr('ipChange.version') }}<input :value="form.expectedVersion" disabled /></label>
        <label>{{ tr('ipChange.window') }}<input v-model.number="form.windowDays" min="1" max="365" type="number" /></label>
        <label>{{ tr('ipChange.free') }}<input v-model.number="form.freeChangesPerWindow" min="0" max="100" type="number" /></label>
        <label>{{ tr('ipChange.maxDaily') }}<input v-model.number="form.maxChangesPerDay" min="1" max="20" type="number" /></label>
        <label>{{ tr('ipChange.amount') }}<input v-model.number="form.amountMinor" min="0" max="1000000000" type="number" /></label>
        <label>{{ tr('ipChange.snapshotAge') }}<input v-model.number="form.maximumSnapshotAgeSeconds" min="60" max="86400" type="number" /></label>
        <label class="policy-switch-row"><input v-model="form.enabled" type="checkbox" />{{ tr('ipChange.enabled') }}</label>
      </div>
      <div class="policy-footer"><span class="status-pill" :class="form.enabled ? 'safe' : 'warning'">{{ tr(form.enabled ? 'ipChange.status.enabled' : 'ipChange.status.disabled') }}</span><button class="button primary" :disabled="saving" type="button" @click="publish">{{ tr('ipChange.action.publish') }}</button></div>
    </section>
  </main>
</template>
