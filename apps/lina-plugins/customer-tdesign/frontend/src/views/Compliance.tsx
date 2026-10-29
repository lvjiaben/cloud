import Dialog from "#/adapter/Dialog";
import { useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  Select,
} from "tdesign-react";
import { Link } from "react-router-dom";
import * as profiles from "#/api/idc/invoice-profiles";
import * as tax from "#/api/idc/customer-tax-invoices";
import * as personal from "#/api/idc/personal-verification";
import { actionKey, clearActionKey, externalURL } from "#/api/request";
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
  useAction,
  useCursor,
  useLoad,
} from "#/components/business";
import EnterpriseVerification from "./EnterpriseVerification";

export function InvoiceProfiles() {
  const cursor = useCursor(),
    data = useLoad(
      () => profiles.listInvoiceProfiles({ cursor: cursor.cursor, limit: 20 }),
      [cursor.cursor],
      "Compliance:InvoiceProfiles:data",
    ),
    action = useAction();
  const [edit, setEdit] = useState<
      profiles.InvoiceProfileItem | null | undefined
    >(),
    [archive, setArchive] = useState<profiles.InvoiceProfileItem>(),
    [type, setType] = useState("individual");
  return (
    <Page
      title="发票抬头"
      description="保存常用抬头，申请发票时直接选择。"
      actions={
        <Button
          tag="button"
          onClick={() => {
            action.clear();
            setType("individual");
            setEdit(null);
          }}
        >
          添加抬头
        </Button>
      }
    >
      <Card>
        <ErrorNote error={action.error} />
        <FetchState state={data}>
          <BusinessTable
            rowKey="profileNo"
            rows={data.data?.items || []}
            columns={[
              { colKey: "invoiceTitle", title: "抬头名称" },
              {
                colKey: "titleTypeCode",
                title: "类型",
                cell: ({ row }) =>
                  row.titleTypeCode === "enterprise" ? "企业" : "个人",
              },
              { colKey: "taxRegistrationNo", title: "纳税人识别号" },
              {
                colKey: "isDefault",
                title: "默认抬头",
                cell: ({ row }) => (row.isDefault ? "默认" : "—"),
              },
              {
                colKey: "actions",
                title: "操作",
                cell: ({ row }) =>
                  row.statusCode === "active" ? (
                    <div className="actions">
                      <Button
                        tag="button"
                        variant="text"
                        onClick={() => {
                          action.clear();
                          setType(row.titleTypeCode);
                          setEdit(row);
                        }}
                      >
                        编辑
                      </Button>
                      {!row.isDefault && (
                        <Button
                          tag="button"
                          variant="text"
                          disabled={action.busy}
                          onClick={() =>
                            void action.run(async () => {
                              await profiles.setDefaultInvoiceProfile(
                                row.profileNo,
                                row.version,
                              );
                              data.reload();
                            })
                          }
                        >
                          设为默认
                        </Button>
                      )}
                      <Button
                        tag="button"
                        variant="text"
                        theme="danger"
                        onClick={() => setArchive(row)}
                      >
                        移除
                      </Button>
                    </div>
                  ) : (
                    "已移除"
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
      {edit !== undefined && (
        <Dialog
          visible
          header={edit ? "编辑抬头" : "添加抬头"}
          footer={false}
          onClose={() => !action.busy && setEdit(undefined)}
        >
          <Form
            initialData={edit ? { ...edit, bankAccount: "" } : undefined}
            labelAlign="top"
            onSubmit={({ fields, validateResult }) => {
              if (validateResult === true)
                void action.run(async () => {
                  const scope = `invoice-profile-${edit?.profileNo || "new"}-${edit?.version || 0}`;
                  const input = {
                    invoiceTitle: String(fields.invoiceTitle),
                    ...(type === "enterprise"
                      ? {
                          taxRegistrationNo: fields.taxRegistrationNo,
                          registeredAddress: fields.registeredAddress,
                          registeredPhone: fields.registeredPhone,
                          bankName: fields.bankName,
                          bankAccount: fields.bankAccount,
                        }
                      : {}),
                    titleTypeCode: type,
                    isDefault: false,
                    idempotencyKey: actionKey(scope),
                  } as profiles.InvoiceProfileCreateInput;
                  if (edit) {
                    const { isDefault: _default, ...update } = input;
                    await profiles.updateInvoiceProfile(edit.profileNo, {
                      ...update,
                      expectedVersion: edit.version,
                      bankAccountAction:
                        type === "individual"
                          ? "clear"
                          : fields.bankAccount
                            ? "replace"
                            : "keep",
                    });
                  } else await profiles.createInvoiceProfile(input);
                  clearActionKey(scope);
                  setEdit(undefined);
                  data.reload();
                });
            }}
          >
            <label className="field">
              抬头类型
              <Select
                value={type}
                onChange={(v) => setType(String(v))}
                options={[
                  { label: "个人", value: "individual" },
                  { label: "企业", value: "enterprise" },
                ]}
              />
            </label>
            {[
              ["invoiceTitle", "抬头名称"],
              ...(type === "enterprise"
                ? [
                    ["taxRegistrationNo", "纳税人识别号"],
                    ["registeredAddress", "注册地址"],
                    ["registeredPhone", "联系电话"],
                    ["bankName", "开户银行"],
                    ["bankAccount", "银行账号（留空保留原账号）"],
                  ]
                : []),
            ].map(([name, label]) => (
              <Form.FormItem
                name={name}
                label={label}
                key={name}
                rules={
                  ["invoiceTitle", "taxRegistrationNo"].includes(name)
                    ? [{ required: true, message: `请输入${label}` }]
                    : []
                }
              >
                <Input
                  autocomplete="off"
                  ref={(input) =>
                    input?.inputElement.setAttribute("aria-label", label)
                  }
                />
              </Form.FormItem>
            ))}
            <ErrorNote error={action.error} />
            <Button tag="button" type="submit" loading={action.busy}>
              保存
            </Button>
          </Form>
        </Dialog>
      )}
      {archive && (
        <Confirm
          title="移除发票抬头"
          busy={action.busy}
          error={action.error}
          onClose={() => setArchive(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              await profiles.deactivateInvoiceProfile(
                archive.profileNo,
                archive.version,
              );
              setArchive(undefined);
              data.reload();
            })
          }
        >
          <p>确认移除“{archive.invoiceTitle}”？历史发票仍会保留。</p>
        </Confirm>
      )}
    </Page>
  );
}

export function TaxInvoices() {
  const cursor = useCursor(),
    data = useLoad(
      () => tax.listCustomerTaxInvoices({ cursor: cursor.cursor, limit: 20 }),
      [cursor.cursor],
      "Compliance:TaxInvoices:data",
    );
  const [open, setOpen] = useState(false);
  return (
    <Page
      title="发票管理"
      description="为已付款的账单申请发票，跟进开票结果。"
      actions={
        <>
          <Link to="/account/billing/invoice-profiles">管理抬头</Link>
          <Button tag="button" onClick={() => setOpen(true)}>
            申请发票
          </Button>
        </>
      }
    >
      <Card>
        <FetchState state={data}>
          <BusinessTable
            rowKey="taxInvoiceNo"
            rows={data.data?.items || []}
            columns={[
              { colKey: "taxInvoiceNo", title: "申请编号" },
              {
                colKey: "grossAmountMinor",
                title: "开票金额",
                cell: ({ row }) =>
                  money(row.grossAmountMinor, row.currencyCode),
              },
              {
                colKey: "statusCode",
                title: "状态",
                cell: ({ row }) => <Status value={row.statusCode} />,
              },
              { colKey: "maskedInvoiceNo", title: "发票号码" },
              {
                colKey: "submittedAt",
                title: "申请时间",
                cell: ({ row }) => date(row.submittedAt),
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
      {open && (
        <TaxApplication
          onClose={() => setOpen(false)}
          onDone={() => {
            setOpen(false);
            data.reload();
          }}
        />
      )}
    </Page>
  );
}
function TaxApplication({
  onClose,
  onDone,
}: {
  onClose: () => void;
  onDone: () => void;
}) {
  const data = useLoad(tax.getCustomerTaxInvoiceOptions, []),
    action = useAction();
  const [profile, setProfile] = useState(""),
    [selected, setSelected] = useState<string[]>([]);
  const chosen =
    data.data?.invoices.filter((i) => selected.includes(i.billingInvoiceNo)) ||
    [];
  return (
    <Dialog
      visible
      header="申请发票"
      confirmBtn={{
        content: "提交申请",
        loading: action.busy,
        disabled: !profile || !selected.length,
      }}
      onClose={() => !action.busy && onClose()}
      onConfirm={() =>
        void action.run(async () => {
          if (new Set(chosen.map((i) => i.currencyCode)).size > 1)
            throw new Error("请分别申请不同币种的发票");
          await tax.createCustomerTaxInvoice({
            idempotencyKey: actionKey("tax-invoice-create"),
            profileNo: profile,
            selections: chosen.map((i) => ({
              billingInvoiceNo: i.billingInvoiceNo,
              requestedGrossMinor: i.remainingGrossMinor,
            })),
          });
          clearActionKey("tax-invoice-create");
          onDone();
        })
      }
    >
      <FetchState state={data}>
        <label className="field">
          发票抬头
          <Select
            value={profile}
            onChange={(v) => setProfile(String(v))}
            options={data.data?.profiles.map((p) => ({
              label: p.invoiceTitle,
              value: p.profileNo,
            }))}
          />
        </label>
        {!data.data?.profiles.length && (
          <Link to="/account/billing/invoice-profiles">先添加发票抬头</Link>
        )}
        <h3>选择需要开票的账单</h3>
        {data.data?.invoices.map((i) => (
          <label className="selection-row" key={i.billingInvoiceNo}>
            <Checkbox
              checked={selected.includes(i.billingInvoiceNo)}
              onChange={(checked) =>
                setSelected((s) =>
                  checked
                    ? [...s, i.billingInvoiceNo]
                    : s.filter((n) => n !== i.billingInvoiceNo),
                )
              }
            />
            <span>{i.billingInvoiceNo}</span>
            <strong>{money(i.remainingGrossMinor, i.currencyCode)}</strong>
          </label>
        ))}
        {!data.data?.invoices.length && <p>暂无可以申请发票的账单。</p>}
        <ErrorNote error={action.error} />
      </FetchState>
    </Dialog>
  );
}

export function Verification() {
  const data = useLoad(
      personal.getPersonalVerification,
      [],
      "Compliance:Verification:data",
    ),
    action = useAction();
  const [accepted, setAccepted] = useState(false),
    [name, setName] = useState(""),
    [number, setNumber] = useState(""),
    [continuation, setContinuation] = useState("");
  const item = data.data?.item;
  const canStart =
    !item || ["expired", "rejected", "unverified"].includes(item.statusCode);
  return (
    <Page
      title="平台认证"
      description="先确认本人身份，认证通过后可继续办理企业认证。"
    >
      <Card title="个人实名认证">
        <FetchState state={data}>
          {item && (
            <DetailList
              items={[
                ["认证状态", <Status value={item.statusCode} />],
                ["姓名", item.maskedName || "—"],
                ["证件号码", item.maskedIdentityNo || "—"],
              ]}
            />
          )}
          <ErrorNote error={action.error} />
          {canStart && (
            <div className="form-width">
              <p>
                支持年满 14 周岁的中国大陆居民身份证。其他证件请联系商家协助。
              </p>
              <label className="field">
                本人姓名
                <Input value={name} onChange={setName} autocomplete="off" />
              </label>
              <label className="field">
                身份证号码
                <Input
                  value={number}
                  onChange={setNumber}
                  autocomplete="off"
                  maxlength={18}
                />
              </label>
              <div className="notice">
                <h3>敏感个人信息单独同意</h3>
                <p>
                  为完成首次付款实名核验、防止身份冒用，商家通过其阿里云账号处理您的姓名、身份证号码；人脸和活体核验由阿里云页面直接采集，您无需承担技术账号费用。
                </p>
                <p>
                  处理区域为中国大陆。本系统不保存人脸图片或活体视频，仅保留脱敏结果、必要证据及受保护的加密快照；保留期限以适用法律和商家隐私政策为准。
                </p>
                <p>
                  您可联系商家查询、更正、依法删除信息或撤回后续处理同意。不同意可能无法完成首次付款；撤回不影响此前合法处理。
                </p>
                <small>告知版本：{data.data?.policyVersion}</small>
              </div>
              <Checkbox checked={accepted} onChange={setAccepted}>
                我已阅读并单独同意上述处理，确认提交本人信息
              </Checkbox>
              <p>
                <Button
                  tag="button"
                  disabled={
                    !accepted || !name.trim() || !/^\d{17}[\dXx]$/.test(number)
                  }
                  loading={action.busy}
                  onClick={() =>
                    void action.run(async () => {
                      let certName = name.trim(),
                        certNo = number.trim().toUpperCase();
                      setName("");
                      setNumber("");
                      try {
                        const consent =
                          await personal.createPersonalVerificationConsent({
                            policyVersion: data.data!.policyVersion,
                            locale: "zh-CN",
                            idempotencyKey: actionKey("personal-consent"),
                          });
                        const result = await personal.startPersonalVerification(
                          { certName, certNo, consentNo: consent.consentNo },
                        );
                        clearActionKey("personal-consent");
                        setAccepted(false);
                        data.reload();
                        const url = externalURL(result.item.continuationUrl);
                        if (!url?.startsWith("https:"))
                          throw new Error(
                            "认证已提交，请更新认证结果或联系商家",
                          );
                        setContinuation(url);
                      } finally {
                        certName = "";
                        certNo = "";
                      }
                    })
                  }
                >
                  同意并开始认证
                </Button>
              </p>
            </div>
          )}
          {(continuation || item?.continuationUrl) && (
            <a
              className="primary-link"
              href={
                externalURL(
                  continuation || item?.continuationUrl || "",
                )?.startsWith("https:")
                  ? externalURL(continuation || item?.continuationUrl || "")!
                  : undefined
              }
              target="_blank"
              rel="noopener noreferrer"
            >
              继续完成认证
            </a>
          )}
          {item?.attemptNo && item.statusCode === "verifying" && (
            <Button
              tag="button"
              loading={action.busy}
              onClick={() =>
                void action.run(async () => {
                  await personal.refreshPersonalVerification(item.attemptNo);
                  data.reload();
                })
              }
            >
              更新认证结果
            </Button>
          )}
          {item?.statusCode === "review_required" && (
            <Alert title="请联系商家协助核验，不要在工单中发送完整身份证号。" />
          )}
          <div className="vertical-links">
            <Link to="/account/orders">返回订单继续付款</Link>
            <Link to="/account/support/new">需要帮助</Link>
          </div>
        </FetchState>
      </Card>
      {item?.statusCode === "verified" && <EnterpriseVerification />}
    </Page>
  );
}
