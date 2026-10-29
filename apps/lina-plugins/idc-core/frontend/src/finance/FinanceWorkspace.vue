<script setup lang="ts">
import type { PluginHostMountContext } from "../host-context";
import type { IsolatedTableColumn, IsolatedTablePagination } from "@linapro/vben-plugin-adapter/isolated";
import type { CreditAccount, CreditApplication, CreditStatement } from "./api";
import { computed, ref } from "vue";

import IsolatedVbenGrid from "../runtime/IsolatedVbenGrid.vue";
import { useCreditWorkspace } from "./use-credit-workspace";

const props = withDefaults(
  defineProps<{ context: PluginHostMountContext; embedded?: boolean }>(),
  { embedded: false },
);
const {
  accountPageRows,
  activeTab,
  applicationPageRows,
  detailOpen,
  error,
  evidence,
  formatDate,
  load: loadCredit,
  money,
  loadPage,
  notice,
  openDetail,
  openEvidence,
  openReview,
  pageState,
  pageLoading,
  reviewForm,
  reviewLoading,
  reviewOpen,
  selected,
  statementPageRows,
  status,
  submitReview,
  tr,
} = useCreditWorkspace(props);
const pagination = computed(() => Object.fromEntries(
  (["applications", "accounts", "statements"] as const).map((kind) => [
    kind,
    { ...pageState[kind], onChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => { void loadPage(kind, currentPage, pageSize); } },
  ]),
) as Record<"applications" | "accounts" | "statements", IsolatedTablePagination>);
const gridLoadFailed = ref(false);
const gridMountVersion = ref(0);

function onGridFailed() {
  gridLoadFailed.value = true;
  error.value = tr("finance.credit.failed");
}

async function load() {
  if (gridLoadFailed.value) {
    gridLoadFailed.value = false;
    gridMountVersion.value += 1;
  }
  await loadCredit();
}

function statusCell(code: string) {
  const element = document.createElement("span");
  element.className = "stage-badge";
  element.dataset.stage = code;
  element.textContent = status(code);
  return element;
}
function applicationActions(row: CreditApplication) {
  const group = document.createElement("div");
  group.className = "row-actions";
  const detail = document.createElement("button");
  detail.type = "button";
  detail.className = "button ghost";
  detail.dataset.testid = "credit-application-detail";
  detail.textContent = tr("finance.credit.detail");
  detail.addEventListener("click", () => void openDetail(row));
  group.append(detail);
  if (row.statusCode === "pending_review") {
    const review = document.createElement("button");
    review.type = "button";
    review.className = "button primary";
    review.dataset.testid = "credit-application-review";
    review.textContent = tr("finance.credit.review");
    review.addEventListener("click", () => openReview(row));
    group.append(review);
  }
  return group;
}
const applicationColumns = computed<IsolatedTableColumn<CreditApplication>[]>(
  () => [
    {
      field: "applicationNo",
      title: tr("finance.credit.application"),
      minWidth: 200,
    },
    {
      field: "customerNo",
      title: tr("finance.credit.customer"),
      minWidth: 120,
    },
    {
      field: "requestedLimitMinor",
      title: tr("finance.credit.amount"),
      minWidth: 140,
      cell: (row) => money(row.requestedLimitMinor, row.currencyCode),
    },
    {
      field: "reviewCount",
      title: tr("finance.credit.progress"),
      width: 100,
      cell: (row) => `${row.reviewCount}/${row.requiredApprovalCount}`,
    },
    {
      field: "statusCode",
      title: tr("finance.credit.status"),
      width: 120,
      cell: (row) => statusCell(row.statusCode),
    },
    {
      field: "actions",
      title: tr("finance.credit.action"),
      width: 160,
      fixed: "right",
      cell: applicationActions,
    },
  ],
);
const accountColumns = computed<
  IsolatedTableColumn<CreditAccount & { usedMinor: string }>[]
