<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import type { IsolatedTableColumn } from "@linapro/vben-plugin-adapter/isolated";

import type { PluginHostMountContext } from "../host-context";
import type {
  ServerBatch,
  ServerBatchAction,
  ServerBatchPreviewItem,
  ServerClassification,
} from "./api";

import { createTranslator } from "../runtime/i18n";
import IsolatedVbenGrid from "../runtime/IsolatedVbenGrid.vue";
import { createServerBatchAPI } from "./api";
import { serverBatchMessages } from "./messages";

const props = defineProps<{ context: PluginHostMountContext }>();
const currentContext = () => props.context;
const api = createServerBatchAPI(currentContext);
const tr = createTranslator(currentContext, serverBatchMessages);

const action = ref<ServerBatchAction>("renewal_reminder");
const services = ref("");
const detailValue = ref("");
const preview = ref<ServerBatch>();
const acknowledged = ref(false);
const loading = ref(false);
const error = ref("");
const success = ref("");
const previewGridVersion = ref(0);
const previewGridReady = ref(false);
const previewGridFailed = ref(false);
const classifications = ref<ServerClassification[]>([]);
const classificationOpen = ref(false);
const classificationNameZh = ref("");
const classificationNameEn = ref("");
const editingClassification = ref<ServerClassification>();

const actions: ServerBatchAction[] = [
  "renewal_reminder",
  "tag_add",
  "tag_remove",
  "classification_set",
  "note_append",
  "reboot",
  "suspend",
  "reinstall",
  "ip_change",
  "terminate",
];
const dangerous = computed(() =>
  ["reboot", "suspend", "reinstall", "ip_change", "terminate"].includes(
    action.value,
  ),
);
const eligibleCount = computed(
  () =>
    preview.value?.items.filter((item) => item.previewStatus === "eligible")
      .length ?? 0,
);

watch([action, services, detailValue], () => {
  preview.value = undefined;
  previewGridReady.value = false;
  previewGridFailed.value = false;
  acknowledged.value = false;
  success.value = "";
});

function parseItems(): ServerBatchPreviewItem[] | undefined {
  const lines = services.value
    .split(/\r?\n/u)
    .map((line) => line.trim())
    .filter(Boolean);
  if (lines.length < 1 || lines.length > 20) return undefined;
  const seen = new Set<string>();
  const items: ServerBatchPreviewItem[] = [];
  for (const line of lines) {
    const [serviceNo = "", image = "", ...extra] = line
      .split("|")
      .map((value) => value.trim());
    if (!serviceNo || seen.has(serviceNo) || extra.length > 0) return undefined;
    seen.add(serviceNo);
    let parameters: Record<string, number | string> = {};
    if (action.value === "reinstall") {
      if (!/^\d+$/u.test(image)) return undefined;
      parameters = { os: Number(image) };
    } else if (image) return undefined;
    if (action.value === "tag_add" || action.value === "tag_remove") {
      const tag = detailValue.value.trim();
      if (!/^[a-z0-9][a-z0-9._-]{0,39}$/u.test(tag)) return undefined;
      parameters = { tag };
    }
    if (action.value === "note_append") {
      const note = detailValue.value.trim();
      if (!note || note.length > 1000) return undefined;
      parameters = { note };
    }
    if (action.value === "classification_set") {
      if (
        !classifications.value.some(
          (entry) =>
            entry.status === "active" &&
            entry.classificationNo === detailValue.value,
        )
      )
        return undefined;
      parameters = { classificationNo: detailValue.value };
    }
    items.push({ parameters, serviceNo });
  }
  return items;
}

async function loadClassifications() {
  try {
    classifications.value = await api.classifications();
  } catch {
    classifications.value = [];
  }
}

async function createClassification() {
  const nameZhCN = classificationNameZh.value.trim();
  const nameEnUS = classificationNameEn.value.trim();
  if (!nameZhCN || !nameEnUS || loading.value) return;
  loading.value = true;
  try {
    await api.saveClassification({
      classificationNo: editingClassification.value?.classificationNo,
      expectedVersion: editingClassification.value?.version,
      nameZhCN,
      nameEnUS,
      requestNo: requestNo(),
    });
    classificationNameZh.value = "";
    classificationNameEn.value = "";
    editingClassification.value = undefined;
    await loadClassifications();
  } finally {
    loading.value = false;
  }
}

