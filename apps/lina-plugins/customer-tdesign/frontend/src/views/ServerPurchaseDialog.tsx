import { useState } from "react";
import { Link } from "react-router-dom";
import { Alert, Button, Form, Input, Select } from "tdesign-react";
import Dialog from "#/adapter/Dialog";
import { actionKey, clearActionKey } from "#/api/request";
import * as api from "#/api/idc/native-servers";
import {
  DetailList,
  ErrorNote,
  FetchState,
  date,
  money,
  useAction,
  useLoad,
} from "#/components/business";

export default function ServerPurchaseDialog({
  item,
  code,
  title,
  options,
  targetQuoteNo,
  onClose,
  onCreated,
}: {
  item: api.NativeServerDetail;
  code: api.CommercialAction;
  title: string;
  options?: api.NativeOptions;
  targetQuoteNo?: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const packages = useLoad(
    () =>
      code === "traffic_add"
        ? api.listNativeTrafficPackages(item.serverNo)
        : Promise.resolve({ items: [] }),
    [item.serverNo, code],
  );
  const [name, setName] = useState(""),
    [disk, setDisk] = useState(""),
    [packageNo, setPackage] = useState("");
  const [quoteKey, setQuoteKey] = useState(() => crypto.randomUUID());
  const [purchaseKey, setPurchaseKey] = useState("");
  const [quote, setQuote] = useState<api.NativeCommercialQuote>();
  const [proof, setProof] = useState<api.NativeConfirmation>();
  const [attempted, setAttempted] = useState(false);
  const [purchase, setPurchase] = useState<api.NativePurchase>();
  const action = useAction();
  const objectCreate = code === "snapshot_create" || code === "backup_create";
  const edit = (setter: (value: string) => void, value: string) => {
    setter(value);
    setQuoteKey(crypto.randomUUID());
  };
  const submit = async (currentProof: api.NativeConfirmation) => {
    setAttempted(true);
    const result = await api.createNativePurchase(
      item.serverNo,
      quote!.quoteNo,
      purchaseKey,
      currentProof.proofToken,
    );
    if (!result.operationNo || result.quoteNo !== quote!.quoteNo)
      throw new Error("暂时无法核对提交结果，请重试本次提交");
    setPurchase(result);
    clearActionKey(`server-purchase-${item.serverNo}-${result.quoteNo}`);
    onCreated();
  };
  return (
    <Dialog
      visible
      header={title}
      closeOnOverlayClick={false}
      onClose={() => !action.busy && onClose()}
      footer={
        purchase ? (
          <Button tag="button" onClick={onClose}>
            关闭
          </Button>
        ) : (
          <>
            <Button
              tag="button"
              variant="outline"
              disabled={action.busy}
              onClick={onClose}
            >
              取消
            </Button>
            <Button
              tag="button"
              loading={action.busy}
              onClick={() =>
                void action.run(async () => {
                  if (!quote) {
                    if (objectCreate && (!disk || !name.trim()))
                      throw new Error("请选择磁盘并填写名称");
                    if (code === "traffic_add" && !packageNo)
                      throw new Error("请选择流量套餐");
                    if (
                      objectCreate &&
                      options?.accessoriesExpiresAt &&
                      options.accessoriesExpiresAt <= Date.now()
                    )
                      throw new Error(
                        "磁盘信息已过期，请关闭窗口并刷新管理选项",
                      );
                    const result = await api.quoteNativePurchase(
                      item.serverNo,
                      {
                        action: code,
                        ...(targetQuoteNo ? { targetQuoteNo } : {}),
                        parameters: objectCreate
                          ? { diskObjectNo: disk, name: name.trim() }
                          : {},
                        ...(code === "traffic_add" ? { packageNo } : {}),
                        idempotencyKey: quoteKey,
                      },
                    );
                    if (
                      !result.quoteNo ||
                      result.action !== code ||
                      !/^\d+$/.test(result.totalAmountMinor) ||
                      !/^\d+$/.test(result.taxAmountMinor) ||
                      result.expiresAt <= Date.now()
                    )
                      throw new Error("报价暂时不可用，请重新获取");
                    setQuote(result);
                    setPurchaseKey(
                      actionKey(
                        `server-purchase-${item.serverNo}-${result.quoteNo}`,
                      ),
                    );
                    return;
                  }
                  if (!attempted && quote.expiresAt <= Date.now())
                    throw new Error("报价已过期，请关闭窗口后重新获取报价");
                  if (!proof) {
                    const result = await api.prepareNativePurchase(
                      item.serverNo,
                      quote.quoteNo,
                      purchaseKey,
                    );
                    if (
                      typeof result.required !== "boolean" ||
                      typeof result.proofToken !== "string" ||
                      (result.required && !result.proofToken)
                    )
                      throw new Error("暂时无法确认操作，请稍后重试");
                    setProof(result);
                    if (!result.required) await submit(result);
                    return;
                  }
                  if (
                    !attempted &&
                    proof.expiresAt &&
                    proof.expiresAt <= Date.now()
                  ) {
                    setProof(undefined);
                    throw new Error("操作确认已过期，请重新确认");
                  }
                  await submit(proof);
                })
              }
            >
              {action.busy
                ? "正在处理"
                : !quote
                  ? "获取报价"
                  : attempted
                    ? "重试本次提交"
                    : proof?.required
                      ? "确认操作并提交"
                      : "确认价格并继续"}
            </Button>
          </>
        )
      }
    >
      {purchase ? (
        <>
          <Alert
            theme={
              purchase.status === "failed" ||
              purchase.status === "manual_review"
                ? "warning"
                : "info"
            }
            message={
              purchase.status === "awaiting_financial_settlement"
                ? "订单已创建，请先支付账单。付款成功后才会执行。"
                : purchase.status === "succeeded"
                  ? "操作已完成。"
                  : purchase.status === "failed"
                    ? "操作未完成，请查看记录或提交工单。"
                    : purchase.status === "manual_review"
                      ? "处理结果需要核对，请通过工单联系我们，勿重复提交。"
                      : "申请已受理，正在等待处理。"
            }
          />
          <DetailList
            items={[
              ["操作编号", purchase.operationNo],
              [
                "本次金额",
                money(purchase.totalAmountMinor, purchase.currencyCode),
              ],
            ]}
          />
          <div className="actions">
            {purchase.invoiceNo && (
              <Link
                to={`/account/billing/invoices/${encodeURIComponent(purchase.invoiceNo)}`}
              >
                查看并支付账单
              </Link>
            )}
            {purchase.orderNo && (
              <Link
                to={`/account/orders/${encodeURIComponent(purchase.orderNo)}`}
              >
                查看订单
              </Link>
            )}
          </div>
        </>
      ) : quote ? (
        <>
          <Alert
            theme={code === "ip_change" ? "warning" : "info"}
            message={
              code === "ip_change"
                ? "换 IP 后原地址将不能继续使用，网站解析和连接配置也需要更新。请确认已做好准备。"
                : code === "resize"
                  ? "配置调整可能中断连接。请核对本次费用与生效时间，付款后才会执行需要收费的变更。"
                  : objectCreate
                    ? "创建期间请勿重复提交。付费对象按下方周期自动续费，删除后停止后续续费，当前周期不退款。"
                    : "流量包只增加当前服务器的临时流量，不会自动开机或解除暂停。"
            }
          />
          <DetailList
            items={[
              ["服务器", item.serverNo],
              ...(objectCreate
                ? ([
                    ["名称", name.trim()],
                    [
                      "磁盘",
                      options?.accessories.find((o) => o.objectNo === disk)
                        ?.name || disk,
                    ],
                    ["每台免费份数", String(quote.freeCountPerServer)],
                    ["现有份数", String(quote.existingObjectCount)],
                    ["本次付费份数", String(quote.chargedQuantity)],
                  ] as [string, string][])
                : []),
              ...(code === "traffic_add"
                ? ([["增加流量", `${quote.trafficGb} GB`]] as [
                    string,
                    string,
                  ][])
                : []),
              ...(code === "resize"
                ? ([
                    [
                      "变更方式",
                      quote.direction === "upgrade"
                        ? "升级"
                        : quote.direction === "downgrade"
                          ? "下次续费时降级"
                          : "配置调整",
                    ],
                    [
                      "生效时间",
                      quote.effectiveAt
                        ? date(quote.effectiveAt)
                        : "付款结清后处理",
                    ],
                    [
                      "调整前周期价格",
                      money(quote.currentRecurringMinor, quote.currencyCode),
                    ],
                    [
                      "调整后周期价格",
                      money(quote.targetRecurringMinor, quote.currencyCode),
                    ],
                  ] as [string, string][])
                : []),
              ["本次支付", money(quote.totalAmountMinor, quote.currencyCode)],
              [
                "其中税费（已包含）",
                money(quote.taxAmountMinor, quote.currencyCode),
              ],
              ...(objectCreate && quote.chargedQuantity > 0
                ? ([
                    [
                      "计费周期",
                      `${quote.periodCount} ${({ day: "天", month: "个月", year: "年" } as Record<string, string>)[quote.periodUnit] || "（周期待确认）"}`,
                    ],
                  ] as [string, string][])
                : []),
              ["报价有效至", date(quote.expiresAt)],
            ]}
          />
          {proof?.required && (
            <Alert
              theme="warning"
              message="请再次核对服务器、操作影响和最终金额，确认后提交。"
            />
          )}
        </>
      ) : (
        <Form disabled={action.busy} labelAlign="top">
          {objectCreate && (
            <>
              <Form.FormItem label="选择磁盘">
                <Select
                  filterable
                  value={disk}
                  placeholder="选择要保存的磁盘"
                  options={(options?.accessories || [])
                    .filter((o) => o.kind === "disk")
                    .map((o) => ({
                      value: o.objectNo,
                      label: o.name || o.objectNo,
                    }))}
                  onChange={(v) => edit(setDisk, String(v))}
                />
              </Form.FormItem>
              <Form.FormItem label="名称">
                <Input
                  value={name}
                  maxlength={128}
                  onChange={(v) => edit(setName, v)}
                  placeholder="例如：系统更新前"
                />
              </Form.FormItem>
              <p className="muted">免费份数和超出后的费用以后台报价为准。</p>
            </>
          )}
          {code === "traffic_add" && (
            <FetchState state={packages}>
              <Form.FormItem label="流量套餐">
                <Select
                  value={packageNo}
                  placeholder="选择流量套餐"
                  options={(packages.data?.items || []).map((p) => ({
                    value: p.packageNo,
                    label: `${p.nameZhCn || p.nameEnUs} · ${p.trafficGb} GB · ${money(p.amountMinor, p.currencyCode)}${!p.unlimitedStock && p.remainingStock === 0 ? " · 已售罄" : ""}`,
                    disabled:
                      !p.unlimitedStock &&
                      (!p.remainingStock || p.remainingStock < 1),
                  }))}
                  onChange={(v) => edit(setPackage, String(v))}
                />
              </Form.FormItem>
              {!packages.data?.items.length && (
                <p className="muted">暂时没有可购买的流量套餐。</p>
              )}
            </FetchState>
          )}
          {code === "ip_change" && (
            <Alert
              theme="warning"
              message="更换后需要更新使用原 IP 的解析和连接配置，可能中断当前连接。费用以后台报价为准。"
            />
          )}
        </Form>
      )}
      {code === "traffic_add" && (
        <p className="muted">
          有效期按上游重置规则执行，暂未提供具体到期时间。
        </p>
      )}
      <ErrorNote error={action.error} />
    </Dialog>
  );
}
