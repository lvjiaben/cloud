import { useState } from "react";
import { Button, Card, Tag } from "tdesign-react";
import { MailIcon, MobileIcon, SecuredIcon } from "tdesign-icons-react";
import ContactChangeDialog from "./ContactChangeDialog";

export function maskContact(
  value: string | undefined,
  kind: "phone" | "email",
) {
  if (!value) return "";
  if (kind === "email") {
    const [name, domain] = value.split("@");
    return domain ? `${name.slice(0, 2)}***@${domain}` : "已绑定邮箱";
  }
  return value.length > 6
    ? `${value.slice(0, 3)} **** ${value.slice(-4)}`
    : "已绑定手机";
}
export default function LoginContacts({
  user,
  onSaved,
}: {
  user: { email?: string; phone?: string };
  onSaved: () => void;
}) {
  const [target, setTarget] = useState<"phone" | "email">();
  return (
    <>
      <Card
        title="账户联系方式"
        className="login-contacts"
        subtitle="用于找回密码和验证账户身份"
      >
        {(["phone", "email"] as const).map((kind) => {
          const value = user[kind],
            title = kind === "phone" ? "手机号码" : "电子邮箱";
          const Icon = kind === "phone" ? MobileIcon : MailIcon;
          return (
            <div className="security-setting" key={kind}>
              <span className="security-setting-icon">
                <Icon size={24} />
              </span>
              <div className="security-setting-main">
                <h3>
                  {title}
                  <Tag
                    size="small"
                    variant="light"
                    theme={value ? "default" : "warning"}
                  >
                    {value ? "已绑定" : "未绑定"}
                  </Tag>
                </h3>
                <p>
                  {maskContact(value, kind) ||
                    (kind === "phone"
                      ? "绑定手机后，可用于账户身份验证"
                      : "绑定邮箱后，可接收找回密码邮件")}
                </p>
              </div>
              <Button
                variant="text"
                theme="primary"
                onClick={() => setTarget(kind)}
              >
                {value ? "更换" : "绑定"}
                {title}
              </Button>
            </div>
          );
        })}
        <div className="security-setting-note">
          <SecuredIcon />
          更换联系方式需要验证原联系方式和新联系方式。
        </div>
      </Card>
      {target && (
        <ContactChangeDialog
          kind={target}
          user={user}
          onClose={() => setTarget(undefined)}
          onSaved={onSaved}
        />
      )}
    </>
  );
}