function editClassification(item?: ServerClassification) {
  editingClassification.value = item;
  classificationNameZh.value = item?.nameZhCN ?? "";
  classificationNameEn.value = item?.nameEnUS ?? "";
}

async function disableClassification(item: ServerClassification) {
  if (loading.value) return;
  loading.value = true;
  try {
    await api.disableClassification(item);
    await loadClassifications();
  } finally {
    loading.value = false;
  }
}

function requestNo() {
  return `server-batch-${crypto.randomUUID()}`;
}

async function buildPreview() {
  const items = parseItems();
  if (!items || loading.value) {
    error.value = tr("serverBatch.error.input");
    return;
  }
  loading.value = true;
  error.value = "";
  success.value = "";
  previewGridReady.value = false;
  previewGridFailed.value = false;
  try {
    preview.value = await api.preview({
      action: action.value,
      items,
      requestNo: requestNo(),
    });
  } catch {
    error.value = tr("serverBatch.error.preview");
  } finally {
    loading.value = false;
  }
}

async function execute() {
  if (
    !preview.value ||
    !previewGridReady.value ||
    !acknowledged.value ||
    loading.value
  )
    return;
  loading.value = true;
  error.value = "";
  success.value = "";
  try {
    let proofToken = "";
    if (preview.value.dangerous) {
      const proof = await api.confirm(preview.value);
      proofToken = proof.proofToken;
    }
    preview.value = await api.execute(preview.value, proofToken);
    success.value = tr("serverBatch.success");
  } catch {
    error.value = tr("serverBatch.error.execute");
  } finally {
    loading.value = false;
  }
}

function onPreviewGridFailed() {
  previewGridReady.value = false;
  previewGridFailed.value = true;
  error.value = tr("serverBatch.error.preview");
}

function retryPreviewGrid() {
  if (!previewGridFailed.value) return;
  error.value = "";
  previewGridFailed.value = false;
  previewGridReady.value = false;
  previewGridVersion.value += 1;
}

function formatMoney(value: number, currency: string) {
  try {
    return new Intl.NumberFormat(props.context.locale, {
      currency,
      style: "currency",
    }).format(value / 100);
  } catch {
    return `${currency} ${(value / 100).toFixed(2)}`;
  }
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString(props.context.locale) : "—";
}

function itemState(item: ServerBatch["items"][number]) {
  if (item.executionStatus !== "pending")
    return tr(`serverBatch.state.${item.executionStatus}`);
  return tr(`serverBatch.state.${item.previewStatus}`);
}

function stackedCell(primary: string, ...details: string[]) {
  const content = document.createElement("div");
  const heading = document.createElement("span");
  heading.textContent = primary;
  content.append(heading);
  for (const text of details.filter(Boolean)) {
    const line = document.createElement("small");
    line.style.display = "block";
    line.textContent = text;
    content.append(line);
  }
  return content;
}
const previewColumns = computed<
  IsolatedTableColumn<ServerBatch["items"][number]>[]
>(() => [
  {
    field: "serviceNo",
    title: tr("serverBatch.column.server"),
    minWidth: 200,
    cell: (item) =>
      stackedCell(
        item.serviceNo,
        `${item.entitlementStatus} · ${item.runtimeStatus}`,
      ),
  },
  {
    field: "customerNo",
    title: tr("serverBatch.column.customer"),
    minWidth: 140,
  },
  {
    field: "unpaidAmountMinor",
    title: tr("serverBatch.column.funds"),
    minWidth: 140,
    cell: (item) =>
      stackedCell(
        formatMoney(item.unpaidAmountMinor, item.currencyCode),
        `+ ${formatMoney(item.actionAmountMinor, item.currencyCode)}`,
      ),
  },
  {
    field: "entitlementExpiresAt",
    title: tr("serverBatch.column.expiry"),
    minWidth: 180,
    cell: (item) => formatTime(item.entitlementExpiresAt),
  },
  {
    field: "resourceSummary",
    title: tr("serverBatch.column.resource"),
    minWidth: 150,
  },
  {
    field: "irreversible",
    title: tr("serverBatch.irreversible"),
    width: 100,
    cell: (item) => {
      const element = document.createElement("span");
      element.className = item.irreversible
        ? "irreversible-mark yes"
        : "irreversible-mark";
      element.textContent = tr(
        item.irreversible ? "serverBatch.yes" : "serverBatch.no",
      );
      return element;
    },
  },
  {
    field: "executionStatus",
    title: tr("serverBatch.column.result"),
    minWidth: 180,
    cell: (item) => {
      const element = stackedCell(
        "",
        item.failureCode ?? "",
        item.operationNo ?? "",
      );
      const status = element.firstElementChild as HTMLElement;
      status.className = `result-pill ${item.executionStatus !== "pending" ? item.executionStatus : item.previewStatus}`;
      status.textContent = itemState(item);
      return element;
    },
  },
]);

