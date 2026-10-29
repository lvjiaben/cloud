import { initialLoad, forgetInitialLoad } from "#/runtime/initial-load";
import Dialog from "#/adapter/Dialog";
import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  Alert,
  Button,
  Card,
  Empty,
  Loading,
  Table,
  Tag,
  type PrimaryTableCol,
  type TableRowData,
} from "tdesign-react";
import { Link, useLocation } from "react-router-dom";
import { FileIcon } from "tdesign-icons-react";
export function money(value: string | undefined, currency = "CNY") {
  if (value === undefined || !/^[-]?\d+$/.test(value)) return "—";
  const minor = BigInt(value),
    negative = minor < 0n,
    absolute = negative ? -minor : minor;
  const digits =
    new Intl.NumberFormat("en", {
      style: "currency",
      currency,
    }).resolvedOptions().maximumFractionDigits ?? 2;
  const unit = 10n ** BigInt(digits);
  return `${currency} ${negative ? "-" : ""}${(absolute / unit).toLocaleString("zh-CN")}${digits ? "." + (absolute % unit).toString().padStart(digits, "0") : ""}`;
}
export function minorAmount(value: string, currency = "CNY") {
  const digits =
    new Intl.NumberFormat("en", {
      style: "currency",
      currency,
    }).resolvedOptions().maximumFractionDigits ?? 2;
  if (
    !new RegExp(`^\\d+(?:\\.\\d{1,${Math.max(digits, 1)}})?$`).test(value) ||
    (digits === 0 && value.includes("."))
  )
    throw new Error("请输入正确金额");
  const [integer, fraction = ""] = value.split(".");
  const result =
    BigInt(integer) * 10n ** BigInt(digits) +
    BigInt(fraction.padEnd(digits, "0") || "0");
  if (result <= 0n) throw new Error("金额必须大于零");
  return result.toString();
}
export function date(value?: number | null) {
  return value
    ? new Date(value).toLocaleString("zh-CN", { hour12: false })
    : "—";
}
const states: Record<string, string> = {
  waiting_staff: "等待客服回复",
  submitting: "正在提交",
  accepted: "已受理",
  retry_wait: "等待处理",
  manual_takeover: "人工处理中",
  draft: "待提交",
  verifying: "核验中",
  awaiting_balance_consent: "待确认退至余额",
  partially_refunded: "部分退款",
  overdue: "已逾期",
  submitted: "已提交",
  pending_signature: "待签署",
  signed: "已签署",
  withdrawn: "已撤回",
  active: "使用中",
  running: "运行中",
  stopped: "已关机",
  suspended: "已暂停",
  expired: "已到期",
  terminated: "已结束",
  pending: "待处理",
  pending_activation: "待开通",
  pending_payment: "待付款",
  awaiting_payment: "待付款",
  unpaid: "待付款",
  open: "待付款",
  partially_paid: "部分付款",
  paid: "已付款",
  settled: "已结清",
  completed: "已完成",
  succeeded: "已成功",
  success: "已成功",
  cancelled: "已取消",
  canceled: "已取消",
  failed: "未完成",
  processing: "处理中",
  queued: "等待处理",
  new: "待回复",
  in_progress: "处理中",
  waiting_customer: "待您回复",
  resolved: "已解决",
  closed: "已关闭",
  pending_review: "待审核",
  approved: "已通过",
  rejected: "未通过",
  preparing: "准备中",
  ready: "已就绪",
  needs_attention: "待人工处理",
  manual_processing: "人工处理中",
  not_completed: "未完成",
  delivered: "已送达",
  issued: "已开具",
  verified: "已认证",
  unverified: "未认证",
  individual: "个人",
  enterprise: "企业",
  customer_balance: "现金余额",
  gift_balance: "赠送余额",
  credit: "信用额度",
  credit_payment: "信用付款",
  recharge: "充值",
  payment: "付款",
  refund: "退款",
  renewal: "续费",
  purchase: "购买",
  enabled: "已开启",
  disabled: "已关闭",
  revoked: "已停用",
  refunded: "已退款",
  awaiting_review: "待审核",
  result_unknown: "结果确认中",
};
export function stateLabel(value: string) {
  return states[value] || "状态待确认";
}
export function Status({ value }: { value: string }) {
  return (
    <Tag
      variant="light"
      theme={
        [
          "active",
          "running",
          "paid",
          "completed",
          "succeeded",
          "settled",
          "verified",
        ].includes(value)
          ? "success"
          : ["failed", "rejected", "expired"].includes(value)
            ? "danger"
            : "default"
      }
    >
      {stateLabel(value)}
    </Tag>
  );
}
export function useLoad<T>(
  load: () => Promise<T>,
  deps: unknown[] = [],
  pageKey?: string,
) {
  const route = useLocation();
  const [initial] = useState(() => {
    if (!pageKey) return undefined;
    const key = JSON.stringify([
      route.key,
      route.pathname,
      route.search,
      pageKey,
      deps,
    ]);
    return { key, entry: initialLoad(key, load, route.key) };
  });
  const [data, setData] = useState<T | undefined>(initial?.entry.data);
  const [error, setError] = useState(initial?.entry.error || "");
  const [loading, setLoading] = useState(!initial?.entry.settled);
  const committed = useRef(false);
  const [revision, setRevision] = useState(0);
  const previousDeps = useRef(deps);
  const current = useRef(load);
  current.current = load;
  useEffect(() => {
    let active = true;
    if (!committed.current && initial) {
      committed.current = true;
      void initial.entry.promise.then(() => {
        if (active) {
          setData(initial.entry.data);
          setError(initial.entry.error);
          setLoading(false);
          forgetInitialLoad(initial.key, initial.entry);
        }
      });
      return () => {
        active = false;
      };
    }
    committed.current = true;
    setLoading(true);
    setError("");
    if (
      deps.some(
        (value, index) => !Object.is(value, previousDeps.current[index]),
      )
    )
      setData(undefined);
    previousDeps.current = deps;
    current
      .current()
      .then((value) => {
        if (active) setData(value);
      })
      .catch((e) => {
        if (active)
          setError(e instanceof Error ? e.message : "加载失败，请重试");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [...deps, revision]);
  return {
    data,
    error,
    loading,
    initialPromise:
      !committed.current && loading ? initial?.entry.promise : undefined,
    reload: () => setRevision((v) => v + 1),
  };
}
export function FetchState({
  state,
  children,
}: {
  state: {
    loading: boolean;
    error: string;
    reload: () => void;
    initialPromise?: Promise<void>;
  };
  children: ReactNode;
}) {
  const hasData = "data" in state && state.data !== undefined;
  if (state.initialPromise && !hasData) throw state.initialPromise;
  if (state.loading && !hasData)
    return (
      <div className="content-skeleton" role="status" aria-label="正在加载">
        <span />
        <span />
        <span />
      </div>
    );
  if (state.error && !hasData)
    return (
      <Alert
        theme="error"
        title={state.error}
        operation={
          <Button tag="button" variant="text" onClick={state.reload}>
            重新加载
          </Button>
        }
      />
    );
  return (
    <div className="content-state" aria-busy={state.loading}>
      {state.error && (
        <Alert
          theme="warning"
          title={state.error}
          operation={
            <Button tag="button" variant="text" onClick={state.reload}>
              重新加载
            </Button>
          }
        />
      )}
      {state.loading && (
        <span className="content-refresh" role="status">
          正在更新
        </span>
      )}
      <div {...(state.loading || state.error ? { inert: "" } : {})}>
        {children}
      </div>
    </div>
  );
}
export function useAction() {
  const lock = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return {
    busy,
    error,
    clear: () => setError(""),
    run: async (action: () => Promise<void>) => {
      if (lock.current) return;
      lock.current = true;
      setBusy(true);
      setError("");
      try {
        await action();
      } catch (e) {
        setError(e instanceof Error ? e.message : "操作未完成，请重试");
      } finally {
        lock.current = false;
        setBusy(false);
      }
    },
  };
}
export function Page({
  title,
  description,
  actions,
  children,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const route = useLocation();
  useEffect(() => {
    window.dispatchEvent(new Event("customer-page-ready"));
  }, [route.key]);
  return (
    <section className="page">
      <header className="page-heading">
        <div>
          <h1>{title}</h1>
          {description && <p>{description}</p>}
        </div>
        <div className="actions">{actions}</div>
      </header>
      {children}
    </section>
  );
}
export function ErrorNote({ error }: { error: string }) {
  return error ? (
    <Alert theme="error" title={error} className="error-note" />
  ) : null;
}
export function BusinessTable<T extends TableRowData>({
  rows,
  columns,
  rowKey,
  total,
  page = 1,
  size = 20,
  onPage,
}: {
  rows: T[];
  columns: PrimaryTableCol<T>[];
  rowKey: string;
  total?: number;
  page?: number;
  size?: number;
  onPage?: (page: number, size: number) => void;
}) {
  return (
    <Table<T>
      rowKey={rowKey}
      data={rows}
      columns={columns}
      stripe={false}
      hover
      maxHeight="calc(100vh - 330px)"
      tableLayout="auto"
      empty={
        <div className="table-empty">
          <FileIcon size={28} />
          <span>暂无记录</span>
        </div>
      }
      pagination={
        total !== undefined
          ? {
              total,
              current: page,
              pageSize: size,
              pageSizeOptions: [10, 20, 50],
              showJumper: true,
              onChange: (p) => onPage?.(p.current, p.pageSize),
            }
          : undefined
      }
    />
  );
}
export function CursorPager({
  next,
  history,
  onNext,
  onPrevious,
}: {
  next?: string;
  history: string[];
  onNext: () => void;
  onPrevious: () => void;
}) {
  return (
    <div className="pager">
      <span>第 {history.length + 1} 页</span>
      <Button
        tag="button"
        variant="outline"
        disabled={!history.length}
        onClick={onPrevious}
      >
        上一页
      </Button>
      <Button tag="button" variant="outline" disabled={!next} onClick={onNext}>
        下一页
      </Button>
    </div>
  );
}
export function useCursor(filter = "") {
  const [state, setState] = useState({
    filter,
    cursor: "",
    history: [] as string[],
  });
  const current =
    state.filter === filter
      ? state
      : { filter, cursor: "", history: [] as string[] };
  return {
    cursor: current.cursor,
    history: current.history,
    next: (value: string) =>
      setState({
        filter,
        cursor: value,
        history: [...current.history, current.cursor],
      }),
    previous: () =>
      setState({
        filter,
        cursor: current.history.at(-1) || "",
        history: current.history.slice(0, -1),
      }),
  };
}
export function Confirm({
  title,
  children,
  onConfirm,
  onClose,
  busy,
  error,
}: {
  title: string;
  children: ReactNode;
  onConfirm: () => void;
  onClose: () => void;
  busy: boolean;
  error: string;
}) {
  return (
    <Dialog
      visible
      header={title}
      confirmBtn={{ content: "确认", loading: busy }}
      cancelBtn={{ content: "取消", disabled: busy }}
      onConfirm={onConfirm}
      onClose={() => !busy && onClose()}
      closeOnOverlayClick={false}
    >
      {children}
      <ErrorNote error={error} />
    </Dialog>
  );
}
export function DetailList({ items }: { items: [string, ReactNode][] }) {
  return (
    <dl className="details">
      {items.map(([label, value]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  );
}
export function EmptyPage() {
  return (
    <Page title="页面不存在">
      <Card>
        <Empty description="这个地址目前没有可用页面" />
        <Link to="/account">返回账户概览</Link>
      </Card>
    </Page>
  );
}
