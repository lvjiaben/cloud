import Dialog from "#/adapter/Dialog";
import { useFeature } from "#/runtime/features";
import { useEffect, useRef, useState } from "react";
import { Alert, Button, Card, Input, Select, Switch } from "tdesign-react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import QRCode from "qrcode";
import * as finance from "#/api/idc/customer-finance";
import {
  getCreditOverview,
  settleInvoiceFromCredit,
} from "#/api/idc/customer-credit";
import * as payments from "#/api/idc/customer-payments";
import {
  preparePayment,
  resumePaymentPreparation,
  type PaymentPreparation,
} from "#/api/idc/customer-commerce";
import { actionKey, externalURL } from "#/api/request";
import {
  BusinessTable,
  Confirm,
  CursorPager,
  DetailList,
  ErrorNote,
  FetchState,
  Page,
  Status,
  date,
  money,
  minorAmount,
  useAction,
  useCursor,
  useLoad,
} from "#/components/business";
import { RenewDialog } from "./Services";
const browserChannels = (channels: payments.CustomerPaymentChannel[]) =>
  channels
    .map((c) => ({
      ...c,
      modes: c.modes.filter((m) => !["jsapi", "app"].includes(m)),
    }))
    .filter((c) => c.modes.length);
const modes: Record<string, string> = {
  page: "网页支付",
  web: "网页支付",
  redirect: "网页支付",
  qr: "扫码支付",
  qrcode: "扫码支付",
  native: "扫码支付",
  h5: "手机支付",
  jsapi: "微信内支付",
  app: "应用内支付",
};
export function PaymentHandoff({
  payment,
}: {
  payment: payments.CustomerPaymentCreated;
}) {
  const [qr, setQr] = useState(""),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    if (payment.redirectMethod === "QR")
      QRCode.toDataURL(payment.redirectUrl, { width: 240, margin: 2 })
        .then((v) => {
          if (active) setQr(v);
        })
        .catch(() => {
          if (active) setError("二维码暂时无法显示，请稍后重试");
        });
    return () => {
      active = false;
    };
  }, [payment]);
  const href = externalURL(payment.redirectUrl);
  return (
    <div className="payment-handoff">
      <Alert theme="info" title="请完成支付后返回查看结果" />
      <ErrorNote error={error} />
      {qr && <img alt="付款二维码" src={qr} width={240} height={240} />}{" "}
      {payment.redirectMethod === "GET" && href && (
        <a
          className="t-button t-button--theme-primary t-button--variant-base"
          href={href}
          target="_blank"
          rel="noopener noreferrer"
        >
          打开支付页面
        </a>
      )}
      {payment.redirectMethod === "POST" && href && (
        <form
          method="POST"
          action={href}
          target="_blank"
          rel="noopener noreferrer"
        >
          {Object.entries(payment.redirectFields || {}).map(([name, value]) => (
            <input key={name} type="hidden" name={name} value={value} />
          ))}
          <Button tag="button" type="submit">
            打开支付页面
          </Button>
        </form>
      )}
      <p>付款有效期：{date(payment.expiresAt)}</p>
      <Link to={`/payment/return/${payment.attemptNo}`}>
        我已付款，查看结果
      </Link>
    </div>
  );
}
export function PayDialog({
  invoice,
  onClose,
  preference,
}: {
  invoice: finance.FinancialInvoice;
  onClose: () => void;
  preference?: { channelNo: string; useBalance: boolean };
}) {
  const paymentLocation = useLocation();
  const paymentNavigate = useNavigate();
  const credit = useLoad(getCreditOverview, []);
  const channels = useLoad(
      () => payments.listCustomerPaymentChannels(invoice.currencyCode),
      [],
    ),
    action = useAction();
  const [channel, setChannel] = useState(""),
    [mode, setMode] = useState(""),
    [created, setCreated] = useState<payments.CustomerPaymentCreated>(),
    [context, setContext] = useState<PaymentPreparation>(),
    [balanceConfirm, setBalanceConfirm] = useState(false),
    [creditConfirm, setCreditConfirm] = useState(false);
  const creditAccount = credit.data?.overview.accounts.find(
    (a) =>
      a.currencyCode === invoice.currencyCode &&
      a.statusCode === "active" &&
      a.validFrom <= Date.now() &&
      a.validUntil > Date.now() &&
      BigInt(a.availableLimitMinor) >= BigInt(invoice.outstandingAmountMinor),
  );
  const options = browserChannels(channels.data?.channels || []);
  const selected = options.find((v) => v.channelNo === channel);
  const appliedPreference = useRef(false);
  // Website parameters select a current option only. They never authorize a
  // payment, prepare a payment context, or bypass the existing confirmation.
  useEffect(() => {
    if (!channels.data || appliedPreference.current) return;
    appliedPreference.current = true;
    const preferred = browserChannels(channels.data.channels).find(
      (item) => item.channelNo === preference?.channelNo,
    );
    if (preferred) {
      setChannel(preferred.channelNo);
      setMode(preferred.modes[0] || "");
    }
  }, [channels.data, preference?.channelNo]);
  useEffect(() => {
    if (!channels.data) return;
    const current = browserChannels(channels.data.channels);
    if (channel && !current.some((item) => item.channelNo === channel)) {
      setChannel("");
      setMode("");
    }
    if (
      !paymentLocation.pathname.endsWith(
        `/invoices/${encodeURIComponent(invoice.invoiceNo)}`,
      )
    )
      return;
    const query = new URLSearchParams(paymentLocation.search);
    let changed = false;
    if (
      query.has("paymentChannel") &&
      !current.some((item) => item.channelNo === query.get("paymentChannel"))
    ) {
      query.delete("paymentChannel");
      changed = true;
    }
    if (
      query.has("useBalance") &&
      (query.get("useBalance") !== "1" || !invoice.canPayFromBalance)
    ) {
      query.delete("useBalance");
      changed = true;
    }
    if (changed)
      paymentNavigate(
        { pathname: paymentLocation.pathname, search: query.toString() },
        { replace: true },
      );
  }, [
    channels.data,
    channel,
    invoice.canPayFromBalance,
    invoice.invoiceNo,
    paymentLocation.pathname,
    paymentLocation.search,
    paymentNavigate,
  ]);
  const storageKey = `customer-tdesign.payment.${invoice.invoiceNo}`;
  useEffect(() => {
    try {
      const pending = JSON.parse(sessionStorage.getItem(storageKey) || "null");
      if (pending && pending.expiresAt > Date.now()) setCreated(pending);
    } catch {
      /* No valid recoverable payment. */
    }
  }, [storageKey]);
  return (
    <Dialog
      visible
      header="支付账单"
      footer={false}
      onClose={() => !action.busy && onClose()}
      closeOnOverlayClick={false}
    >
      <div className="pay-amount">
        {money(invoice.outstandingAmountMinor, invoice.currencyCode)}
      </div>
      <p className="muted">账单 {invoice.invoiceNo}</p>
      {created ? (
        <PaymentHandoff payment={created} />
      ) : (
        <>
          {preference?.useBalance && invoice.canPayFromBalance && (
            <Alert theme="info" title="已优先选择账户余额，请确认后付款" />
          )}
          <FetchState state={channels}>
            <label className="field">
              支付方式
              <Select
                value={channel}
                placeholder="请选择支付方式"
                options={options.map((v) => ({
                  label: v.displayName,
                  value: v.channelNo,
                }))}
                onChange={(v) => {
                  setChannel(String(v));
                  const first =
                    options.find((c) => c.channelNo === v)?.modes[0] || "";
                  setMode(first);
                }}
              />
            </label>
            {selected && (
              <label className="field">
                付款方式
                <Select
                  value={mode}
                  options={selected.modes.map((v) => ({
                    label: modes[v] || "在线支付",
                    value: v,
                  }))}
                  onChange={(v) => setMode(String(v))}
                />
              </label>
            )}
            {!options.length && (
              <Alert theme="info" title="暂未开通在线支付方式" />
            )}
          </FetchState>
          <div className="actions">
            {creditAccount && (
              <Button
                tag="button"
                variant="outline"
                disabled={action.busy}
                onClick={() => setCreditConfirm(true)}
              >
                使用信用额度付款
              </Button>
            )}
            {invoice.canPayFromBalance && (
              <Button
                tag="button"
                theme={preference?.useBalance ? "primary" : "default"}
                variant="outline"
                disabled={action.busy}
                onClick={() => setBalanceConfirm(true)}
              >
                使用余额付款
              </Button>
            )}
            <Button
              tag="button"
              disabled={!channel || !mode || !invoice.orderNo}
              loading={action.busy}
              onClick={() =>
                void action.run(async () => {
                  if (!invoice.orderNo)
                    throw new Error("这笔账单请使用余额付款");
                  const prepared = context
                    ? await resumePaymentPreparation(context.contextNo)
                    : await preparePayment(invoice.orderNo);
                  setContext(prepared.preparation);
                  if (!prepared.preparation.ready)
                    throw new Error("请先完成账户认证，再继续付款");
                  const payment = await payments.createCustomerOrderPayment({
                    paymentContextNo: prepared.preparation.contextNo,
                    channelNo: channel,
                    modeCode: mode,
                    balanceAmountMinor: "0",
                  });
                  sessionStorage.setItem(storageKey, JSON.stringify(payment));
                  setCreated(payment);
                })
              }
            >
              前往支付
            </Button>
          </div>
          <ErrorNote error={action.error} />
        </>
      )}
      {creditConfirm && creditAccount && (
        <Confirm
          title="确认信用付款"
          busy={action.busy}
          error={action.error}
          onClose={() => setCreditConfirm(false)}
          onConfirm={() =>
            void action.run(async () => {
              await settleInvoiceFromCredit(invoice.invoiceNo);
              setCreditConfirm(false);
              onClose();
            })
          }
        >
          <DetailList
            items={[
              [
                "本次付款",
                money(invoice.outstandingAmountMinor, invoice.currencyCode),
              ],
              [
                "付款后可用额度",
                money(
                  (
                    BigInt(creditAccount.availableLimitMinor) -
                    BigInt(invoice.outstandingAmountMinor)
                  ).toString(),
                  invoice.currencyCode,
                ),
              ],
            ]}
          />
          <p>付款后请按信用账单约定按时还款。</p>
        </Confirm>
      )}
      {balanceConfirm && (
        <Confirm
          title="确认使用余额付款"
          busy={action.busy}
          error={action.error}
          onClose={() => setBalanceConfirm(false)}
          onConfirm={() =>
            void action.run(async () => {
              await finance.payFinancialInvoiceFromBalance(
                invoice.invoiceNo,
                actionKey(`balance-${invoice.invoiceNo}`),
              );
              setBalanceConfirm(false);
              onClose();
            })
          }
        >
          <p>
            将从当前币种的可用余额扣除{" "}
            {money(invoice.outstandingAmountMinor, invoice.currencyCode)}。
          </p>
        </Confirm>
      )}
    </Dialog>
  );
}
export function RechargeDialog({ onClose }: { onClose: () => void }) {
  const channels = useLoad(() => payments.listCustomerPaymentChannels(), []),
    action = useAction();
  const [amount, setAmount] = useState("100.00"),
    [currency, setCurrency] = useState("CNY"),
    [channel, setChannel] = useState(""),
    [mode, setMode] = useState(""),
    [created, setCreated] = useState<payments.CustomerPaymentCreated>(() => {
      try {
        const value = JSON.parse(
          sessionStorage.getItem("customer-tdesign.recharge") || "null",
        );
        return value?.expiresAt > Date.now() ? value : undefined;
      } catch {
        return undefined;
      }
    });
  useEffect(() => {
    const availableCurrencies = [
      ...new Set(
        browserChannels(channels.data?.channels || []).flatMap(
          (v) => v.currencies,
        ),
      ),
    ];
    if (availableCurrencies.length && !availableCurrencies.includes(currency)) {
      setCurrency(availableCurrencies[0]);
      setChannel("");
      setMode("");
    }
  }, [channels.data, currency]);
  const available = browserChannels(channels.data?.channels || []).filter((v) =>
    v.currencies.includes(currency),
  );
  const currencies = [
    ...new Set(
      browserChannels(channels.data?.channels || []).flatMap(
        (v) => v.currencies,
      ),
    ),
  ];
  const selected = available.find((v) => v.channelNo === channel);
  return (
    <Dialog
      visible
      header="账户充值"
      footer={false}
      onClose={() => !action.busy && onClose()}
      closeOnOverlayClick={false}
    >
      {created ? (
        <PaymentHandoff payment={created} />
      ) : (
        <>
          <p className="muted">充值到账后可用于付款和续费。</p>
          <FetchState state={channels}>
            {!currencies.length && (
              <Alert
                theme="info"
                message="暂时没有可用的充值方式，请稍后再试或联系商家。"
              />
            )}
            <label className="field">
              币种
              <Select
                value={currency}
                options={currencies.map((v) => ({ label: v, value: v }))}
                onChange={(v) => {
                  setCurrency(String(v));
                  setChannel("");
                  setMode("");
                }}
              />
            </label>
            <label className="field">
              充值金额
              <Input value={amount} onChange={setAmount} />
            </label>
            <label className="field">
              支付方式
              <Select
                value={channel}
                options={available.map((v) => ({
                  label: v.displayName,
                  value: v.channelNo,
                }))}
                onChange={(v) => {
                  setChannel(String(v));
                  setMode(
                    available.find((c) => c.channelNo === v)?.modes[0] || "",
                  );
                }}
              />
            </label>
            {selected && (
              <label className="field">
                付款方式
                <Select
                  value={mode}
                  options={selected.modes.map((v) => ({
                    label: modes[v] || "在线支付",
                    value: v,
                  }))}
                  onChange={(v) => setMode(String(v))}
                />
              </label>
            )}
            <Button
              tag="button"
              block
              disabled={!channel || !mode}
              loading={action.busy}
              onClick={() =>
                void action.run(async () => {
                  const payment = await payments.createCustomerBalanceRecharge({
                    amountMinor: minorAmount(amount, currency),
                    currencyCode: currency,
                    channelNo: channel,
                    modeCode: mode,
                  });
                  sessionStorage.setItem(
                    "customer-tdesign.recharge",
                    JSON.stringify(payment),
                  );
                  setCreated(payment);
                })
              }
            >
              确认充值
            </Button>
          </FetchState>
          <ErrorNote error={action.error} />
        </>
      )}
    </Dialog>
  );
}
export default function Finance() {
  const compliance = useFeature("idc-compliance");
  const location = useLocation(),
    navigate = useNavigate();
  const overview = useLoad(
    finance.getFinancialOverview,
    [],
    "Finance:Finance:overview",
  );
  const [status, setStatus] = useState("");
  const cursor = useCursor(location.pathname + status);
  const transactions = location.pathname.endsWith("/transactions");
  const list = useLoad(
    async () =>
      transactions
        ? {
            invoices: [],
            transactions: await finance.listFinancialTransactions({
              cursor: cursor.cursor,
              limit: 20,
            }),
          }
        : {
            invoices: await finance.listFinancialInvoices({
              statusCode: status,
              cursor: cursor.cursor,
              limit: 20,
            }),
            transactions: undefined,
          },
    [transactions, cursor.cursor, status],
    "Finance:Finance:list",
  );
  const [recharge, setRecharge] = useState(
      new URLSearchParams(location.search).get("recharge") === "1",
    ),
    [pay, setPay] = useState<finance.FinancialInvoice>();
  useEffect(() => {
    if (new URLSearchParams(location.search).get("recharge") === "1")
      setRecharge(true);
  }, [location.search]);
  return (
    <Page
      title={transactions ? "收支明细" : "余额与账单"}
      description={
        transactions ? "每一笔收支都有记录。" : "管理账户余额，查看和支付账单。"
      }
      actions={
        <>
          <Link to="/account/billing/transactions">收支明细</Link>
          {
            <Button tag="button" onClick={() => setRecharge(true)}>
              充值
            </Button>
          }
        </>
      }
    >
      <nav className="page-links" aria-label="费用相关功能">
        <Link to="/account/billing">账单</Link>
        <Link to="/account/billing/transactions">收支明细</Link>
        <Link to="/account/billing/renewals">续费管理</Link>
        <Link to="/account/refunds">退款</Link>
        <Link to="/account/billing/credit">信用账户</Link>
        <Link to="/account/invoice-profiles">发票抬头</Link>
        {compliance && <Link to="/account/tax-invoices">申请发票</Link>}
      </nav>
      <FetchState state={overview}>
        <div className="overview-grid">
          {(overview.data?.overview.balances || []).map((v) => (
            <Card
              key={v.currencyCode + v.accountTypeCode}
              title={
                v.accountTypeCode === "customer_balance"
                  ? "可用现金余额"
                  : "赠送余额"
              }
            >
              <div className="balance-line">
                <strong>
                  {money(v.availableBalanceMinor, v.currencyCode)}
                </strong>
                <small>
                  冻结金额 {money(v.heldBalanceMinor, v.currencyCode)}
                </small>
              </div>
            </Card>
          ))}
        </div>
      </FetchState>
      <Card title={transactions ? "收支记录" : "我的账单"}>
        {!transactions && (
          <div className="table-toolbar">
            <Select
              aria-label="账单状态"
              value={status}
              onChange={(v) => setStatus(String(v))}
              options={[
                { label: "全部账单", value: "" },
                { label: "待付款", value: "open" },
                { label: "已付款", value: "paid" },
                { label: "已退款", value: "refunded" },
                { label: "已取消", value: "cancelled" },
              ]}
              style={{ width: 180 }}
            />
            <Button tag="button" variant="outline" onClick={list.reload}>
              刷新
            </Button>
          </div>
        )}
        <FetchState state={list}>
          {transactions ? (
            <BusinessTable
              rowKey="transactionNo"
              rows={list.data?.transactions?.transactions || []}
              columns={[
                { colKey: "transactionNo", title: "流水编号" },
                {
                  colKey: "transactionTypeCode",
                  title: "类型",
                  cell: ({ row }) => <Status value={row.transactionTypeCode} />,
                },
                {
                  colKey: "changeMinor",
                  title: "金额变化",
                  cell: ({ row }) => money(row.changeMinor, row.currencyCode),
                },
                {
                  colKey: "postedAt",
                  title: "时间",
                  cell: ({ row }) => date(row.postedAt),
                },
                {
                  colKey: "relatedInvoiceNo",
                  title: "相关账单",
                  cell: ({ row }) =>
                    row.relatedInvoiceNo ? (
                      <Link
                        to={`/account/billing/invoices/${row.relatedInvoiceNo}`}
                      >
                        查看账单
                      </Link>
                    ) : (
                      "—"
                    ),
                },
              ]}
            />
          ) : (
            <BusinessTable
              rowKey="invoiceNo"
              rows={
                !Array.isArray(list.data?.invoices)
                  ? list.data?.invoices.invoices || []
                  : []
              }
              columns={[
                {
                  colKey: "invoiceNo",
                  title: "账单编号",
                  cell: ({ row }) => (
                    <Link to={`/account/billing/invoices/${row.invoiceNo}`}>
                      {row.invoiceNo}
                    </Link>
                  ),
                },
                {
                  colKey: "totalAmountMinor",
                  title: "账单金额",
                  cell: ({ row }) =>
                    money(row.totalAmountMinor, row.currencyCode),
                },
                {
                  colKey: "outstandingAmountMinor",
                  title: "待付款",
                  cell: ({ row }) =>
                    money(row.outstandingAmountMinor, row.currencyCode),
                },
                {
                  colKey: "statusCode",
                  title: "状态",
                  cell: ({ row }) => <Status value={row.statusCode} />,
                },
                {
                  colKey: "paymentDueAt",
                  title: "付款截止",
                  cell: ({ row }) => date(row.paymentDueAt),
                },
                {
                  colKey: "actions",
                  title: "操作",
                  fixed: "right",
                  width: 100,
                  cell: ({ row }) => (
                    <Link to={`/account/billing/invoices/${row.invoiceNo}`}>
                      查看详情
                    </Link>
                  ),
                },
              ]}
            />
          )}
          <CursorPager
            history={cursor.history}
            next={
              transactions
                ? list.data?.transactions?.nextCursor
                : !Array.isArray(list.data?.invoices)
                  ? list.data?.invoices.nextCursor
                  : ""
            }
            onNext={() =>
              cursor.next(
                transactions
                  ? list.data?.transactions?.nextCursor || ""
                  : !Array.isArray(list.data?.invoices)
                    ? list.data?.invoices.nextCursor || ""
                    : "",
              )
            }
            onPrevious={cursor.previous}
          />
        </FetchState>
      </Card>
      {recharge && (
        <RechargeDialog
          onClose={() => {
            setRecharge(false);
            const query = new URLSearchParams(location.search);
            query.delete("recharge");
            navigate(location.pathname + (query.size ? "?" + query : ""), {
              replace: true,
            });
            overview.reload();
          }}
        />
      )}
      {pay && (
        <PayDialog
          invoice={pay}
          onClose={() => {
            setPay(undefined);
            list.reload();
            overview.reload();
          }}
        />
      )}
    </Page>
  );
}
export function InvoiceDetail() {
  const { invoiceNo = "" } = useParams();
  const location = useLocation();
  const paymentQuery = new URLSearchParams(location.search);
  const preference = {
    channelNo: paymentQuery.get("paymentChannel") || "",
    useBalance: paymentQuery.get("useBalance") === "1",
  };
  const cursor = useCursor(invoiceNo),
    data = useLoad(
      () => finance.getFinancialInvoice(invoiceNo, cursor.cursor),
      [invoiceNo, cursor.cursor],
      "Finance:InvoiceDetail:data",
    );
  const [pay, setPay] = useState(false),
    invoice = data.data?.detail.invoice;
  const openedInvoice = useRef("");
  useEffect(() => {
    if (
      invoice &&
      openedInvoice.current !== invoice.invoiceNo &&
      paymentQuery.get("pay") === "1" &&
      BigInt(invoice.outstandingAmountMinor) > 0n &&
      (invoice.canPayFromBalance || invoice.orderNo)
    ) {
      openedInvoice.current = invoice.invoiceNo;
      setPay(true);
    }
  }, [invoice, location.search]);
  return (
    <Page
      title="账单详情"
      actions={<Link to="/account/billing">返回账单列表</Link>}
    >
      <FetchState state={data}>
        {invoice && (
          <>
            <Card>
              <div className="service-title">
                <h2>{invoice.invoiceNo}</h2>
                <Status value={invoice.statusCode} />
              </div>
              <DetailList
                items={[
                  [
                    "账单总额",
                    money(invoice.totalAmountMinor, invoice.currencyCode),
                  ],
                  [
                    "已付款",
                    money(invoice.paidAmountMinor, invoice.currencyCode),
                  ],
                  [
                    "待付款",
                    money(invoice.outstandingAmountMinor, invoice.currencyCode),
                  ],
                  ["付款截止", date(invoice.paymentDueAt)],
                ]}
              />
              <div className="actions">
                {BigInt(invoice.outstandingAmountMinor) > 0n &&
                  (invoice.canPayFromBalance || invoice.orderNo) && (
                    <Button tag="button" onClick={() => setPay(true)}>
                      立即付款
                    </Button>
                  )}
                {invoice.orderNo && (
                  <Link to={`/account/orders/${invoice.orderNo}`}>
                    查看订单
                  </Link>
                )}
                <Link to="/account/support/new">账单有疑问</Link>
              </div>
            </Card>
            <Card title="账单明细">
              <BusinessTable
                rowKey="invoiceItemNo"
                rows={data.data?.detail.items || []}
                columns={[
                  {
                    colKey: "descriptionZhCN",
                    title: "项目",
                    cell: ({ row }) =>
                      row.descriptionZhCN || row.descriptionEnUS,
                  },
                  {
                    colKey: "subtotalMinor",
                    title: "原价",
                    cell: ({ row }) =>
                      money(row.subtotalMinor, invoice.currencyCode),
                  },
                  {
                    colKey: "discountMinor",
                    title: "优惠",
                    cell: ({ row }) =>
                      money(row.discountMinor, invoice.currencyCode),
                  },
                  {
                    colKey: "totalAmountMinor",
                    title: "金额",
                    cell: ({ row }) =>
                      money(row.totalAmountMinor, invoice.currencyCode),
                  },
                ]}
              />
              <CursorPager
                history={cursor.history}
                next={data.data?.detail.nextItemCursor}
                onNext={() => cursor.next(data.data!.detail.nextItemCursor)}
                onPrevious={cursor.previous}
              />
            </Card>
            {pay && (
              <PayDialog
                invoice={invoice}
                preference={preference}
                onClose={() => {
                  setPay(false);
                  data.reload();
                }}
              />
            )}
          </>
        )}
      </FetchState>
    </Page>
  );
}
export function Renewals() {
  const cursor = useCursor(),
    data = useLoad(
      () =>
        finance.listFinancialRenewalEntitlements({
          cursor: cursor.cursor,
          limit: 20,
        }),
      [cursor.cursor],
      "Finance:Renewals:data",
    ),
    action = useAction();
  const [renew, setRenew] = useState(""),
    [policy, setPolicy] = useState<finance.FinancialRenewalEntitlement>();
  return (
    <Page title="续费管理" description="查看到期时间，按需续费或设置自动续费。">
      <Card>
        <ErrorNote error={action.error} />
        <FetchState state={data}>
          <BusinessTable
            rowKey="entitlementNo"
            rows={data.data?.entitlements || []}
            columns={[
              {
                colKey: "productNameZhCN",
                title: "服务",
                cell: ({ row }) => row.productNameZhCN || row.productNameEnUS,
              },
              {
                colKey: "entitlementExpiresAt",
                title: "到期时间",
                cell: ({ row }) => date(row.entitlementExpiresAt),
              },
              {
                colKey: "entitlementStatusCode",
                title: "状态",
                cell: ({ row }) => <Status value={row.entitlementStatusCode} />,
              },
              {
                colKey: "autoRenewEnabled",
                title: "自动续费",
                cell: ({ row }) => (
                  <Switch
                    disabled={!row.canConfigureAutomaticRenewal || action.busy}
                    value={row.autoRenewEnabled}
                    onChange={() => setPolicy(row)}
                  />
                ),
              },
              {
                colKey: "actions",
                title: "操作",
                fixed: "right",
                width: 100,
                cell: ({ row }) => (
                  <Button
                    tag="button"
                    variant="text"
                    onClick={() => setRenew(row.entitlementNo)}
                  >
                    续费
                  </Button>
                ),
              },
            ]}
          />
          <CursorPager
            history={cursor.history}
            next={data.data?.nextCursor}
            onNext={() => cursor.next(data.data!.nextCursor)}
            onPrevious={cursor.previous}
          />
        </FetchState>
      </Card>
      {renew && (
        <RenewDialog entitlementNo={renew} onClose={() => setRenew("")} />
      )}{" "}
      {policy && (
        <Confirm
          title={policy.autoRenewEnabled ? "关闭自动续费" : "开启自动续费"}
          busy={action.busy}
          error={action.error}
          onClose={() => setPolicy(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              await finance.updateFinancialRenewalPolicy(policy.entitlementNo, {
                autoRenewEnabled: !policy.autoRenewEnabled,
                expectedEntitlementVersion: policy.entitlementVersion,
                failureNotificationEnabled: policy.failureNotificationEnabled,
                leadSeconds: policy.leadSeconds,
                maximumAttempts: policy.maximumAttempts,
                renewalCycles: policy.renewalCycles,
                retryIntervalSeconds: policy.retryIntervalSeconds,
              });
              setPolicy(undefined);
              data.reload();
            })
          }
        >
          <p>
            {policy.autoRenewEnabled
              ? "关闭后，请在到期前手动续费。"
              : "开启后，系统将按当前服务的续费设置从可用余额付款，请保持余额充足。"}
          </p>
        </Confirm>
      )}
    </Page>
  );
}
export function PaymentReturn() {
  const { attemptNo = "" } = useParams(),
    data = useLoad(
      () => payments.getCustomerPaymentStatus(attemptNo),
      [attemptNo],
      "Finance:PaymentReturn:data",
    );
  const item = data.data;
  useEffect(() => {
    if (
      !item ||
      ["succeeded", "completed", "failed", "cancelled", "expired"].includes(
        item.statusCode,
      )
    )
      return;
    const timer = setTimeout(() => {
      if (document.visibilityState === "visible") data.reload();
    }, 5000);
    return () => clearTimeout(timer);
  }, [item]);
  return (
    <Page title="付款结果">
      <Card>
        <FetchState state={data}>
          {item && (
            <>
              <Status value={item.statusCode} />
              <h2>
                {item.statusCode === "succeeded"
                  ? "付款已完成"
                  : "付款结果以到账记录为准"}
              </h2>
              <DetailList
                items={[
                  ["付款金额", money(item.totalAmountMinor, item.currencyCode)],
                  ["更新时间", date(item.confirmedAt)],
                ]}
              />
              <div className="actions">
                <Button tag="button" variant="outline" onClick={data.reload}>
                  刷新结果
                </Button>
                {item.invoiceNo && (
                  <Link to={`/account/billing/invoices/${item.invoiceNo}`}>
                    返回账单
                  </Link>
                )}
                {item.orderNo && (
                  <Link to={`/account/orders/${item.orderNo}`}>
                    查看开通进度
                  </Link>
                )}
              </div>
            </>
          )}
        </FetchState>
      </Card>
    </Page>
  );
}