onMounted(loadClassifications);
</script>

<template>
  <main
    class="idc-workspace server-batch-workspace"
    data-testid="server-batch-page"
  >
    <section class="panel workspace-toolbar">
      <div>
        <h2>{{ tr("serverBatch.title") }}</h2>
      </div>
      <span class="batch-limit">01—20</span>
    </section>

    <p v-if="error" class="notice danger" role="alert">
      {{ error }}
      <button
        v-if="previewGridFailed"
        class="button ghost"
        type="button"
        @click="retryPreviewGrid"
      >
        {{ tr("serverBatch.retry") }}
      </button>
    </p>
    <p v-if="success" class="notice success" role="status">{{ success }}</p>

    <section v-if="!preview" class="batch-builder">
      <aside class="batch-rail" :aria-label="tr('serverBatch.title')">
        <button
          v-for="value in actions"
          :key="value"
          class="rail-action"
          :class="{
            active: action === value,
            risky: [
              'reboot',
              'suspend',
              'reinstall',
              'ip_change',
              'terminate',
            ].includes(value),
          }"
          type="button"
          @click="action = value"
        >
          <span>{{ tr(`serverBatch.action.${value}`) }}</span>
          <small>{{
            [
              "reboot",
              "suspend",
              "reinstall",
              "ip_change",
              "terminate",
            ].includes(value)
              ? tr("serverBatch.danger")
              : tr("serverBatch.safe")
          }}</small>
        </button>
      </aside>
      <form
        class="panel batch-form"
        data-testid="server-batch-form"
        @submit.prevent="buildPreview"
      >
        <div class="batch-form-heading">
          <div>
            <span class="status-pill" :class="dangerous ? 'warning' : 'safe'">{{
              dangerous ? tr("serverBatch.danger") : tr("serverBatch.safe")
            }}</span>
            <h2>{{ tr(`serverBatch.action.${action}`) }}</h2>
          </div>
          <strong>{{ tr("serverBatch.limit") }}</strong>
        </div>
        <label class="field">
          <span>{{ tr("serverBatch.field.services") }}</span>
          <textarea
            v-model="services"
            data-testid="server-batch-services"
            rows="9"
            spellcheck="false"
            required
          />
          <small>{{ tr("serverBatch.field.servicesHelp") }}</small>
        </label>
        <label
          v-if="action === 'tag_add' || action === 'tag_remove'"
          class="field"
        >
          <span>{{ tr("serverBatch.field.tag") }}</span>
          <input
            v-model="detailValue"
            data-testid="server-batch-tag"
            maxlength="40"
            required
          />
          <small>{{ tr("serverBatch.field.tagHelp") }}</small>
        </label>
        <label v-if="action === 'note_append'" class="field">
          <span>{{ tr("serverBatch.field.note") }}</span>
          <textarea
            v-model="detailValue"
            data-testid="server-batch-note"
            maxlength="1000"
            rows="4"
            required
          />
          <small>{{ tr("serverBatch.field.noteHelp") }}</small>
        </label>
        <label v-if="action === 'classification_set'" class="field">
          <span>{{ tr("serverBatch.field.classification") }}</span>
          <select
            v-model="detailValue"
            data-testid="server-batch-classification"
            required
          >
            <option value="" disabled>
              {{ tr("serverBatch.classification.empty") }}
            </option>
            <option
              v-for="item in classifications.filter(
                (entry) => entry.status === 'active',
              )"
              :key="item.classificationNo"
              :value="item.classificationNo"
            >
              {{
                context.locale.toLowerCase().startsWith("zh")
                  ? item.nameZhCN
                  : item.nameEnUS
              }}
            </option>
          </select>
          <button
            class="button ghost manage-classifications"
            type="button"
            @click="classificationOpen = true"
          >
            {{ tr("serverBatch.classification.manage") }}
          </button>
        </label>
        <p v-if="dangerous" class="danger-callout">
          <strong>{{ tr("serverBatch.recentLogin") }}</strong
          ><span>{{ tr("serverBatch.dangerHelp") }}</span>
        </p>
        <button
          class="button primary preview-button"
          data-testid="server-batch-preview"
          :disabled="loading"
          type="submit"
        >
          {{ tr("serverBatch.action.preview") }}
        </button>
      </form>
    </section>

    <div
      v-if="classificationOpen"
      class="modal-backdrop"
      role="presentation"
      @click.self="classificationOpen = false"
    >
      <section
        class="native-modal classification-modal"
        role="dialog"
        aria-modal="true"
        data-testid="server-classification-manager"
      >
        <h2>{{ tr("serverBatch.classification.manage") }}</h2>
        <form
          class="classification-form"
          @submit.prevent="createClassification"
        >
          <label class="field"
            ><span>{{ tr("serverBatch.classification.nameZh") }}</span
            ><input v-model="classificationNameZh" maxlength="80" required
          /></label>
          <label class="field"
            ><span>{{ tr("serverBatch.classification.nameEn") }}</span
            ><input v-model="classificationNameEn" maxlength="80" required
          /></label>
          <button class="button primary" :disabled="loading" type="submit">
            {{
              tr(
                editingClassification
                  ? "serverBatch.classification.save"
                  : "serverBatch.classification.new",
              )
            }}
          </button>
        </form>
        <button
          v-if="editingClassification"
          class="button ghost edit-cancel"
          type="button"
          @click="editClassification()"
        >
          {{ tr("serverBatch.classification.cancel") }}
        </button>
        <article
          v-for="item in classifications"
          :key="item.classificationNo"
          class="classification-row"
        >
          <div>
            <strong>{{ item.nameZhCN }}</strong
            ><small>{{ item.nameEnUS }} · {{ item.status }}</small>
          </div>
          <div class="classification-actions">
            <button
              class="button ghost"
              type="button"
              @click="editClassification(item)"
            >
              {{ tr("serverBatch.classification.edit") }}</button
            ><button
              v-if="item.status === 'active'"
              class="button danger"
              type="button"
              @click="disableClassification(item)"
            >
              {{ tr("serverBatch.classification.disable") }}
            </button>
          </div>
        </article>
        <footer class="modal-actions">
          <button
            class="button ghost"
            type="button"
            @click="classificationOpen = false"
          >
            ×
          </button>
        </footer>
      </section>
    </div>

    <section
      v-if="preview"
      class="preview-stage"
      data-testid="server-batch-preview-panel"
    >
      <div class="preview-heading">
        <div>
          <p class="panel-kicker">
            {{ tr("serverBatch.batchNo") }} · {{ preview.batchNo }}
          </p>
          <h2>{{ tr(`serverBatch.action.${preview.action}`) }}</h2>
          <p>
            {{
              tr("serverBatch.previewIntro", {
                eligible: eligibleCount,
                total: preview.items.length,
              })
            }}
          </p>
        </div>
        <span class="impact-count"
          >{{ eligibleCount }}<small>/ {{ preview.items.length }}</small></span
        >
      </div>
      <div v-if="preview.dangerous" class="danger-banner">
        <strong>{{ tr("serverBatch.danger") }}</strong
        ><span>{{ tr("serverBatch.dangerHelp") }}</span>
      </div>
      <IsolatedVbenGrid
        :key="previewGridVersion"
        :columns="previewColumns"
        :rows="preview.items"
        row-key="lineNumber"
        :height="400"
        :locale="context.locale"
        :loading="loading"
        @failed="onPreviewGridFailed"
        @ready="previewGridReady = true"
      />
      <footer v-if="preview.status !== 'completed'" class="preview-actions">
        <label class="acknowledge"
          ><input
            v-model="acknowledged"
            data-testid="server-batch-acknowledge"
            :disabled="!previewGridReady"
            type="checkbox"
          /><span>{{ tr("serverBatch.ack") }}</span></label
        >
        <div>
          <button
            class="button ghost"
            type="button"
            @click="preview = undefined"
          >
            {{ tr("serverBatch.cancelPreview") }}</button
          ><button
            class="button"
            :class="preview.dangerous ? 'danger' : 'primary'"
            data-testid="server-batch-execute"
            :disabled="
              !acknowledged ||
              !previewGridReady ||
              loading ||
              eligibleCount === 0
            "
            type="button"
            @click="execute"
          >
            {{ tr("serverBatch.action.execute") }}
          </button>
        </div>
      </footer>
    </section>
  </main>
</template>
