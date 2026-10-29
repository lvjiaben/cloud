import Dialog from "#/adapter/Dialog";
import { useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Input,
  InputNumber,
  Select,
  Textarea,
} from "tdesign-react";
import { Link, useSearchParams } from "react-router-dom";
import * as credit from "#/api/idc/customer-credit";
import * as refunds from "#/api/idc/customer-refunds";
import { getOrder } from "#/api/idc/customer-commerce";
import { getFinancialInvoice } from "#/api/idc/customer-finance";
import { actionKey, clearActionKey } from "#/api/request";
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
  minorAmount,
  money,
  useAction,
  useCursor,
  useLoad,
} from "#/components/business";

export function Credit() {
  const [page, setPage] = useState(1),
    [applicationPage, setApplicationPage] = useState(1),
    [apply, setApply] = useState(false),
    [repay, setRepay] = useState<credit.CreditStatement>();
  const accounts = useLoad(
      credit.getCreditOverview,
      [],
      "AdditionalFinance:Credit:accounts",
    ),
    applications = useLoad(
      () => credit.listCreditApplications(applicationPage, 10),
      [applicationPage],
      "AdditionalFinance:Credit:applications",
    ),
    statements = useLoad(
      () => credit.listCreditStatements(page, 10),
      [page],
      "AdditionalFinance:Credit:statements",
    ),
    action = useAction();
  return (
    <Page
      title="信用账户"
      description="查看可用额度、账单和还款情况。不同币种分别计算。"
      actions={
        <Button tag="button" onClick={() => setApply(true)}>
          申请额度
        </Button>
      }
    >
      <FetchState state={accounts}>
        <div className="overview-grid">
          {accounts.data?.overview.accounts.map((a) => (
            <Card key={a.creditAccountNo} title={`${a.currencyCode} 可用额度`}>
              <strong className="metric small">
                {money(a.availableLimitMinor, a.currencyCode)}
              </strong>
              <p>总额度 {money(a.totalLimitMinor, a.currencyCode)}</p>
              <Status value={a.statusCode} />
              <p>有效至 {date(a.validUntil)}</p>
            </Card>
          ))}
        </div>
        {!accounts.data?.overview.accounts.length && (
          <Card>
            <p>您还没有信用额度，可以提交申请。</p>
          </Card>
        )}
      </FetchState>
      <Card title="待还账单">
        <FetchState state={statements}>
          <BusinessTable
            rowKey="statementNo"
            rows={statements.data?.statements || []}
            total={statements.data?.total}
            page={page}
            size={10}
            onPage={(p) => setPage(p)}
            columns={[
              { colKey: "statementNo", title: "账单编号" },
              {
                colKey: "outstandingAmountMinor",
                title: "待还金额",
                cell: ({ row }) =>
                  money(row.outstandingAmountMinor, row.currencyCode),
              },
              {
                colKey: "paymentDueAt",
                title: "最迟还款时间",
                cell: ({ row }) => date(row.paymentDueAt),
              },
              {
                colKey: "statusCode",
                title: "状态",
                cell: ({ row }) => <Status value={row.statusCode} />,
              },
              {
                colKey: "actions",
                title: "操作",
                cell: ({ row }) =>
                  row.canRepayFromBalance ? (
                    <Button
                      tag="button"
                      variant="text"
                      onClick={() => setRepay(row)}
                    >
                      余额还款
                    </Button>
                  ) : (
                    <Link to="/account/billing">查看余额</Link>
                  ),
              },
            ]}
          />
        </FetchState>
      </Card>
      <Card title="额度申请">
        <FetchState state={applications}>
          <BusinessTable
            rowKey="applicationNo"
            rows={applications.data?.applications || []}
            total={applications.data?.total}
            page={applicationPage}
            size={10}
            onPage={(p) => setApplicationPage(p)}
            columns={[
              { colKey: "applicationNo", title: "申请编号" },
              {
                colKey: "requestedLimitMinor",
                title: "申请额度",
                cell: ({ row }) =>
                  money(row.requestedLimitMinor, row.currencyCode),
              },
              {
                colKey: "statusCode",
                title: "状态",
                cell: ({ row }) => <Status value={row.statusCode} />,
              },
              {
                colKey: "createdAt",
                title: "申请时间",
                cell: ({ row }) => date(row.createdAt),
              },
            ]}
          />
        </FetchState>
      </Card>
      {apply && (
        <CreditApplication
          onClose={() => setApply(false)}
          onDone={() => {
            setApply(false);
            applications.reload();
          }}
        />
      )}
      {repay && (
        <Confirm
          title="确认余额还款"
          busy={action.busy}
          error={action.error}
          onClose={() => setRepay(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              const scope = `credit-repay-${repay.statementNo}-${repay.outstandingAmountMinor}`,
                requestNo = actionKey(scope),
                amountMinor = repay.outstandingAmountMinor;
              const result = await credit.issueBalanceRepaymentProof(
                repay.statementNo,
                { amountMinor, requestNo },
              );
              await credit.repayStatementFromBalance(repay.statementNo, {
                amountMinor,
                requestNo,
                proofToken: result.proof.proofToken,
              });
              clearActionKey(scope);
              setRepay(undefined);
              statements.reload();
              accounts.reload();
            })
          }
        >
          <DetailList
            items={[
              [
                "本次还款",
                money(repay.outstandingAmountMinor, repay.currencyCode),
              ],
              [
                "可用余额",
                money(repay.availableBalanceMinor, repay.currencyCode),
              ],
            ]}
          />
          <p>从同币种余额中扣除，不会使用其他币种。</p>
        </Confirm>
      )}
    </Page>
  );
}
function CreditApplication({
  onClose,
  onDone,
}: {
  onClose: () => void;
  onDone: () => void;
}) {
  const action = useAction(),
    [currency, setCurrency] = useState("CNY"),
    [amount, setAmount] = useState(""),
    [from, setFrom] = useState(""),
    [until, setUntil] = useState(""),
    [files, setFiles] = useState<File[]>([]);
  return (
    <Dialog
      visible
      header="申请信用额度"
      confirmBtn={{ content: "提交申请", loading: action.busy }}
      onClose={() => !action.busy && onClose()}
      onConfirm={() =>
        void action.run(async () => {
          const start = new Date(from).getTime(),
            end = new Date(until).getTime();
          if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start)
            throw new Error("请选择正确的有效期");
          if (!/^[A-Z]{3}$/.test(currency))
            throw new Error("请输入三个字母的币种，如 CNY");
          const requestedLimitMinor = minorAmount(amount, currency);
          if (
            !files.length ||
            files.length > 10 ||
            files.some(
              (f) =>
                f.size > 20 * 1024 * 1024 ||
                !["application/pdf", "image/jpeg", "image/png"].includes(
                  f.type,
                ),
            )
          )
            throw new Error(
              "请上传 1 至 10 份 PDF、JPG 或 PNG，每份不超过 20 MB",
            );
          const uploadNos: string[] = [];
          for (const file of files) {
            const r = await credit.reserveCreditEvidence(file);
            await credit.uploadCreditEvidence(r.uploadNo, file);
            uploadNos.push(r.uploadNo);
          }
          await credit.submitCreditApplication({
            currencyCode: currency,
            requestedLimitMinor,
            requestedValidFrom: start,
            requestedValidUntil: end,
            uploadNos,
            idempotencyKey: actionKey("credit-application"),
          });
          clearActionKey("credit-application");
          onDone();
        })
      }
    >
      <label className="field">
        币种
        <Input
          value={currency}
          onChange={(v) => setCurrency(v.toUpperCase())}
          maxlength={3}
        />
      </label>
      <label className="field">
        申请额度
        <Input value={amount} onChange={setAmount} />
      </label>
      <label className="field">
        开始日期
        <input
          type="date"
          value={from}
          onChange={(e) => setFrom(e.target.value)}
        />
      </label>
      <label className="field">
        结束日期
        <input
          type="date"
          value={until}
          onChange={(e) => setUntil(e.target.value)}
        />
      </label>
      <label className="field">
        证明材料
        <input
          type="file"
          multiple
          accept="application/pdf,image/jpeg,image/png"
          onChange={(e) => setFiles(Array.from(e.target.files || []))}
        />
      </label>
      <p>最多 10 份，每份不超过 20 MB。材料仅供审核人员查看。</p>
      <ErrorNote error={action.error} />
    </Dialog>
  );
}

