import LoginContacts from "./LoginContacts";
import { SecuredIcon, LockOnIcon } from "tdesign-icons-react";
import { startTransition, useState } from "react";
import { Button, Card, Form, Input } from "tdesign-react";
import { Link, useNavigate } from "react-router-dom";
import { requestClient, saveSession } from "#/api/request";
import {
  Page,
  ErrorNote,
  FetchState,
  useAction,
  useLoad,
} from "#/components/business";
export default function Security() {
  const navigate = useNavigate(),
    action = useAction();
  const user = useLoad(
    () =>
      requestClient.get<{ username: string; phone: string; email: string }>(
        "/user/profile",
      ),
    [],
    "Security:Security:user",
  );
  const [form] = Form.useForm();
  const [saved, setSaved] = useState(false);
  return (
    <Page
      title="登录与安全"
      description="管理联系方式与密码，保护您的云资源和账户资金。"
      actions={<Link to="/account/profile">返回账户资料</Link>}
    >
      <div className="security-overview">
        <span className="security-overview-icon">
          <SecuredIcon size={32} />
        </span>
        <div>
          <h2>账户安全设置</h2>
          <p>
            手机和邮箱是您的身份凭据，更换时需要验证；通知联系人可在账户资料中单独管理。
          </p>
        </div>
      </div>
      <FetchState state={user}>
        {user.data && <LoginContacts user={user.data} onSaved={user.reload} />}
      </FetchState>
      <div className="security-columns">
        <div>
          <Card title="修改登录密码" className="form-card security-card">
            <FetchState state={user}>
              <p className="muted">
                修改后需要重新登录。请选择至少 8
                位的新密码，避免与其他网站相同。
              </p>
              <Form
                form={form}
                labelAlign="top"
                onSubmit={({ fields, validateResult }) => {
                  if (validateResult !== true) return;
                  void action.run(async () => {
                    if (fields.password !== fields.confirmPassword)
                      throw new Error("两次输入的新密码不一致");
                    if (fields.password === fields.currentPassword)
                      throw new Error("新密码不能与当前密码相同");
                    const session = await requestClient.post<{
                      accessToken?: string;
                      refreshToken?: string;
                    }>("/auth/login", {
                      username: user.data!.username,
                      password: fields.currentPassword,
                      clientType: "web",
                    });
                    if (!session.accessToken)
                      throw new Error("当前密码验证未完成，请重新登录后重试");
                    saveSession({
                      accessToken: session.accessToken,
                      refreshToken: session.refreshToken,
                    });
                    await requestClient.put("/user/profile", {
                      password: fields.password,
                    });
                    setSaved(true);
                    form.reset();
                    try {
                      await requestClient.post("/auth/logout");
                    } finally {
                      startTransition(() => {
                        saveSession(null);
                        navigate("/auth/login", {
                          replace: true,
                          state: { message: "密码已更新，请使用新密码登录。" },
                        });
                      });
                    }
                  });
                }}
              >
                {[
                  ["currentPassword", "当前密码"],
                  ["password", "新密码"],
                  ["confirmPassword", "确认新密码"],
                ].map(([name, label]) => (
                  <Form.FormItem
                    key={name}
                    name={name}
                    label={label}
                    rules={[
                      { required: true, message: `请输入${label}` },
                      ...(name === "password"
                        ? [{ min: 8, message: "新密码至少需要 8 个字符" }]
                        : []),
                    ]}
                  >
                    <Input
                      type="password"
                      autocomplete={
                        name === "currentPassword"
                          ? "current-password"
                          : "new-password"
                      }
                      ref={(input) =>
                        input?.inputElement.setAttribute("aria-label", label)
                      }
                    />
                  </Form.FormItem>
                ))}
                <ErrorNote error={action.error} />
                <div className="form-footer">
                  <Button
                    tag="button"
                    type="submit"
                    loading={action.busy}
                    disabled={saved}
                  >
                    保存新密码
                  </Button>
                </div>
              </Form>
            </FetchState>
          </Card>
        </div>
        <aside className="security-guidance">
          <Card title="保护您的账户">
            <div className="guidance-item">
              <LockOnIcon />
              <div>
                <strong>使用独立密码</strong>
                <p>建议同时包含字母、数字和符号，不与其他网站共用。</p>
              </div>
            </div>
            <div className="guidance-item">
              <SecuredIcon />
              <div>
                <strong>验证码只由本人使用</strong>
                <p>请勿向任何人提供登录密码或验证码，客服不会索取您的密码。</p>
              </div>
            </div>
            <Link to="/account/support/new">联系方式无法使用？提交工单</Link>
          </Card>
        </aside>
      </div>
    </Page>
  );
}
