import { Link } from "react-router-dom";
import {
  SecuredIcon,
  ChevronRightIcon,
  MailIcon,
  MobileIcon,
} from "tdesign-icons-react";
import { maskContact } from "./LoginContacts";
import { useState } from "react";
import { Avatar, Button, Card, Form, Input, Upload } from "tdesign-react";
import { UserIcon } from "tdesign-icons-react";
import Dialog from "#/adapter/Dialog";
import {
  ErrorNote,
  FetchState,
  useAction,
  useLoad,
} from "#/components/business";
import { externalURL, requestClient, safePath } from "#/api/request";
type UserProfile = {
  username: string;
  nickname: string;
  email: string;
  phone: string;
  avatar: string;
};
export default function PersonalProfile() {
  const profile = useLoad(
    () => requestClient.get<UserProfile>("/user/profile"),
    [],
    "PersonalProfile:PersonalProfile:profile",
  );
  const action = useAction();
  const [edit, setEdit] = useState(false),
    [uploadError, setUploadError] = useState("");
  const item = profile.data;
  const updated = () => {
    profile.reload();
    window.dispatchEvent(new Event("customer-profile-updated"));
  };
  return (
    <Card
      title="个人资料"
      className="personal-profile"
      actions={
        <Button
          disabled={!item}
          onClick={() => {
            action.clear();
            setEdit(true);
          }}
        >
          编辑个人资料
        </Button>
      }
    >
      <FetchState state={profile}>
        {item && (
          <div className="personal-profile-content">
            <div className="profile-avatar">
              <Avatar
                size="80px"
                image={item.avatar || undefined}
                icon={<UserIcon />}
              />
              <Upload
                theme="custom"
                accept="image/png,image/jpeg,image/webp"
                max={1}
                showUploadProgress={false}
                fileListDisplay={null}
                trigger={
                  <Button variant="outline" size="small">
                    更换头像
                  </Button>
                }
                beforeUpload={(file) => {
                  const raw = file instanceof File ? file : file.raw;
                  if (
                    !raw ||
                    !["image/png", "image/jpeg", "image/webp"].includes(
                      raw.type,
                    ) ||
                    raw.size > 5 * 1024 * 1024
                  ) {
                    setUploadError(
                      "请选择不超过 5 MB 的 JPG、PNG 或 WebP 图片。",
                    );
                    return false;
                  }
                  setUploadError("");
                  return true;
                }}
                requestMethod={async (file) => {
                  try {
                    const raw = (Array.isArray(file) ? file[0] : file).raw;
                    if (!raw) throw new Error("请选择头像图片。");
                    const data = new FormData();
                    data.append("file", raw);
                    const uploaded = await requestClient.post<{ url: string }>(
                      "/x/idc-core/api/v1/account/customer/avatar-uploads",
                      data,
                    );
                    if (!safePath(uploaded.url) && !externalURL(uploaded.url))
                      throw new Error("头像地址不可用，请重新上传。");
                    await requestClient.put("/user/profile/avatar", {
                      avatar: uploaded.url,
                    });
                    updated();
                    return {
                      status: "success",
                      response: { url: uploaded.url },
                    };
                  } catch (e) {
                    const message =
                      e instanceof Error ? e.message : "头像保存失败，请重试。";
                    setUploadError(message);
                    return { status: "fail", error: message, response: {} };
                  }
                }}
              />
              <small>
                JPG、PNG、WebP
                <br />
                最大 5 MB
              </small>
            </div>
            <div className="profile-identity">
              <div className="profile-name">
                <h2>{item.nickname || item.username}</h2>
                <span>个人登录账户</span>
              </div>
              <p className="profile-username">
                用户名 <span>{item.username}</span>
              </p>
              <div className="profile-contact-summary">
                <span>
                  <MobileIcon />
                  {maskContact(item.phone, "phone") || "暂未绑定手机"}
                </span>
                <span>
                  <MailIcon />
                  {maskContact(item.email, "email") || "暂未绑定邮箱"}
                </span>
              </div>
              <Link className="profile-security-link" to="/account/security">
                <SecuredIcon />
                管理登录方式与账户安全
                <ChevronRightIcon />
              </Link>
            </div>
          </div>
        )}
      </FetchState>
      <ErrorNote error={uploadError} />
      {edit && item && (
        <Dialog
          visible
          header="编辑个人资料"
          footer={false}
          onClose={() => !action.busy && setEdit(false)}
        >
          <Form
            initialData={item}
            labelAlign="top"
            onSubmit={({ fields, validateResult }) => {
              if (validateResult !== true) return;
              void action.run(async () => {
                await requestClient.put("/user/profile", {
                  nickname: String(fields.nickname || "").trim(),
                });
                setEdit(false);
                updated();
              });
            }}
          >
            <Form.FormItem
              name="nickname"
              label="昵称"
              rules={[{ required: true, message: "请输入昵称" }]}
            >
              <Input
                maxlength={64}
                ref={(input) =>
                  input?.inputElement.setAttribute("aria-label", "昵称")
                }
              />
            </Form.FormItem>
            <p className="muted">
              手机号和邮箱用于账户验证，邮箱也可用于找回密码。请前往“登录与安全”验证后更换。
            </p>
            <ErrorNote error={action.error} />
            <Button type="submit" loading={action.busy}>
              保存个人资料
            </Button>
          </Form>
        </Dialog>
      )}
    </Card>
  );
}
