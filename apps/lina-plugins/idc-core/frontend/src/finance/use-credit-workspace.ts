import { computed, onMounted, reactive, ref } from "vue";

import type { PluginHostMountContext } from "../host-context";
import type {
  CreditAccount,
  CreditApplication,
  CreditEvidence,
  CreditStatement,
} from "./api";

import { createTranslator } from "../runtime/i18n";
import { PluginRequestError } from "../runtime/request";
import { createFinanceAPI } from "./api";
import { financeMessages } from "./messages";

export function useCreditWorkspace(props: { context: PluginHostMountContext }) {
  const currentContext = () => props.context;
  const api = createFinanceAPI(currentContext);
  const tr = createTranslator(currentContext, financeMessages);

  const activeTab = ref<"accounts" | "applications" | "statements">(
    "applications",
  );
  const applications = ref<CreditApplication[]>([]);
  const accounts = ref<CreditAccount[]>([]);
  const statements = ref<CreditStatement[]>([]);
  const selected = ref<CreditApplication | null>(null);
  const evidence = ref<CreditEvidence[]>([]);
  const detailOpen = ref(false);
  const reviewOpen = ref(false);
  type Directory = "accounts" | "applications" | "statements";
  const pageState = reactive({
    accounts: { currentPage: 1, pageSize: 10, total: 0 },
    applications: { currentPage: 1, pageSize: 10, total: 0 },
    statements: { currentPage: 1, pageSize: 10, total: 0 },
  });
  const requestedPages = {
    accounts: { currentPage: 1, pageSize: 10 },
    applications: { currentPage: 1, pageSize: 10 },
    statements: { currentPage: 1, pageSize: 10 },
  };
  const generations = { accounts: 0, applications: 0, statements: 0 };
  let detailGeneration = 0;
  const pageLoading = reactive({ accounts: false, applications: false, statements: false });
  const pageFailed = { accounts: false, applications: false, statements: false };
  const loading = computed(() => Object.values(pageLoading).some(Boolean));
  const reviewLoading = ref(false);
  const error = ref("");
  const notice = ref("");
  const reviewForm = reactive({
    comment: "",
    decisionCode: "approved" as "approved" | "rejected",
    evidence: "",
  });

  const accountRows = computed(() =>
    accounts.value.map((item) => ({
      ...item,
      usedMinor: (
        BigInt(item.unbilledPrincipalMinor) + BigInt(item.billedPrincipalMinor)
      ).toString(),
    })),
  );

  const applicationPageRows = computed(() => applications.value);
  const accountPageRows = computed(() => accountRows.value);
  const statementPageRows = computed(() => statements.value);

  function money(value: string, currencyCode: string) {
    const minor = value.padStart(3, "0");
    return `${currencyCode} ${minor.slice(0, -2)}.${minor.slice(-2)}`;
  }

  function status(code: string) {
    const value = tr(`finance.status.${code}`);
    return value === `finance.status.${code}`
      ? tr("finance.status.unknown")
      : value;
  }

  function formatDate(value: number) {
    return new Intl.DateTimeFormat(props.context.locale, {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    }).format(new Date(value));
  }

  function clearApplicationDetails() {
    detailGeneration += 1;
    selected.value = null;
    evidence.value = [];
    detailOpen.value = false;
    reviewOpen.value = false;
    reviewForm.comment = "";
    reviewForm.evidence = "";
    reviewForm.decisionCode = "approved";
    reviewLoading.value = false;
    notice.value = "";
  }

  function clearDirectory(kind: Directory) {
    generations[kind] += 1;
    pageLoading[kind] = false;
    pageState[kind] = { ...pageState[kind], currentPage: 1, total: 0 };
    requestedPages[kind] = { currentPage: 1, pageSize: pageState[kind].pageSize };
    if (kind === "applications") {
      applications.value = [];
      clearApplicationDetails();
    } else if (kind === "accounts") accounts.value = [];
    else statements.value = [];
  }

  function handleAccessFailure(failure: unknown, kind?: Directory) {
    if (!(failure instanceof PluginRequestError)) return;
    if (failure.status === 401) {
      for (const directory of Object.keys(requestedPages) as Directory[]) clearDirectory(directory);
    } else if (failure.status === 403) {
      if (kind) clearDirectory(kind);
      else clearApplicationDetails();
    }
  }

  async function loadPage(kind: Directory, currentPage: number, pageSize: number) {
    if (pageSize !== pageState[kind].pageSize) currentPage = 1;
    const generation = ++generations[kind];
    requestedPages[kind] = { currentPage, pageSize };
    pageLoading[kind] = true;
    pageFailed[kind] = false;
    if (!Object.values(pageFailed).some(Boolean)) error.value = "";
    try {
      let total: number;
      if (kind === "applications") {
        const result = await api.applications(currentPage, pageSize);
        if (generation !== generations[kind]) return;
        applications.value = result.applications ?? [];
        total = result.total;
      } else if (kind === "accounts") {
        const result = await api.accounts(currentPage, pageSize);
        if (generation !== generations[kind]) return;
        accounts.value = result.accounts ?? [];
        total = result.total;
      } else {
        const result = await api.statements(currentPage, pageSize);
        if (generation !== generations[kind]) return;
        statements.value = result.statements ?? [];
        total = result.total;
      }
      pageState[kind] = { currentPage, pageSize, total };
    } catch (failure) {
      if (generation !== generations[kind]) return;
      handleAccessFailure(failure, kind);
      pageFailed[kind] = true;
      error.value = tr("finance.credit.failed");
      // VXE updates its pager optimistically; restore the last confirmed page.
      pageState[kind] = { ...pageState[kind] };
    } finally {
      if (generation === generations[kind]) pageLoading[kind] = false;
    }
  }

  async function load() {
    await Promise.all((Object.keys(requestedPages) as Directory[]).map((kind) => {
      const { currentPage, pageSize } = requestedPages[kind];
      return loadPage(kind, currentPage, pageSize);
    }));
  }

  async function openDetail(item: CreditApplication) {
    const generation = ++detailGeneration;
    try {
      const result = await api.application(item.applicationNo);
      if (generation !== detailGeneration) return;
      selected.value = result.detail.application;
      evidence.value = result.evidence ?? [];
      detailOpen.value = true;
    } catch (failure) {
      if (generation !== detailGeneration) return;
      handleAccessFailure(failure);
      error.value = tr("finance.credit.failed");
    }
  }

  function openReview(item: CreditApplication) {
    detailGeneration += 1;
    selected.value = item;
    reviewForm.comment = "";
    reviewForm.decisionCode = "approved";
    reviewForm.evidence = "";
    reviewOpen.value = true;
  }

  async function digest(value: string) {
    const result = await crypto.subtle.digest(
      "SHA-256",
      new TextEncoder().encode(value.trim()),
    );
    return [...new Uint8Array(result)]
      .map((item) => item.toString(16).padStart(2, "0"))
      .join("");
  }

  async function submitReview() {
    if (
      !selected.value
    ) {
      notice.value = tr("finance.credit.invalid");
      return;
    }
    reviewLoading.value = true;
    const generation = detailGeneration;
    notice.value = "";
    try {
      await api.review(selected.value.applicationNo, {
        commentDigest: await digest(reviewForm.comment),
        decisionCode: reviewForm.decisionCode,
        evidenceDigest: await digest(reviewForm.evidence),
        requestNo: `credit-review-${crypto.randomUUID()}`,
      });
      if (generation !== detailGeneration) return;
      reviewOpen.value = false;
      notice.value = tr("finance.credit.reviewSuccess");
      await load();
    } catch (failure) {
      if (generation !== detailGeneration) return;
      handleAccessFailure(failure);
      notice.value = tr("finance.credit.reviewFailed");
    } finally {
      if (generation === detailGeneration) reviewLoading.value = false;
    }
  }

  async function openEvidence(item: CreditEvidence) {
    if (!selected.value) return;
    const generation = detailGeneration;
    try {
      const blob = await api.evidence(selected.value.applicationNo, item);
      if (generation !== detailGeneration) return;
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.target = "_blank";
      anchor.rel = "noopener noreferrer";
      anchor.click();
      globalThis.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch (failure) {
      if (generation !== detailGeneration) return;
      handleAccessFailure(failure);
      notice.value = tr("finance.credit.failed");
    }
  }

  onMounted(load);

  return {
    accountPageRows,
    accountRows,
    accounts,
    activeTab,
    applicationPageRows,
    applications,
    detailOpen,
    error,
    evidence,
    formatDate,
    load,
    loadPage,
    loading,
    money,
    notice,
    openDetail,
    openEvidence,
    openReview,
    pageLoading,
    pageState,
    reviewForm,
    reviewLoading,
    reviewOpen,
    selected,
    statementPageRows,
    statements,
    status,
    submitReview,
    tr,
  };
}