>(() => [
  {
    field: "creditAccountNo",
    title: tr("finance.credit.account"),
    minWidth: 200,
  },
  { field: "customerNo", title: tr("finance.credit.customer"), minWidth: 120 },
  { field: "currencyCode", title: tr("finance.credit.currency"), width: 90 },
  {
    field: "availableLimitMinor",
    title: tr("finance.credit.available"),
    minWidth: 140,
    cell: (row) => money(row.availableLimitMinor, row.currencyCode),
  },
  {
    field: "usedMinor",
    title: tr("finance.credit.used"),
    minWidth: 140,
    cell: (row) => money(row.usedMinor, row.currencyCode),
  },
  {
    field: "statusCode",
    title: tr("finance.credit.status"),
    width: 120,
    cell: (row) => statusCell(row.statusCode),
  },
]);
const statementColumns = computed<IsolatedTableColumn<CreditStatement>[]>(
  () => [
    {
      field: "statementNo",
      title: tr("finance.credit.statement"),
      minWidth: 200,
    },
    {
      field: "customerNo",
      title: tr("finance.credit.customer"),
      minWidth: 120,
    },
    {
      field: "totalAmountMinor",
      title: tr("finance.credit.amount"),
      minWidth: 140,
      cell: (row) => money(row.totalAmountMinor, row.currencyCode),
    },
    {
      field: "outstandingAmountMinor",
      title: tr("finance.credit.outstanding"),
      minWidth: 140,
      cell: (row) => money(row.outstandingAmountMinor, row.currencyCode),
    },
    {
      field: "paymentDueAt",
      title: tr("finance.credit.due"),
      width: 140,
      cell: (row) => formatDate(row.paymentDueAt),
    },
    {
      field: "statusCode",
      title: tr("finance.credit.status"),
      width: 120,
      cell: (row) => statusCell(row.statusCode),
    },
  ],
);
</script>

