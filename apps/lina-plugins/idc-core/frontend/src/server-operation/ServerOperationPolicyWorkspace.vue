<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import type { PluginHostMountContext } from '../host-context';
import type { ServerOperationPolicy } from './api';

import { createTranslator } from '../runtime/i18n';
import { createServerOperationPolicyAPI } from './api';
import { serverOperationMessages } from './messages';

const props = defineProps<{ context: PluginHostMountContext }>();
const currentContext = () => props.context;
const api = createServerOperationPolicyAPI(currentContext);
const tr = createTranslator(currentContext, serverOperationMessages);

const policy = ref<ServerOperationPolicy | null>(null);
const selected = ref(true);
const allowTransfer = ref(false);
const loading = ref(false);
const saving = ref(false);
const error = ref('');
const success = ref('');
const confirmDisable = ref(false);

const dirty = computed(
  () => policy.value && (selected.value !== policy.value.requireHighRiskReauth ||
    allowTransfer.value !== policy.value.allowCustomerTransfer),
);

function formatTime(value: string) {
  if (!value) return '—';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? '—' : parsed.toLocaleString();
}

async function load() {
  if (loading.value) return;
  loading.value = true;
  error.value = '';
  success.value = '';
  try {
    policy.value = await api.get();
    selected.value = policy.value.requireHighRiskReauth;
    allowTransfer.value = policy.value.allowCustomerTransfer;
  } catch {
    error.value = tr('serverOperation.error.load');
  } finally {
    loading.value = false;
  }
}

async function commit() {
  if (!policy.value || !dirty.value || saving.value) return;
  saving.value = true;
  error.value = '';
  success.value = '';
  try {
    policy.value = await api.update(selected.value, allowTransfer.value, policy.value.version);
    selected.value = policy.value.requireHighRiskReauth;
    allowTransfer.value = policy.value.allowCustomerTransfer;
    success.value = tr('serverOperation.saved');
    confirmDisable.value = false;
  } catch {
    error.value = tr('serverOperation.error.save');
  } finally {
    saving.value = false;
  }
}

function save() {
  if (!selected.value && policy.value?.requireHighRiskReauth) {
    confirmDisable.value = true;
    return;
  }
  void commit();
}

onMounted(load);
</script>

<template>
  <main class="idc-workspace" data-testid="server-operation-policy-page">
    <section class="panel workspace-toolbar">
      <div><h2>{{ tr('serverOperation.title') }}</h2></div>
      <button class="button secondary" data-testid="server-operation-policy-refresh" type="button" @click="load">
        {{ tr('serverOperation.action.refresh') }}
      </button>
    </section>

    <p v-if="error" class="notice danger" role="alert">{{ error }}</p>
    <p v-if="success" class="notice success" role="status">{{ success }}</p>
    <section v-if="loading && !policy" class="panel empty-state">{{ tr('serverOperation.loading') }}</section>

    <section v-else-if="policy" class="policy-layout">
      <article class="panel policy-card">
        <div class="policy-heading">
          <div>
            <span class="status-pill" :class="selected ? 'safe' : 'warning'">
              {{ tr(selected ? 'serverOperation.status.enabled' : 'serverOperation.status.disabled') }}
            </span>
            <h2>{{ tr('serverOperation.policyLabel') }}</h2>
          </div>
          <label class="policy-switch">
            <input v-model="selected" data-testid="server-operation-policy-toggle" type="checkbox" />
            <span aria-hidden="true"></span>
          </label>
        </div>
        <p class="policy-copy">{{ tr('serverOperation.enabledDescription') }}</p>
        <div class="policy-footer">
          <span>{{ tr('serverOperation.lastUpdated') }} · {{ formatTime(policy.updatedAt) }}</span>
          <button class="button primary" data-testid="server-operation-policy-save" :disabled="!dirty || saving" type="button" @click="save">
            {{ tr('serverOperation.action.save') }}
          </button>
        </div>
      </article>
      <article class="panel policy-card">
        <div class="policy-heading">
          <div>
            <span class="status-pill" :class="allowTransfer ? 'safe' : 'warning'">
              {{ tr(allowTransfer ? 'serverOperation.transfer.enabled' : 'serverOperation.transfer.disabled') }}
            </span>
            <h2>{{ tr('serverOperation.transfer.label') }}</h2>
          </div>
          <label class="policy-switch">
            <input v-model="allowTransfer" data-testid="server-transfer-policy-toggle" type="checkbox" />
            <span aria-hidden="true"></span>
          </label>
        </div>
        <p class="policy-copy">{{ tr('serverOperation.transfer.description') }}</p>
      </article>
    </section>

    <div v-if="confirmDisable" class="modal-backdrop" data-testid="server-operation-policy-disable-dialog" role="presentation" @click.self="confirmDisable = false">
      <section aria-modal="true" class="native-modal danger-modal" role="dialog">
        <span class="status-pill warning">{{ tr('serverOperation.status.disabled') }}</span>
        <h2>{{ tr('serverOperation.disableTitle') }}</h2>
        <p>{{ tr('serverOperation.disableDescription') }}</p>
        <div class="modal-actions">
          <button class="button ghost" type="button" @click="confirmDisable = false">{{ tr('serverOperation.action.cancel') }}</button>
          <button class="button danger" data-testid="server-operation-policy-confirm-disable" :disabled="saving" type="button" @click="commit">
            {{ tr('serverOperation.action.confirmDisable') }}
          </button>
        </div>
      </section>
    </div>
  </main>
</template>
