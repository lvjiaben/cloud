import Dialog from "#/adapter/Dialog";
import { useState } from "react";
import { Button, Card, Form, Input } from "tdesign-react";
import * as api from "#/api/idc/customer-account";
import { actionKey, clearActionKey } from "#/api/request";
import {
  BusinessTable,
  Confirm,
  ErrorNote,
  FetchState,
  Page,
  useLoad,
  useAction,
} from "#/components/business";
export default function Contacts() {
  const data = useLoad(api.listCustomerContacts, [], "Contacts:Contacts:data"),
    action = useAction();
  const [edit, setEdit] = useState<api.CustomerContact | null | undefined>(),
    [archive, setArchive] = useState<api.CustomerContact>();
  const start = (value: api.CustomerContact | null) => {
    action.clear();
    setEdit(value);
  };
  return (
    <Page
      title="通知联系人"
      description="联系人用于接收业务通知，不会获得您的登录或服务操作权限。"
      actions={
        <Button tag="button" onClick={() => start(null)}>
          添加联系人
        </Button>
      }
    >
      <Card>
        <FetchState state={data}>
          <BusinessTable
            rowKey="contactNo"
            rows={data.data?.items || []}
            columns={[
              { colKey: "displayName", title: "姓名" },
              { colKey: "email", title: "邮箱" },
              { colKey: "phone", title: "手机号" },
              {
                colKey: "status",
                title: "状态",
                cell: ({ row }) =>
                  row.status === "active" ? "使用中" : "已移除",
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
                        onClick={() => start(row)}
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
                    "—"
                  ),
              },
            ]}
          />
        </FetchState>
      </Card>
      {edit !== undefined && (
        <Dialog
          visible
          header={edit ? "编辑联系人" : "添加联系人"}
          footer={false}
          onClose={() => !action.busy && setEdit(undefined)}
        >
          <Form
            initialData={
              edit || { phoneCountryCode: "+86", email: "", phone: "" }
            }
            labelAlign="top"
            onSubmit={({ fields, validateResult }) => {
              if (validateResult !== true) return;
              void action.run(async () => {
                const email = String(fields.email || "").trim(),
                  phone = String(fields.phone || "").trim();
                if (!email && !phone)
                  throw new Error("邮箱和手机号至少填写一项");
                const scope = `contact-${edit?.contactNo || "new"}-${edit?.version || 0}`;
                const input = {
                  displayName: String(fields.displayName).trim(),
                  email,
                  phone,
                  phoneCountryCode: phone
                    ? String(fields.phoneCountryCode || "+86")
                    : "",
                  preferredLocale: "zh-CN",
                  idempotencyKey: actionKey(scope),
                };
                if (edit)
                  await api.updateCustomerContact(edit.contactNo, {
                    ...input,
                    expectedVersion: edit.version,
                  });
                else await api.createCustomerContact(input);
                clearActionKey(scope);
                setEdit(undefined);
                data.reload();
              });
            }}
          >
            <Form.FormItem
              name="displayName"
              label="姓名"
              rules={[{ required: true, message: "请输入姓名" }]}
            >
              <Input
                ref={(input) =>
                  input?.inputElement.setAttribute("aria-label", "姓名")
                }
              />
            </Form.FormItem>
            <Form.FormItem
              name="email"
              label="邮箱"
              rules={[{ email: true, message: "请输入正确的邮箱" }]}
            >
              <Input
                ref={(input) =>
                  input?.inputElement.setAttribute("aria-label", "邮箱")
                }
              />
            </Form.FormItem>
            <Form.FormItem name="phoneCountryCode" label="国际区号">
              <Input placeholder="例如 +86" />
            </Form.FormItem>
            <Form.FormItem name="phone" label="手机号">
              <Input
                ref={(input) =>
                  input?.inputElement.setAttribute("aria-label", "手机号")
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
      {archive && (
        <Confirm
          title="移除联系人"
          error={action.error}
          busy={action.busy}
          onClose={() => setArchive(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              await api.archiveCustomerContact(archive.contactNo, {
                expectedVersion: archive.version,
                idempotencyKey: actionKey(
                  `archive-contact-${archive.contactNo}-${archive.version}`,
                ),
              });
              setArchive(undefined);
              data.reload();
            })
          }
        >
          <p>
            移除“{archive.displayName}
            ”后，不再向其发送新的业务通知。历史记录会保留。
          </p>
        </Confirm>
      )}
    </Page>
  );
}