<template>
  <component
    :is="props.embedded ? 'section' : 'main'"
    class="idc-workspace finance-workspace"
    :class="{ 'credit-workspace-embedded': props.embedded }"
    :data-testid="props.embedded ? undefined : 'financial-management-page'"
  >
    <section v-if="!props.embedded" class="panel workspace-toolbar">
      <div>
        <h2>{{ tr("finance.credit.title") }}</h2>
      </div>
      <button class="button secondary" type="button" @click="load">
        {{ tr("finance.credit.retry") }}
      </button>
    </section>

    <p v-if="error" class="inline-alert danger-alert">
      {{ error }}
      <button class="button ghost" type="button" @click="load">
        {{ tr("finance.credit.retry") }}
      </button>
    </p>
    <p v-if="notice" class="inline-alert">{{ notice }}</p>

    <section class="panel finance-panel" data-testid="credit-management-panel">
      <div class="panel-heading">
        <div>
          <p class="panel-kicker">{{ tr("finance.credit.control") }}</p>
          <h2>{{ tr("finance.credit.management") }}</h2>
        </div>
      </div>
      <nav class="workspace-tabs" :aria-label="tr('finance.credit.management')">
        <button
          data-testid="credit-applications-tab"
          :class="{ active: activeTab === 'applications' }"
          type="button"
          @click="activeTab = 'applications'"
        >
          {{ tr("finance.credit.applications") }}
        </button>
        <button
          :class="{ active: activeTab === 'accounts' }"
          type="button"
          @click="activeTab = 'accounts'"
        >
          {{ tr("finance.credit.accounts") }}
        </button>
        <button
          :class="{ active: activeTab === 'statements' }"
          type="button"
          @click="activeTab = 'statements'"
        >
          {{ tr("finance.credit.statements") }}
        </button>
      </nav>

      <template v-if="activeTab === 'applications'">
        <IsolatedVbenGrid
          :key="`applications-${gridMountVersion}`"
          :pagination="pagination.applications"
          :columns="applicationColumns"
          :rows="applicationPageRows"
          row-key="applicationNo"
          :locale="context.locale"
          :loading="pageLoading[activeTab]"
          :empty-text="tr('finance.credit.empty')"
          @failed="onGridFailed"
        />
      </template>

      <template v-else-if="activeTab === 'accounts'">
        <IsolatedVbenGrid
          :key="`accounts-${gridMountVersion}`"
          :pagination="pagination.accounts"
          :columns="accountColumns"
          :rows="accountPageRows"
          row-key="creditAccountNo"
          :locale="context.locale"
          :loading="pageLoading[activeTab]"
          :empty-text="tr('finance.credit.empty')"
          @failed="onGridFailed"
        />
      </template>

      <template v-else>
        <IsolatedVbenGrid
          :key="`statements-${gridMountVersion}`"
          :pagination="pagination.statements"
          :columns="statementColumns"
          :rows="statementPageRows"
          row-key="statementNo"
          :locale="context.locale"
          :loading="pageLoading[activeTab]"
          :empty-text="tr('finance.credit.empty')"
          @failed="onGridFailed"
        />
      </template>
    </section>

    <div
      v-if="detailOpen && selected"
      class="modal-backdrop"
      @click.self="detailOpen = false"
    >
      <section
        class="native-modal"
        role="dialog"
        aria-modal="true"
        data-testid="credit-application-detail-drawer"
      >
        <div class="modal-heading">
          <h2>{{ tr("finance.credit.detailTitle") }}</h2>
          <button
            class="icon-button"
            type="button"
            aria-label="Close"
            @click="detailOpen = false"
          >
            ×
          </button>
        </div>
        <dl class="fact-grid">
          <div>
            <dt>{{ tr("finance.credit.application") }}</dt>
            <dd>{{ selected.applicationNo }}</dd>
          </div>
          <div>
            <dt>{{ tr("finance.credit.customer") }}</dt>
            <dd>{{ selected.customerNo }}</dd>
          </div>
          <div>
            <dt>{{ tr("finance.credit.amount") }}</dt>
            <dd>
              {{ money(selected.requestedLimitMinor, selected.currencyCode) }}
            </dd>
          </div>
          <div>
            <dt>{{ tr("finance.credit.progress") }}</dt>
            <dd>
              {{ selected.reviewCount }}/{{ selected.requiredApprovalCount }}
            </dd>
          </div>
        </dl>
        <h3>{{ tr("finance.credit.evidence") }}</h3>
        <div class="evidence-list">
          <button
            v-for="item in evidence"
            :key="item.uploadNo"
            class="button secondary"
            data-testid="credit-evidence-open"
            type="button"
            @click="openEvidence(item)"
          >
            {{ item.fileName }} · {{ Math.ceil(item.sizeBytes / 1024) }} KiB
          </button>
        </div>
      </section>
    </div>

    <div
      v-if="reviewOpen && selected"
      class="modal-backdrop"
      @click.self="reviewOpen = false"
    >
      <section
        class="native-modal compact-modal"
        role="dialog"
        aria-modal="true"
        data-testid="credit-review-modal"
      >
        <div class="modal-heading">
          <h2>{{ tr("finance.credit.reviewTitle") }}</h2>
          <button
            class="icon-button"
            type="button"
            aria-label="Close"
            @click="reviewOpen = false"
          >
            ×
          </button>
        </div>
        <div class="decision-options">
          <label
            ><input
              v-model="reviewForm.decisionCode"
              type="radio"
              value="approved"
            />{{ tr("finance.credit.approve") }}</label
          ><label
            ><input
              v-model="reviewForm.decisionCode"
              type="radio"
              value="rejected"
            />{{ tr("finance.credit.reject") }}</label
          >
        </div>
        <label class="field"
          ><span>{{ tr("finance.credit.comment") }}</span
          ><textarea
            v-model="reviewForm.comment"
            :placeholder="tr('finance.credit.comment')"
            rows="4"
          />
        </label>
        <label class="field"
          ><span>{{ tr("finance.credit.evidenceBasis") }}</span
          ><textarea
            v-model="reviewForm.evidence"
            :placeholder="tr('finance.credit.evidenceBasis')"
            rows="4"
          />
        </label>
        <div class="modal-actions">
          <button
            class="button primary"
            :disabled="reviewLoading"
            type="button"
            @click="submitReview"
          >
            {{ tr("finance.credit.submit") }}
          </button>
        </div>
      </section>
    </div>
  </component>
</template>
