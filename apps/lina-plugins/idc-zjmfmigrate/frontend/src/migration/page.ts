import {
  mountIsolatedVbenTable,
  mountIsolatedVbenFormDrawer,
  type IsolatedTableHandle,
} from "@linapro/vben-plugin-adapter/isolated";
import zh from "../../../manifest/i18n/zh-CN/plugin.json";
import en from "../../../manifest/i18n/en-US/plugin.json";
import { createBridge } from "../shared/bridge";
import { createNumberedPage } from "../../pages/numbered-page";
import type {
  MigrationBatch,
  MigrationConflict,
  MigrationPreflightInput,
  MigrationPreflightResult,
  MigrationReport,
} from "./model";

export function startMigrationPage(
  win: Window = window,
  doc: Document = document,
) {
  const bridge = createBridge(
    win,
    "migration-checks",
    "idc-migration.admin.v1",
  );
  let locale = "zh-CN",
    busy = false,
    preflight: MigrationPreflightResult | undefined,
    source: MigrationPreflightInput | undefined,
    batch: MigrationBatch | undefined;
  let table:
    | IsolatedTableHandle<MigrationConflict & Record<string, unknown>>
    | undefined;
  let page = {
    currentPage: 1,
    pageSize: 10,
    total: 0,
    items: [] as MigrationConflict[],
  };
  const t = (key: string) => {
    const words = locale === "en-US" ? en : zh;
    return (
      (words as Record<string, string>)[
        `plugin.idc-zjmfmigrate.migration.${key}`
      ] ?? key
    );
  };
  const words = (cn: string, english: string) =>
    locale === "en-US" ? english : cn;
  const node = (id: string) =>
    doc.querySelector<HTMLElement>(`[data-testid="${id}"]`)!;
  const resize = () =>
    bridge.resize(Math.ceil(doc.documentElement.scrollHeight));
  const setNotice = (value: string) => {
    node("migration-notice").textContent = value;
    resize();
  };
  const request = async <T>(
    method: string,
    path: string,
    data?: Record<string, unknown>,
  ) => {
    const response = await bridge.request(
      method,
      `admin/migration/${path}`,
      method === "GET" ? { params: data ?? {} } : { body: data ?? {} },
    );
    if (response.ok !== true)
      throw new Error(
        words(
          "操作未完成，请检查权限或稍后重试。",
          "The operation could not be completed. Check access or try again.",
        ),
      );
    return response.result as T;
  };
  const run = async (action: () => Promise<void>) => {
    if (busy) return;
    busy = true;
    renderActions();
    try {
      await action();
    } catch (error) {
      setNotice(
        error instanceof Error
          ? error.message
          : words("操作失败，请重试。", "Operation failed. Try again."),
      );
    } finally {
      busy = false;
      renderActions();
      resize();
    }
  };
  const pager = createNumberedPage<MigrationConflict>(
    (state) => {
      page = state;
      renderTable();
    },
    () => {
      node("migration-report").replaceChildren();
    },
  );
  const paragraph = (text: string) => {
    const p = doc.createElement("p");
    p.textContent = text;
    return p;
  };
  function renderResult(result: MigrationPreflightResult) {
    node("migration-result").replaceChildren(
      paragraph(
        t(
          result.supported
            ? "messages.preflightPassed"
            : "messages.preflightBlocked",
        ),
      ),
    );
    for (const [label, items] of [
      ["labels.blockers", result.blockers],
      ["labels.warnings", result.warnings],
      ["labels.capabilities", result.capabilities],
    ] as const) {
      const block = doc.createElement("section"),
        title = doc.createElement("strong");
      title.textContent = t(label);
      block.append(title);
      for (const value of items ?? [])
        block.append(
          paragraph(
            t(
              label === "labels.capabilities"
                ? `capabilities.${value === "billing_invoice" ? "billingInvoice" : value}`
                : `messages.${messageKey(value)}`,
            ),
          ),
        );
      node("migration-result").append(block);
    }
  }
  function messageKey(value: string) {
    return (
      (
        {
          "charset needs explicit conversion review": "charsetReview",
          "core mappings complete": "coreMappingsComplete",
          "no external side effects": "noExternalSideEffects",
          "source credentials are task-scoped and never persisted":
            "credentialsNotPersisted",
          "source freeze confirmed": "sourceFreezeConfirmed",
          "source system is not in the verified adapter matrix":
            "unsupportedSource",
          "source version is required": "sourceVersionRequired",
          "timezone is missing; timestamps require manual review":
            "timezoneReview",
          "unresolved conflicts or reconciliation differences":
            "unresolvedDifferences",
        } as Record<string, string>
      )[value] ?? value
    );
  }
  function renderTable() {
    const options = {
      locale,
      rows: page.items as (MigrationConflict & Record<string, unknown>)[],
      rowKey: "id",
      columns: [
        { field: "objectType", title: t("columns.objectType") },
        { field: "sourceId", title: t("columns.sourceId") },
        { field: "summary", title: t("columns.summary"), minWidth: 240 },
        { field: "decision", title: t("columns.decision") },
        {
          field: "actions",
          title: t("columns.actions"),
          fixed: "right" as const,
          width: 140,
          cell: (row: MigrationConflict) => {
            const button = doc.createElement("button");
            button.textContent = t("actions.resolveConflict");
            button.disabled = row.decision !== "pending" || busy;
            button.onclick = () => decide(row);
            return button;
          },
        },
      ],
      pagination: {
        currentPage: page.currentPage,
        pageSize: page.pageSize,
        total: page.total,
        onChange: ({
          currentPage,
          pageSize,
        }: {
          currentPage: number;
          pageSize: number;
        }) => {
          void run(() => loadConflicts(currentPage, pageSize));
        },
      },
    };
    if (table) table.update(options);
    else {
      table = mountIsolatedVbenTable(node("migration-conflicts"), options);
      void table.ready
        .then(resize)
        .catch(() =>
          setNotice(
            words(
              "表格加载失败，请刷新页面。",
              "The table could not load. Refresh the page.",
            ),
          ),
        );
    }
  }
  async function loadConflicts(
    currentPage = page.currentPage,
    pageSize = page.pageSize,
  ) {
    if (!batch) return;
    const id = batch.id;
    await pager.query({ currentPage, pageSize }, async (p, size) => {
      const result = await request<{
        list: MigrationConflict[];
        total: number;
      }>("GET", `batches/${id}/conflicts`, { page: p, pageSize: size });
      return { items: result.list, total: result.total };
    });
  }
  async function loadEvidence() {
    if (!batch) return;
    const id = batch.id;
    const [report] = await Promise.all([
      request<MigrationReport>("GET", `batches/${id}/report`),
      loadConflicts(),
    ]);
    if (batch?.id !== id) return;
    node("migration-report").replaceChildren(
      paragraph(t(`status.${report.status}`)),
    );
    for (const [key, value] of Object.entries(report.byType ?? {}))
      node("migration-report").append(
        paragraph(
          `${t(`capabilities.${key === "billing_invoice" ? "billingInvoice" : key}`)}：${t(`evidence.${value}`)}`,
        ),
      );
    for (const value of [
      ...(report.blockers ?? []),
      ...(report.warnings ?? []),
    ])
      node("migration-report").append(
        paragraph(t(`messages.${messageKey(value)}`)),
      );
  }
  function sourceDrawer() {
    const drawer = mountIsolatedVbenFormDrawer(node("migration-drawer"), {
      title: t("sections.source"),
      locale,
      values: {
        sourceSystem: "zjmf-finance",
        sourceVersion: "",
        charset: "UTF-8",
        timezone: "Asia/Shanghai",
        currencies: "CNY",
        ...source,
      },
      fields: [
        {
          name: "sourceSystem",
          label: t("fields.sourceSystem"),
          component: "Select",
          required: true,
          props: {
            options: [
              { label: t("sources.zjmf"), value: "zjmf-finance" },
              { label: t("sources.v10"), value: "v10" },
            ],
          },
        },
        ...["sourceVersion", "charset", "timezone", "currencies"].map(
          (name) => ({
            name,
            label: t(`fields.${name}`),
            component: "Input" as const,
            required: true,
          }),
        ),
      ],
      onSubmit: async (values) => {
        try {
          const candidate = {
            sourceSystem: String(values.sourceSystem),
            sourceVersion: String(values.sourceVersion).trim(),
            charset: String(values.charset),
            timezone: String(values.timezone),
            currencies: String(values.currencies)
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean),
          };
          preflight = await request("POST", "preflight", candidate);
          source = candidate;
          renderResult(preflight!);
          renderActions();
          return true;
        } catch {
          drawer.update({
            errorText: words(
              "检查失败，请检查权限或重试。",
              "Preflight failed. Check access or retry.",
            ),
          });
          return false;
        }
      },
      onClosed: () => drawer.destroy(),
    });
    void drawer.ready.catch(() =>
      setNotice(
        words(
          "表单加载失败，请刷新页面。",
          "The form could not load. Refresh the page.",
        ),
      ),
    );
  }
  function batchDrawer() {
    if (!preflight?.supported || !source) return;
    const frozen = { ...source };
    const drawer = mountIsolatedVbenFormDrawer(node("migration-drawer"), {
      title: t("sections.batch"),
      locale,
      values: { batchNo: `MIG-${Date.now()}`, mode: "dry_run" },
      fields: [
        {
          name: "batchNo",
          label: t("fields.batchNo"),
          component: "Input",
          required: true,
        },
        {
          name: "mode",
          label: t("fields.mode"),
          component: "Select",
          required: true,
          props: {
            options: [
              { label: t("modes.dryRun"), value: "dry_run" },
              { label: t("modes.import"), value: "import" },
            ],
          },
        },
      ],
      onSubmit: async (values) => {
        if (
          values.mode === "import" &&
          !win.confirm(
            words(
              "正式导入可能写入目标系统。确认已备份并获准执行当前迁移批次？",
              "Import can write to the target system. Confirm backups and authorization for this batch?",
            ),
          )
        )
          return false;
        try {
          batch = await request("POST", "batches", {
            batchNo: String(values.batchNo).trim(),
            mode: values.mode,
            sourceSystem: frozen.sourceSystem,
            sourceVersion: frozen.sourceVersion,
          });
          pager.reset();
          node("migration-report").replaceChildren();
          renderActions();
          setNotice(t("messages.batchCreated"));
          return true;
        } catch {
          drawer.update({
            errorText: words(
              "批次创建失败，请检查权限或重试。",
              "Batch creation failed. Check access or retry.",
            ),
          });
          return false;
        }
      },
      onClosed: () => drawer.destroy(),
    });
    void drawer.ready.catch(() =>
      setNotice(words("表单加载失败。", "The form could not load.")),
    );
  }
  function decide(row: MigrationConflict) {
    const drawer = mountIsolatedVbenFormDrawer(node("migration-drawer"), {
      title: t("actions.resolveConflict"),
      locale,
      description: row.summary,
      values: { decision: "skip", reason: "" },
      fields: [
        {
          name: "decision",
          label: t("columns.decision"),
          component: "Select",
          required: true,
          props: {
            options: [
              { label: words("保留来源", "Keep source"), value: "keep_source" },
              { label: words("合并", "Merge"), value: "merge" },
              { label: words("跳过", "Skip"), value: "skip" },
            ],
          },
        },
        {
          name: "reason",
          label: words("处理原因", "Reason"),
          component: "Textarea",
          required: true,
          props: { minlength: 2, maxlength: 500 },
        },
      ],
      onSubmit: async (values) => {
        if (String(values.reason).trim().length < 2) return false;
        try {
          await request("PUT", `conflicts/${row.id}/decision`, {
            decision: values.decision,
            reason: String(values.reason).trim(),
          });
          await loadEvidence();
          return true;
        } catch {
          drawer.update({
            errorText: words(
              "冲突处理失败，请检查权限或重试。",
              "Decision failed. Check access or retry.",
            ),
          });
          return false;
        }
      },
      onClosed: () => drawer.destroy(),
    });
    void drawer.ready.catch(() =>
      setNotice(words("表单加载失败。", "The form could not load.")),
    );
  }
  async function execute(action: string) {
    if (!batch) return;
    if (
      ["run", "rollback", "cutover"].includes(action) &&
      !win.confirm(
        words(
          "确认执行当前批次的此项操作？",
          "Confirm this operation for the current batch?",
        ),
      )
    )
      return;
    const id = batch.id;
    const data =
      action === "run"
        ? { watermark: (node("migration-watermark") as HTMLInputElement).value }
        : {};
    const result = await request<{
      status?: string;
      allowed?: boolean;
      checks?: string[];
      mapped?: number;
      differences?: number;
    }>("POST", `batches/${id}/${action}`, data);
    if (result.status) batch.status = result.status;
    if (action === "cutover" && result.allowed) batch.status = "cutover";
    setNotice(
      result.checks?.map((v) => t(`messages.${messageKey(v)}`)).join("；") ??
        t(
          `messages.${action === "run" ? "runCompleted" : action === "reconcile" ? "reconcileCompleted" : action === "rollback" ? "rollbackCompleted" : "cutoverBlocked"}`,
        ).replace("{count}", String(result.mapped ?? result.differences ?? 0)),
    );
    await loadEvidence();
  }
  function renderActions() {
    for (const button of doc.querySelectorAll<HTMLButtonElement>(
      "[data-action]",
    )) {
      const action = button.dataset.action!;
      button.disabled =
        busy ||
        (action === "createBatch"
          ? !preflight?.supported
          : action === "preflight"
            ? false
            : !batch || (action === "rollback" && batch.status === "cutover"));
    }
    node("migration-batch").textContent = batch
      ? `${batch.batchNo} · ${t(`status.${batch.status}`)}`
      : words("尚未创建批次", "No batch selected");
  }
  function render() {
    doc.title = t("title");
    bridge.setTitle(doc.title);
    doc.documentElement.lang = locale;
    node("migration-title").textContent = t("title");
    node("migration-boundary").textContent =
      `${t("boundaryTitle")}。${t("boundaryDescription")}`;
    for (const button of doc.querySelectorAll<HTMLButtonElement>(
      "[data-action]",
    ))
      button.textContent =
        button.dataset.action === "refresh"
          ? words("刷新证据", "Refresh evidence")
          : t(`actions.${button.dataset.action}`);
    for (const label of doc.querySelectorAll<HTMLElement>("[data-label]"))
      label.textContent = t(label.dataset.label!);
    renderActions();
    if (table) renderTable();
    if (preflight) renderResult(preflight);
    resize();
  }
  doc.body.innerHTML =
    '<main class="migration"><header><h1 data-testid="migration-title"></h1><div><button data-action="preflight"></button><button data-action="createBatch"></button></div></header><p class="notice" data-testid="migration-boundary"></p><p role="status" data-testid="migration-notice"></p><section class="panel"><h2 data-label="sections.result"></h2><div data-testid="migration-result"></div></section><section class="panel"><h2 data-label="tabs.execution"></h2><p data-testid="migration-batch"></p><input data-testid="migration-watermark" aria-label="Watermark" maxlength="128"><div class="actions"><button data-action="run"></button><button data-action="reconcile"></button><button data-action="cutover"></button><button data-action="rollback"></button><button data-action="refresh"></button></div></section><section class="panel"><h2 data-label="tabs.conflicts"></h2><div data-testid="migration-conflicts"></div></section><section class="panel"><h2 data-label="tabs.report"></h2><div data-testid="migration-report"></div></section><div data-testid="migration-drawer"></div></main>';
  for (const button of doc.querySelectorAll<HTMLButtonElement>("[data-action]"))
    button.onclick = () => {
      const action = button.dataset.action!;
      if (action === "preflight") sourceDrawer();
      else if (action === "createBatch") batchDrawer();
      else
        void run(() =>
          action === "refresh" ? loadEvidence() : execute(action),
        );
    };
  const off = bridge.subscribeContext((context) => {
    locale = context.locale === "en-US" ? "en-US" : "zh-CN";
    const theme = context.theme as Record<string, unknown> | undefined;
    doc.documentElement.dataset.theme = String(theme?.mode ?? "light");
    for (const [key, value] of Object.entries(theme?.cssVariables ?? {}))
      if (typeof value === "string")
        doc.documentElement.style.setProperty(key, value);
    render();
  });
  render();
  bridge.ready();
  const observer = new ResizeObserver(resize);
  observer.observe(doc.body);
  return () => {
    off();
    pager.invalidate();
    table?.destroy();
    observer.disconnect();
    bridge.dispose();
  };
}