export function Refunds() {
  const [params] = useSearchParams(),
    orderNo = params.get("orderNo") || "",
    cursor = useCursor(),
    data = useLoad(
      () => refunds.listCustomerRefunds({ cursor: cursor.cursor }),
      [cursor.cursor],
      "AdditionalFinance:Refunds:data",
    ),
    action = useAction();
  const [respond, setRespond] = useState<{
    item: refunds.CustomerRefund;
    consent: boolean;
  }>();
  return (
    <Page
      title="退款"
      description="查看退款进度。申请退款请先选择对应订单。"
      actions={
        <Link className="primary-link" to="/account/orders">
          选择订单
        </Link>
      }
    >
      {orderNo && (
        <RefundApplication
          key={orderNo}
          orderNo={orderNo}
          onDone={data.reload}
        />
      )}
      <Card title="退款记录">
        <FetchState state={data}>
          <BusinessTable
            rowKey="refundNo"
            rows={data.data?.refunds || []}
            columns={[
              { colKey: "refundNo", title: "退款编号" },
              {
                colKey: "orderNo",
                title: "关联订单",
                cell: ({ row }) => (
                  <Link
                    to={`/account/orders/${encodeURIComponent(row.orderNo)}`}
                  >
                    {row.orderNo}
                  </Link>
                ),
              },
              {
                colKey: "amountMinor",
                title: "退款金额",
                cell: ({ row }) => money(row.amountMinor, row.currencyCode),
              },
              {
                colKey: "statusCode",
                title: "进度",
                cell: ({ row }) => <Status value={row.statusCode} />,
              },
              {
                colKey: "submittedAt",
                title: "申请时间",
                cell: ({ row }) => date(row.submittedAt),
              },
              {
                colKey: "actions",
                title: "操作",
                cell: ({ row }) =>
                  row.statusCode === "awaiting_balance_consent" ? (
                    <div className="actions">
                      <Button
                        tag="button"
                        variant="text"
                        onClick={() => setRespond({ item: row, consent: true })}
                      >
                        同意退到余额
                      </Button>
                      <Button
                        tag="button"
                        variant="text"
                        onClick={() =>
                          setRespond({ item: row, consent: false })
                        }
                      >
                        不同意
                      </Button>
                    </div>
                  ) : (
                    "—"
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
      {respond && (
        <Confirm
          title={respond.consent ? "确认退款方式" : "保留待处理退款"}
          busy={action.busy}
          error={action.error}
          onClose={() => setRespond(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              await refunds.respondCustomerRefundBalance({
                refundNo: respond.item.refundNo,
                consent: respond.consent,
                expectedVersion: respond.item.version,
                idempotencyKey: actionKey(
                  `refund-consent-${respond.item.refundNo}-${respond.item.version}-${respond.consent}`,
                ),
              });
              setRespond(undefined);
              data.reload();
            })
          }
        >
          <p>
            {respond.consent
              ? "原付款方式已确认无法退回的部分将退到同币种余额，其余部分仍按原路退回。"
              : "不同意后，退款将继续等待商家处理，不会自动转入余额。"}
          </p>
          <p>
            本笔退款{" "}
            {money(respond.item.amountMinor, respond.item.currencyCode)}
          </p>
        </Confirm>
      )}
    </Page>
  );
}
function RefundApplication({
  orderNo,
  onDone,
}: {
  orderNo: string;
  onDone: () => void;
}) {
  const data = useLoad(async () => {
      const { order } = await getOrder(orderNo);
      const invoiceNo = String(order.invoice?.invoiceNo || "");
      if (!invoiceNo) throw new Error("这个订单还没有可退款账单");
      const { detail } = await getFinancialInvoice(invoiceNo);
      return { order, detail };
    }, [orderNo]),
    action = useAction();
  const [selections, setSelections] = useState<
      Record<
        string,
        {
          selected: boolean;
          amount: string;
          quantity: number;
          entitlementNos: string[];
        }
      >
    >({}),
    [preview, setPreview] = useState<refunds.CustomerRefundPreview>(),
    [inputs, setInputs] = useState<refunds.CustomerRefundSelection[]>([]),
    [reason, setReason] = useState(""),
    [detail, setDetail] = useState(""),
    [confirmed, setConfirmed] = useState(false),
    [submitted, setSubmitted] = useState(false);
  const currency = data.data?.detail.invoice.currencyCode || "CNY";
  return (
    <Card title="申请订单退款">
      <FetchState state={data}>
        <ErrorNote error={action.error} />
        {submitted ? (
          <Alert
            theme="success"
            title="退款申请已提交，请在下方查看处理进度。"
          />
        ) : (
          <>
            <p>
              订单 {orderNo} · 当前账单可退{" "}
              {money(data.data?.detail.invoice.refundableAmountMinor, currency)}
            </p>
            {data.data?.detail.items.map((item) => {
              const value = selections[item.invoiceItemNo] || {
                selected: false,
                amount: "",
                quantity: 1,
                entitlementNos: [],
              };
              return (
                <div className="refund-line" key={item.invoiceItemNo}>
                  <Checkbox
                    checked={value.selected}
                    onChange={(selected) =>
                      setSelections((s) => ({
                        ...s,
                        [item.invoiceItemNo]: { ...value, selected },
                      }))
                    }
                  >
                    {item.descriptionZhCN || item.descriptionEnUS}
                  </Checkbox>
                  <label className="field">
                    退款数量
                    <InputNumber
                      min={1}
                      value={value.quantity}
                      onChange={(n) =>
                        setSelections((s) => ({
                          ...s,
                          [item.invoiceItemNo]: {
                            ...value,
                            quantity: Number(n),
                          },
                        }))
                      }
                    />
                  </label>
                  <label className="field">
                    退款金额（{currency}）
                    <Input
                      value={value.amount}
                      onChange={(amount) =>
                        setSelections((s) => ({
                          ...s,
                          [item.invoiceItemNo]: { ...value, amount },
                        }))
                      }
                    />
                  </label>
                  {data.data?.order.serviceEntitlements
                    .filter((e) => e.orderItemNo === item.sourceReference)
                    .map((e) => (
                      <Checkbox
                        key={e.entitlementNo}
                        checked={value.entitlementNos.includes(e.entitlementNo)}
                        onChange={(checked) =>
                          setSelections((s) => ({
                            ...s,
                            [item.invoiceItemNo]: {
                              ...value,
                              selected: true,
                              entitlementNos: checked
                                ? [...value.entitlementNos, e.entitlementNo]
                                : value.entitlementNos.filter(
                                    (n) => n !== e.entitlementNo,
                                  ),
                            },
                          }))
                        }
                      >
                        关联服务：{e.productNameZhCN || e.productNameEnUS} ·{" "}
                        {date(e.startedAt)}
                      </Checkbox>
                    ))}
                </div>
              );
            })}
            {data.data?.detail.nextItemCursor && (
              <Alert title="该账单包含更多明细，请联系商家协助核对全部明细后申请。" />
            )}
            <Button
              tag="button"
              loading={action.busy}
              onClick={() =>
                void action.run(async () => {
                  const chosen = Object.entries(selections)
                    .filter(([, v]) => v.selected)
                    .map(([invoiceItemNo, v]) => ({
                      invoiceItemNo,
                      quantity: v.quantity,
                      amountMinor: minorAmount(v.amount, currency),
                      entitlementNos: v.entitlementNos,
                    }));
                  if (!chosen.length)
                    throw new Error("请选择退款项目并填写金额");
                  const result = await refunds.previewCustomerRefund(
                    orderNo,
                    chosen,
                  );
                  setInputs(chosen);
                  setPreview(result.preview);
                  setConfirmed(false);
                })
              }
            >
              查看退款影响
            </Button>
          </>
        )}
      </FetchState>
      {preview && (
        <Dialog
          visible
          width={640}
          header="确认退款申请"
          confirmBtn={{
            content: "确认并提交",
            loading: action.busy,
            disabled: !confirmed || !reason,
          }}
          onClose={() => !action.busy && setPreview(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              if (reason === "other" && !detail.trim())
                throw new Error("请填写退款原因");
              await refunds.submitCustomerRefund({
                orderNo,
                selections: inputs,
                previewDigest: preview.impactDigest,
                reasonCode: reason,
                reasonDetail: detail,
                idempotencyKey: actionKey(
                  `refund-${orderNo}-${preview.impactDigest}`,
                ),
              });
              setPreview(undefined);
              setSubmitted(true);
              onDone();
            })
          }
        >
          <p>
            预计退款{" "}
            <strong>{money(preview.amountMinor, preview.currencyCode)}</strong>
            ，
            {preview.approvalModeCode === "manual"
              ? "需要商家审核"
              : "按规则自动审核"}
            。
          </p>
          {preview.items.map((i) => (
            <div className="notice" key={i.invoiceItemNo}>
              <strong>{i.descriptionZhCN || i.descriptionEnUS}</strong>
              <p>
                {i.quantity} 项 · {money(i.amountMinor, preview.currencyCode)}
              </p>
              {i.impacts.map((impact, index) => (
                <p key={index}>
                  {impact.productNameZhCN || impact.productNameEnUS}：
                  {impact.requiresTermination
                    ? "退款执行后停止使用"
                    : "按服务规则处理"}
                </p>
              ))}
            </div>
          ))}
          {preview.requiresEntitlementTermination && (
            <Alert
              theme="warning"
              title="退款会结束相关服务。请先备份需要保留的数据；具体数据保留规则请向商家确认。"
            />
          )}
          {preview.hasDangerousOperation && (
            <Alert
              theme="warning"
              title="相关服务有操作尚未确认，退款需商家核对后处理。"
            />
          )}
          <label className="field">
            退款原因
            <Select
              value={reason}
              onChange={(v) => setReason(String(v))}
              options={[
                ["duplicate_purchase", "重复购买"],
                ["wrong_configuration", "配置选错"],
                ["service_issue", "服务问题"],
                ["no_longer_needed", "不再需要"],
                ["other", "其他"],
              ].map(([value, label]) => ({ value, label }))}
            />
          </label>
          <label className="field">
            补充说明
            <Textarea value={detail} onChange={setDetail} />
          </label>
          <Checkbox checked={confirmed} onChange={setConfirmed}>
            我已核对退款金额与服务停止影响
          </Checkbox>
          <ErrorNote error={action.error} />
        </Dialog>
      )}
    </Card>
  );
}
