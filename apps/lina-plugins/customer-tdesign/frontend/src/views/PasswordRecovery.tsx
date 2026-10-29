// Password recovery keeps contact verification and password saving in one clear flow.
import { useEffect, useState } from "react";
import { Alert, Button, Form, Input, Radio, Steps } from "tdesign-react";
import { actionKey, clearActionKey } from "#/api/request";
import { ErrorNote, useAction } from "#/components/business";
import * as auth from "#/api/idc/customer-auth";
import type {
  HumanChallenge,
  VerificationCode,
} from "#/api/idc/customer-onboarding";

export default function PasswordRecovery({ enabled }: { enabled: boolean }) {
  const [form] = Form.useForm();
  const action = useAction();
  const [kind, setKind] = useState<"email" | "phone">("email");
  const [binding, setBinding] = useState<auth.RecoveryBinding>();
  const [challenge, setChallenge] = useState<HumanChallenge>();
  const [verification, setVerification] = useState<VerificationCode>();
  const [result, setResult] = useState<{
    cleanupPending: boolean;
    cleanupToken: string;
  }>();
  const [now, setNow] = useState(Date.now());
  const step = verification ? 2 : challenge ? 1 : 0;
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  const expired = !!verification && now >= verification.expiresAt;
  const retryAfter = Math.max(
    0,
    Math.ceil(((verification?.retryAfterAt || 0) - now) / 1000),
  );
  const contactLabel = kind === "email" ? "电子邮箱" : "手机号";
  async function submit(values: Record<string, string>) {
    await action.run(async () => {
      if (!enabled) throw new Error("当前暂未开放找回密码，请联系网站客服");
      if (!binding) {
        const next: auth.RecoveryBinding = {
          kind,
          account: values.account.trim(),
          browserNonce: crypto.randomUUID(),
          locale: "zh-CN",
        };
        const nextChallenge = await auth.createRecoveryChallenge(next);
        setBinding(next);
        setChallenge(nextChallenge);
        return;
      }
      if (!verification && challenge) {
        const proof = await auth.completeRecoveryChallenge({
          ...binding,
          challengeNo: challenge.challengeNo,
          answer: values.answer,
        });
        let sent: VerificationCode;
        try {
          sent = await auth.sendRecoveryCode({
            ...binding,
            proofToken: proof.proofToken,
            idempotencyKey: actionKey(`recovery-${binding.browserNonce}`),
          });
        } catch (error) {
          setChallenge(await auth.createRecoveryChallenge(binding));
          form.reset({ fields: ["answer"] });
          throw error;
        }
        if (
          !["accepted", "delivered", "result_unknown"].includes(
            sent.deliveryState,
          )
        ) {
          restart();
          throw new Error("验证码暂未发送成功，请重新获取");
        }
        setNow(Date.now());
        setVerification(sent);
        return;
      }
      if (!verification) return;
      if (expired) throw new Error("验证码已过期，请重新获取");
      if (!/^\d{6}$/u.test(values.code || ""))
        throw new Error("请输入六位验证码");
      if (values.password.length < 8 || values.password.length > 32)
        throw new Error("密码需要 8 到 32 个字符");
      if (values.password !== values.confirmPassword)
        throw new Error("两次输入的新密码不一致");
      const saved = await auth.resetPassword({
        ...binding,
        verificationNo: verification.verificationNo,
        code: values.code,
        password: values.password,
      });
      if (!saved.reset) throw new Error("密码尚未保存，请重试");
      setResult(saved);
      form.reset();
      clearActionKey(`recovery-${binding.browserNonce}`);
    });
  }
  function restart() {
    setBinding(undefined);
    setChallenge(undefined);
    setVerification(undefined);
    form.reset();
    action.clear();
  }
  return (
    <>
      {result ? (
        <div className="recovery-result">
          <Alert
            theme="success"
            title="新密码已保存"
            message="请返回登录，使用新密码进入用户中心。"
          />
          {result.cleanupPending && (
            <>
              <Alert
                theme="warning"
                message="旧登录已停止使用，部分退出操作暂未完成。您可以重试完成退出。"
              />
              <Button
                loading={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    if (!binding || !verification) return;
                    const cleaned = await auth.retryRecoveryCleanup({
                      ...binding,
                      verificationNo: verification.verificationNo,
                      cleanupToken: result.cleanupToken,
                    });
                    setResult({ ...result, cleanupPending: cleaned.pending });
                    if (cleaned.pending)
                      throw new Error("暂时未能全部退出，请稍后再试");
                  })
                }
              >
                重试退出旧登录
              </Button>
            </>
          )}
          <ErrorNote error={action.error} />
        </div>
      ) : (
        <>
          <Steps current={step} className="auth-steps">
            <Steps.StepItem title="确认账户" />
            <Steps.StepItem title="安全验证" />
            <Steps.StepItem title="设置密码" />
          </Steps>
          <Form
            form={form}
            labelAlign="top"
            onSubmit={({ validateResult, fields }) => {
              if (validateResult === true)
                void submit(fields as Record<string, string>);
            }}
          >
            {step === 0 && (
              <>
                <Radio.Group
                  theme="button"
                  className="auth-methods"
                  variant="default-filled"
                  value={kind}
                  onChange={(value) => {
                    setKind(value as "email" | "phone");
                    form.reset();
                    action.clear();
                  }}
                  options={[
                    { label: "通过邮箱找回", value: "email" },
                    { label: "通过手机找回", value: "phone" },
                  ]}
                />
                <Form.FormItem
                  name="account"
                  label={contactLabel}
                  rules={[
                    {
                      required: true,
                      message: `请输入账户已绑定的${contactLabel}`,
                    },
                  ]}
                >
                  <Input
                    size="large"
                    autocomplete={kind === "email" ? "email" : "tel"}
                    placeholder={`请输入已绑定的${contactLabel}`}
                    ref={(input) =>
                      input?.inputElement.setAttribute(
                        "aria-label",
                        contactLabel,
                      )
                    }
                  />
                </Form.FormItem>
              </>
            )}
            {step === 1 && (
              <Form.FormItem
                name="answer"
                label={challenge?.accessiblePrompt || "请完成安全验证"}
                rules={[{ required: true, message: "请输入验证答案" }]}
              >
                <Input
                  size="large"
                  ref={(input) =>
                    input?.inputElement.setAttribute(
                      "aria-label",
                      "人机验证答案",
                    )
                  }
                />
              </Form.FormItem>
            )}
            {step === 2 && (
              <>
                <Alert
                  theme={expired ? "warning" : "info"}
                  message={
                    expired
                      ? "验证码已过期，请重新获取"
                      : `验证码已发往 ${verification?.maskedDestination}，10 分钟内有效。`
                  }
                />
                <Form.FormItem
                  name="code"
                  label="验证码"
                  rules={[{ required: true, message: "请输入验证码" }]}
                >
                  <Input
                    size="large"
                    autocomplete="one-time-code"
                    maxlength={6}
                    ref={(input) =>
                      input?.inputElement.setAttribute("aria-label", "验证码")
                    }
                  />
                </Form.FormItem>
                <Form.FormItem
                  name="password"
                  label="新密码"
                  rules={[{ required: true, message: "请输入新密码" }]}
                >
                  <Input
                    size="large"
                    type="password"
                    autocomplete="new-password"
                    maxlength={32}
                    ref={(input) =>
                      input?.inputElement.setAttribute("aria-label", "新密码")
                    }
                  />
                </Form.FormItem>
                <Form.FormItem
                  name="confirmPassword"
                  label="再次输入新密码"
                  rules={[{ required: true, message: "请再次输入新密码" }]}
                >
                  <Input
                    size="large"
                    type="password"
                    autocomplete="new-password"
                    maxlength={32}
                    ref={(input) =>
                      input?.inputElement.setAttribute(
                        "aria-label",
                        "再次输入新密码",
                      )
                    }
                  />
                </Form.FormItem>
              </>
            )}
            <ErrorNote error={action.error} />
            <Button
              type="submit"
              block
              size="large"
              loading={action.busy}
              disabled={!enabled || expired}
            >
              {step === 0 ? "下一步" : step === 1 ? "获取验证码" : "保存新密码"}
            </Button>
            {step > 0 && (
              <Button
                variant="text"
                disabled={
                  action.busy || (step === 2 && retryAfter > 0 && !expired)
                }
                onClick={restart}
              >
                {step === 2
                  ? retryAfter > 0 && !expired
                    ? `${retryAfter} 秒后可重新获取`
                    : "重新获取验证码"
                  : "重新填写账户"}
              </Button>
            )}
          </Form>
          <p className="muted">
            原邮箱或手机号已无法使用，请联系网站客服提交工单。
          </p>
        </>
      )}
    </>
  );
}
