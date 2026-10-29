import PersonalProfile from "./PersonalProfile";
import { countryOptions } from "#/runtime/countries";
import Dialog from "#/adapter/Dialog";
import { useFeature } from "#/runtime/features";
import { useState } from "react";
import {
  Checkbox,
  Button,
  Card,
  Col,
  Form,
  Input,
  Row,
  Select,
  Textarea,
} from "tdesign-react";
import { Link } from "react-router-dom";
import * as account from "#/api/idc/customer-account";
import * as keys from "#/api/idc/ssh-public-keys";
import * as channel from "#/api/idc/customer-sales-channel";
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
  money,
  useAction,
  useCursor,
  useLoad,
} from "#/components/business";

export default function Profile() {
  const compliance = useFeature("idc-compliance");
  const profile = useLoad(
      account.getCustomerProfile,
      [],
      "Account:Profile:profile",
    ),
    contacts = useLoad(
      account.listCustomerContacts,
      [],
      "Account:Profile:contacts",
    ),
    action = useAction();
  const [edit, setEdit] = useState(false);
  const item = profile.data?.item;
  return (
    <Page title="账户资料" description="查看账户身份、客户信息与通知联系人。">
      <PersonalProfile />
      <Row gutter={[20, 20]}>
        <Col xs={12} lg={9}>
          <Card
            className="profile-card"
            title="客户信息"
            actions={
              <Button
                tag="button"
                variant="text"
                disabled={!item}
                onClick={() => {
                  action.clear();
                  setEdit(true);
                }}
              >
                编辑
              </Button>
            }
          >
            <FetchState state={profile}>
              {item && (
                <DetailList
                  items={[
                    ["账户名称", item.displayName],
                    ["客户编号", String(item.customerCode)],
                    ["账户类型", item.type === "enterprise" ? "企业" : "个人"],
                    ["账户状态", <Status value={item.status} />],
                    ["认证状态", <Status value={item.verification} />],
                    ["姓名", item.legalNameMasked || "尚未认证"],
                  ]}
                />
              )}
            </FetchState>
          </Card>
          <Card
            title="通知联系人"
            actions={<Link to="/account/contacts">管理联系人</Link>}
          >
            <FetchState state={contacts}>
              <BusinessTable
                rowKey="contactNo"
                rows={contacts.data?.items || []}
                columns={[
                  { colKey: "displayName", title: "联系人" },
                  { colKey: "email", title: "电子邮箱" },
                  { colKey: "phone", title: "手机号码" },
                  {
                    colKey: "emailVerified",
                    title: "邮箱验证",
                    cell: ({ row }) =>
                      row.emailVerified ? "已验证" : "未验证",
                  },
                ]}
              />
            </FetchState>
          </Card>
        </Col>
        <Col xs={12} lg={3}>
          <Card title="账户服务">
            <div className="vertical-links">
              {compliance && (
                <Link to="/account/verification">完成账户认证</Link>
              )}
              <Link to="/account/addresses">管理联系地址</Link>
              <Link to="/account/ssh-keys">管理登录密钥</Link>
              <Link to="/account/privacy">数据与账户注销</Link>
              <Link to="/account/referrals">推广与佣金</Link>
              <Link to="/account/security">登录方式与密码</Link>
              <Link to="/account/support/new">联系客服</Link>
            </div>
          </Card>
        </Col>
      </Row>
      {edit && item && (
        <Dialog
          visible
          header="编辑账户资料"
          footer={false}
          onClose={() => !action.busy && setEdit(false)}
        >
          <Form
            initialData={item}
            labelAlign="top"
            onSubmit={({ fields, validateResult }) => {
              if (validateResult === true)
                void action.run(async () => {
                  await account.updateCustomerProfile({
                    countryCode: item.countryCode,
                    displayName: String(fields.displayName),
                    expectedVersion: item.version,
                    idempotencyKey: actionKey(`profile-${item.version}`),
                    preferredLocale: "zh-CN",
                  });
                  setEdit(false);
                  profile.reload();
                });
            }}
          >
            <Form.FormItem
              name="displayName"
              label="账户名称"
              rules={[{ required: true, message: "请输入账户名称" }]}
            >
              <Input
                ref={(input) =>
                  input?.inputElement.setAttribute("aria-label", "账户名称")
                }
              />
            </Form.FormItem>
            <ErrorNote error={action.error} />
            <Button tag="button" type="submit" loading={action.busy}>
              保存
            </Button>
          </Form>
        </Dialog>
      )}
    </Page>
  );
}
export function SSHKeys() {
  const data = useLoad(keys.listSSHPublicKeysApi, [], "Account:SSHKeys:data"),
    action = useAction();
  const [create, setCreate] = useState(false),
    [target, setTarget] = useState<keys.SSHPublicKeyItem>(),
    [form] = Form.useForm();
  return (
    <Page
      title="登录密钥"
      description="添加公钥，用于安全连接服务器。请勿上传私钥。"
      actions={
        <Button
          tag="button"
          onClick={() => {
            form.reset();
            action.clear();
            setCreate(true);
          }}
        >
          添加密钥
        </Button>
      }
    >
      <Card>
        <FetchState state={data}>
          <BusinessTable
            rowKey="keyNo"
            rows={data.data?.items || []}
            columns={[
              { colKey: "name", title: "名称" },
              { colKey: "fingerprint", title: "密钥指纹" },
              {
                colKey: "status",
                title: "状态",
                cell: ({ row }) => <Status value={row.status} />,
              },
              {
                colKey: "createdAt",
                title: "添加时间",
                cell: ({ row }) => date(row.createdAt),
              },
              {
                colKey: "actions",
                title: "操作",
                fixed: "right",
                width: 100,
                cell: ({ row }) =>
                  row.status === "active" ? (
                    <Button
                      tag="button"
                      variant="text"
                      theme="danger"
                      onClick={() => setTarget(row)}
                    >
                      停用
                    </Button>
                  ) : (
                    "—"
                  ),
              },
            ]}
          />
        </FetchState>
      </Card>
      {create && (
        <Dialog
          visible
          header="添加登录密钥"
          footer={false}
          onClose={() => !action.busy && setCreate(false)}
        >
          <Form
            form={form}
            labelAlign="top"
            onSubmit={({ fields, validateResult }) => {
              if (validateResult === true)
                void action.run(async () => {
                  await keys.createSSHPublicKeyApi({
                    name: String(fields.name),
                    authorizedKey: String(fields.authorizedKey),
                    requestNo: actionKey("ssh-create"),
                  });
                  clearActionKey("ssh-create");
                  setCreate(false);
                  data.reload();
                });
            }}
          >
            <Form.FormItem
              name="name"
              label="名称"
              rules={[{ required: true, message: "请输入名称" }]}
            >
              <Input />
            </Form.FormItem>
            <Form.FormItem
              name="authorizedKey"
              label="公钥内容"
              rules={[{ required: true, message: "请粘贴公钥内容" }]}
            >
              <Textarea autosize={{ minRows: 5, maxRows: 10 }} />
            </Form.FormItem>
            <ErrorNote error={action.error} />
            <Button tag="button" type="submit" loading={action.busy}>
              添加
            </Button>
          </Form>
        </Dialog>
      )}
      {target && (
        <Confirm
          title="停用密钥"
          busy={action.busy}
          error={action.error}
          onClose={() => setTarget(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              await keys.revokeSSHPublicKeyApi(target.keyNo, {
                expectedVersion: target.version,
                requestNo: actionKey(`revoke-${target.keyNo}`),
              });
              setTarget(undefined);
              data.reload();
            })
          }
        >
          <p>确认停用“{target.name}”？</p>
        </Confirm>
      )}
    </Page>
  );
}
export function Addresses() {
  const data = useLoad(
      account.listCustomerAddresses,
      [],
      "Account:Addresses:data",
    ),
    action = useAction();
  const [target, setTarget] = useState<
      account.CustomerAddress | null | undefined
    >(),
    [archive, setArchive] = useState<account.CustomerAddress>();
  return (
    <Page
      title="联系地址"
      actions={
        <Button
          tag="button"
          onClick={() => {
            action.clear();
            setTarget(null);
          }}
        >
          添加地址
        </Button>
      }
    >
      <Card>
        <FetchState state={data}>
          <BusinessTable
            rowKey="addressNo"
            rows={data.data?.items || []}
            columns={[
              { colKey: "label", title: "名称" },
              { colKey: "recipientName", title: "联系人" },
              { colKey: "recipientPhone", title: "联系电话" },
              {
                colKey: "addressLine1",
                title: "地址",
                cell: ({ row }) =>
                  [
                    row.province,
                    row.city,
                    row.district,
                    row.addressLine1,
                    row.addressLine2,
                  ]
                    .filter(Boolean)
                    .join(" "),
              },
              {
                colKey: "actions",
                title: "操作",
                width: 160,
                fixed: "right",
                cell: ({ row }) =>
                  row.status === "active" ? (
                    <div className="actions">
                      <Button
                        tag="button"
                        variant="text"
                        onClick={() => {
                          action.clear();
                          setTarget(row);
                        }}
                      >
                        编辑
                      </Button>
                      <Button
                        tag="button"
                        variant="text"
                        theme="danger"
                        onClick={() => {
                          action.clear();
                          setArchive(row);
                        }}
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
        </FetchState>
      </Card>
      {target !== undefined && (
        <Dialog
          visible
          header={target ? "编辑地址" : "添加地址"}
          width={640}
          footer={false}
          onClose={() => !action.busy && setTarget(undefined)}
        >
          <Form
            className="address-form"
            initialData={
              target || { countryCode: "CN", type: "contact", default: false }
            }
            labelAlign="top"
            onSubmit={({ fields, validateResult }) => {
              if (validateResult !== true) return;
              void action.run(async () => {
                const input = {
                  ...fields,
                  addressLine2: fields.addressLine2 || "",
                  district: fields.district || "",
                  postalCode: fields.postalCode || "",
                  default: Boolean(fields.default),
                  type: fields.type,
                  idempotencyKey: actionKey(
                    `address-${target?.addressNo || "new"}-${target?.version || 0}`,
                  ),
                } as account.CustomerAddressSaveInput;
                if (target)
                  await account.updateCustomerAddress(target.addressNo, {
                    ...input,
                    expectedVersion: target.version,
                  });
                else await account.createCustomerAddress(input);
                clearActionKey(
                  `address-${target?.addressNo || "new"}-${target?.version || 0}`,
                );
                setTarget(undefined);
                data.reload();
              });
            }}
          >
            <Form.FormItem name="type" label="用途">
              <Select
                options={[
                  { value: "contact", label: "联系地址" },
                  { value: "billing", label: "账单地址" },
                  { value: "registered", label: "注册地址" },
                  { value: "service", label: "服务地址" },
                ]}
              />
            </Form.FormItem>
            <Form.FormItem
              name="countryCode"
              label="国家或地区"
              rules={[{ required: true, message: "请选择国家或地区" }]}
            >
              <Select filterable options={countryOptions} />
            </Form.FormItem>
            <Form.FormItem name="default" className="form-span">
              <Checkbox>设为该用途的默认地址</Checkbox>
            </Form.FormItem>
            {[
              ["label", "名称"],
              ["recipientName", "联系人"],
              ["recipientPhone", "联系电话"],

              ["province", "省份"],
              ["city", "城市"],
              ["district", "区县"],
              ["addressLine1", "详细地址"],
              ["addressLine2", "补充地址"],
              ["postalCode", "邮政编码"],
            ].map(([name, label]) => (
              <Form.FormItem
                key={name}
                className={
                  ["addressLine1", "addressLine2"].includes(name)
                    ? "form-span"
                    : undefined
                }
                name={name}
                label={label}
                rules={
                  ["district", "postalCode", "addressLine2"].includes(name)
                    ? []
                    : [{ required: true, message: `请输入${label}` }]
                }
              >
                <Input
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
          title="移除地址"
          busy={action.busy}
          error={action.error}
          onClose={() => setArchive(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              await account.archiveCustomerAddress(archive.addressNo, {
                expectedVersion: archive.version,
                idempotencyKey: actionKey(
                  `archive-address-${archive.addressNo}`,
                ),
              });
              setArchive(undefined);
              data.reload();
            })
          }
        >
          <p>确认移除“{archive.label}”？</p>
        </Confirm>
      )}
    </Page>
  );
}
export function Referrals() {
  const referral = useLoad(
      channel.getCustomerReferralOverview,
      [],
      "Account:Referrals:referral",
    ),
    cursor = useCursor(),
    commission = useLoad(
      () =>
        channel.getCustomerCommissionOverview({
          cursor: cursor.cursor,
          limit: 20,
        }),
      [cursor.cursor],
      "Account:Referrals:commission",
    );
  return (
    <Page title="推广与佣金" description="查看您的推广成果和佣金记录。">
      <Card title="我的推广">
        <FetchState state={referral}>
          {referral.data && (
            <>
              <DetailList
                items={[
                  ["推荐客户", String(referral.data.referredCustomerCount)],
                  ["有效订单", String(referral.data.qualifiedOrderCount)],
                ]}
              />
              <label className="field">
                推广链接
                <Input readonly value={referral.data.referralUrl} />
              </label>
            </>
          )}
        </FetchState>
      </Card>
      <FetchState state={commission}>
        <div className="overview-grid">
          {commission.data?.accounts.map((v) => (
            <Card key={v.currencyCode} title="可结算佣金">
              <strong className="metric small">
                {money(v.availableAmountMinor, v.currencyCode)}
              </strong>
              <p>待确认 {money(v.pendingAmountMinor, v.currencyCode)}</p>
            </Card>
          ))}
        </div>
        <Card title="佣金记录">
          <BusinessTable
            rowKey="entryNo"
            rows={commission.data?.entries || []}
            columns={[
              { colKey: "entryNo", title: "记录编号" },
              {
                colKey: "amountMinor",
                title: "金额",
                cell: ({ row }) => money(row.amountMinor, row.currencyCode),
              },
              {
                colKey: "statusCode",
                title: "状态",
                cell: ({ row }) => <Status value={row.statusCode} />,
              },
              {
                colKey: "createdAt",
                title: "时间",
                cell: ({ row }) => date(row.createdAt),
              },
            ]}
          />
          <CursorPager
            history={cursor.history}
            next={commission.data?.nextCursor}
            onNext={() => cursor.next(commission.data!.nextCursor)}
            onPrevious={cursor.previous}
          />
        </Card>
      </FetchState>
    </Page>
  );
}
