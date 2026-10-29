import PasswordRecovery from "./PasswordRecovery";
import * as customerAuth from "#/api/idc/customer-auth";
import Dialog from "#/adapter/Dialog";
import { useState, useEffect } from "react";
import { Button, Card, Checkbox, Form, Input, Steps } from "tdesign-react";
import {
  CloudIcon,
  LockOnIcon,
  WalletIcon,
  ServiceIcon,
} from "tdesign-icons-react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  requestClient,
  safePath,
  saveSession,
  actionKey,
  clearActionKey,
} from "#/api/request";
import * as onboarding from "#/api/idc/customer-onboarding";
import { ErrorNote, useAction, useLoad } from "#/components/business";
export default function Auth() {
  const navigate = useNavigate(),
    location = useLocation(),
    action = useAction();
  const [form] = Form.useForm();
  const settings = useLoad(
    () =>
      requestClient.get<{
        auth: {
          registerEnabled: boolean;
          forgetPasswordEnabled: boolean;
          privacyPolicy: string;
          termsOfService: string;
        };
      }>("/config/public/frontend"),
    [],
  );
  const brand = useLoad(
    () =>
      requestClient.get<{ common?: { siteName?: string; logoUrl?: string } }>(
        "/x/idc-portal/api/v1/public/customer-center/configuration",
      ),
    [],
  );
  const [policy, setPolicy] = useState<"privacyPolicy" | "termsOfService">();
  useEffect(() => {
    form.reset();
    setStep(0);
    setBinding(undefined);
    setConsent(false);
    setSuccess(location.state?.message || "");
    action.clear();
  }, [location.pathname]);
  const mode = location.pathname.split("/").at(-1);
  const initialPassword = mode === "initial-password";
  const register = mode === "register",
    forgot = mode === "forget-password",
    reset = mode === "reset-password";
  const [step, setStep] = useState(0),
    [binding, setBinding] = useState<onboarding.OnboardingBinding>(),
    [challenge, setChallenge] = useState<onboarding.HumanChallenge>(),
    [verification, setVerification] = useState<onboarding.VerificationCode>(),
    [consent, setConsent] = useState(false),
    [success, setSuccess] = useState(location.state?.message || "");
  const returnPath =
    safePath(
      new URLSearchParams(location.search).get("redirect") || "/account",
    ) || "/account";
  const title = initialPassword
    ? "修改初始密码"
    : register
      ? "创建账户"
      : forgot
        ? "找回密码"
        : reset
          ? "设置新密码"
          : "登录用户中心";
  async function submit(values: Record<string, string>) {
    await action.run(async () => {
      if (
        (register && !settings.data?.auth.registerEnabled) ||
        ((forgot || reset) && !settings.data?.auth.forgetPasswordEnabled)
      )
        throw new Error("当前暂未开放此功能");
      if (initialPassword) {
        if (values.password !== values.confirmPassword)
          throw new Error("两次密码不一致");
        if (values.password.length < 8)
          throw new Error("密码至少需要 8 个字符");
        await requestClient.put("/user/profile", { password: values.password });
        await requestClient.put("/x/idc-core/api/v1/account/initial-password", {
          newPassword: values.password,
        });
        navigate(returnPath, { replace: true });
        return;
      }
      if (reset) {
        if (values.password !== values.confirmPassword)
          throw new Error("两次输入的新密码不一致");
        if (values.password.length < 8)
          throw new Error("密码至少需要 8 个字符");
        if (!new URLSearchParams(location.search).get("token"))
          throw new Error("重置链接不完整，请从邮件中重新打开");
        await requestClient.post("/auth/reset-password", {
          password: values.password,
          token: new URLSearchParams(location.search).get("token"),
        });
        navigate("/auth/login");
        return;
      }
      if (register) {
        if (!consent) throw new Error("请先确认注册及验证联系方式");
        if (step === 0) {
          if (values.password.length < 8)
            throw new Error("密码至少需要 8 个字符");
          if (values.password !== values.confirmPassword)
            throw new Error("两次输入的密码不一致");
          const next: onboarding.OnboardingBinding = {
            browserNonce: crypto.randomUUID(),
            customerType: "individual",
            destination: values.email,
            destinationKind: "email",
            locale: "zh-CN",
            returnPath,
            username: values.username.trim(),
          };
          setChallenge(await onboarding.createOnboardingHumanChallenge(next));
          setBinding(next);
          setStep(1);
          return;
        }
        if (step === 1 && binding && challenge) {
          const proof = await onboarding.completeOnboardingHumanChallenge({
            ...binding,
            answer: values.answer,
            challengeNo: challenge.challengeNo,
          });
          setVerification(
            await onboarding.sendOnboardingVerificationCode({
              ...binding,
              proofToken: proof.proofToken,
              idempotencyKey: actionKey(
                `onboarding-code-${binding.browserNonce}`,
              ),
            }),
          );
          setStep(2);
          return;
        }
        if (
          step === 2 &&
          binding &&
          verification &&
          !sessionStorage.getItem("customer-tdesign.onboarding")
        ) {
          const intent = await onboarding.createVerifiedOnboardingIntent({
            ...binding,
            code: values.code,
            verificationNo: verification.verificationNo,
            idempotencyKey: actionKey(
              `onboarding-intent-${binding.browserNonce}`,
            ),
          });
          const account = await requestClient.post<{ userId: number }>(
            "/auth/register",
            {
              email: binding.destination,
              username: binding.username,
              password: values.password,
              nickname: binding.username,
            },
          );
          sessionStorage.setItem(
            "customer-tdesign.onboarding",
            JSON.stringify({
              token: intent.token,
              browserNonce: binding.browserNonce,
              hostRegistrationUserId: account.userId,
              deviceId: onboarding.getOrCreateRegistrationDeviceID(),
            }),
          );
        }
      }
      const session = await customerAuth.passwordLogin(
        values.username,
        values.password,
        register ? "username" : undefined,
      );
      if (!session.accessToken)
        throw new Error("该账号暂时不能进入用户中心，请联系商家");
      saveSession({
        accessToken: session.accessToken,
        refreshToken: session.refreshToken,
      });
      const pending = sessionStorage.getItem("customer-tdesign.onboarding");
      if (pending) {
        await onboarding.redeemCustomerOnboarding(JSON.parse(pending));
        sessionStorage.removeItem("customer-tdesign.onboarding");
        clearActionKey(`onboarding-code-${JSON.parse(pending).browserNonce}`);
        clearActionKey(`onboarding-intent-${JSON.parse(pending).browserNonce}`);
      }
      navigate(returnPath, { replace: true });
    });
  }
  return (
    <div className="auth-page">
      <header className="auth-brand">
        <span className="brand-symbol">
          <CloudIcon size={24} />
        </span>
        <strong>{brand.data?.common?.siteName || "用户中心"}</strong>
      </header>
      <div className="auth-intro">
        <span className="auth-eyebrow">您的云服务工作空间</span>
        <h1>
          每一项服务，
          <br />
          都在掌握之中。
        </h1>
        <p>从选购到续费，让日常管理更简单。</p>
        <div className="auth-features">
          <span>
            <CloudIcon />
            管理已购服务
          </span>
          <span>
            <WalletIcon />
            查看账单与费用
          </span>
          <span>
            <ServiceIcon />
            随时获取帮助
          </span>
        </div>
        <div className="auth-art" aria-hidden="true">
          <div className="auth-art-cloud">
            <CloudIcon size={76} />
          </div>
          <div className="auth-art-line" />
          <div className="auth-art-nodes">
            <span>
              <LockOnIcon />
            </span>
            <span>
              <WalletIcon />
            </span>
            <span>
              <ServiceIcon />
            </span>
          </div>
        </div>
      </div>
      <Card className="auth-card" bordered>
        <h1>{title}</h1>
        <p className="muted">
          {initialPassword
            ? "首次登录，请把初始密码更换为您自己的密码。"
            : forgot
              ? "验证账户联系方式，安全找回密码。"
              : "欢迎使用您的服务控制台"}
        </p>
        {register && (
          <Steps current={step} className="auth-steps">
            <Steps.StepItem title="账户" />
            <Steps.StepItem title="验证" />
            <Steps.StepItem title="完成" />
          </Steps>
        )}
        {forgot ? (
          <PasswordRecovery
            enabled={!!settings.data?.auth.forgetPasswordEnabled}
          />
        ) : (
          <Form
            form={form}
            labelAlign="top"
            onSubmit={({ validateResult, fields }) => {
              if (validateResult === true)
                void submit(fields as Record<string, string>);
            }}
          >
            {!forgot && !reset && !initialPassword && (
              <Form.FormItem
                name="username"
                label={register ? "用户名" : "账号"}
                rules={[
                  {
                    required: true,
                    message: register
                      ? "请输入用户名"
                      : "请输入用户名、邮箱或手机号",
                  },
                  ...(register
                    ? [
                        {
                          validator: (value: string) =>
                            customerAuth.usernameAllowed(value),
                          message: "用户名不能是纯数字、手机号或邮箱",
                        },
                      ]
                    : []),
                ]}
              >
                <Input
                  size="large"
                  ref={(input) =>
                    input?.inputElement.setAttribute(
                      "aria-label",
                      register ? "用户名" : "账号",
                    )
                  }
                  placeholder={
                    register ? "请输入用户名" : "用户名 / 邮箱 / 手机号"
                  }
                  disabled={step > 0}
                  autocomplete="username"
                />
              </Form.FormItem>
            )}
            {(register || forgot) && (
              <Form.FormItem
                name="email"
                label="电子邮箱"
                rules={[
                  { required: true, message: "请输入电子邮箱" },
                  { email: true, message: "请输入正确的邮箱" },
                ]}
              >
                <Input
                  size="large"
                  ref={(input) =>
                    input?.inputElement.setAttribute("aria-label", "电子邮箱")
                  }
                  disabled={step > 0}
                />
              </Form.FormItem>
            )}
            {!forgot && (
              <Form.FormItem
                name="password"
                label="密码"
                rules={[{ required: true, message: "请输入密码" }]}
              >
                <Input
                  size="large"
                  ref={(input) =>
                    input?.inputElement.setAttribute("aria-label", "密码")
                  }
                  type="password"
                  autocomplete={
                    register || reset || initialPassword
                      ? "new-password"
                      : "current-password"
                  }
                />
              </Form.FormItem>
            )}
            {(initialPassword || reset || (register && step === 0)) && (
              <Form.FormItem
                name="confirmPassword"
                label="再次输入新密码"
                rules={[{ required: true, message: "请再次输入新密码" }]}
              >
                <Input
                  size="large"
                  ref={(input) =>
                    input?.inputElement.setAttribute(
                      "aria-label",
                      "再次输入新密码",
                    )
                  }
                  type="password"
                  autocomplete="new-password"
                />
              </Form.FormItem>
            )}
            {register && step === 1 && (
              <Form.FormItem
                name="answer"
                label={challenge?.accessiblePrompt || "请完成验证"}
                rules={[{ required: true, message: "请输入答案" }]}
              >
                <Input
                  ref={(input) =>
                    input?.inputElement.setAttribute(
                      "aria-label",
                      "人机验证答案",
                    )
                  }
                />
              </Form.FormItem>
            )}
            {register && step === 2 && (
              <Form.FormItem
                name="code"
                label={`邮箱验证码（${verification?.maskedDestination || ""}）`}
                rules={[{ required: true, message: "请输入验证码" }]}
              >
                <Input
                  autocomplete="one-time-code"
                  ref={(input) =>
                    input?.inputElement.setAttribute("aria-label", "邮箱验证码")
                  }
                />
              </Form.FormItem>
            )}
            {register && (
              <Checkbox checked={consent} onChange={setConsent}>
                我使用本人邮箱注册，并同意服务条款及隐私政策
              </Checkbox>
            )}
            <ErrorNote error={action.error} />
            {success && <p role="status">{success}</p>}
            <Button
              tag="button"
              size="large"
              type="submit"
              block
              loading={action.busy}
            >
              {register ? (step === 2 ? "完成注册" : "下一步") : title}
            </Button>
            {register &&
              step > 0 &&
              !sessionStorage.getItem("customer-tdesign.onboarding") && (
                <Button
                  tag="button"
                  variant="text"
                  disabled={action.busy}
                  onClick={() => {
                    setStep(0);
                    setBinding(undefined);
                    setChallenge(undefined);
                    setVerification(undefined);
                    action.clear();
                  }}
                >
                  重新填写账户信息
                </Button>
              )}
          </Form>
        )}
        {register && (
          <div className="auth-links">
            <Button
              tag="button"
              variant="text"
              onClick={() => setPolicy("termsOfService")}
            >
              服务条款
            </Button>
            <Button
              tag="button"
              variant="text"
              onClick={() => setPolicy("privacyPolicy")}
            >
              隐私政策
            </Button>
          </div>
        )}
        {policy && (
          <Dialog
            visible
            header={policy === "privacyPolicy" ? "隐私政策" : "服务条款"}
            footer={false}
            onClose={() => setPolicy(undefined)}
          >
            <div style={{ whiteSpace: "pre-wrap" }}>
              {settings.data?.auth[policy] || "暂时无法加载，请稍后重试"}
            </div>
          </Dialog>
        )}
        <div className="auth-links">
          {(register ||
            forgot ||
            reset ||
            settings.data?.auth.registerEnabled) && (
            <Link
              to={
                register || forgot || reset ? "/auth/login" : "/auth/register"
              }
            >
              {register || forgot || reset ? "返回登录" : "注册账户"}
            </Link>
          )}
          {!forgot && settings.data?.auth.forgetPasswordEnabled && (
            <Link to="/auth/forget-password">忘记密码</Link>
          )}
        </div>
      </Card>
    </div>
  );
}
