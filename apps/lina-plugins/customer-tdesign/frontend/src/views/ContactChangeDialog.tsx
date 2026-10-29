// A short, fully automatic old-contact → new-contact verification flow.
import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form, Input, Steps } from "tdesign-react";
import { Link } from "react-router-dom";
import { CheckCircleFilledIcon } from "tdesign-icons-react";
import Dialog from "#/adapter/Dialog";
import { ApiError, pluginApiPath, requestClient } from "#/api/request";
import { ErrorNote, useAction } from "#/components/business";
import { maskContact } from "./LoginContacts";

type Kind = "email" | "phone";
type CodeDelivery = {
  verificationNo: string;
  maskedDestination: string;
  deliveryState: string;
  expiresAt: number;
  retryAfterAt: number;
};
const codesPath = pluginApiPath("idc-notify", "account/security/contact-codes");

export default function ContactChangeDialog({
  kind,
  user,
  onClose,
  onSaved,
}: {
  kind: Kind;
  user: { email?: string; phone?: string };
  onClose: () => void;
  onSaved: () => void;
}) {
  const action = useAction();
  const initiallyBound = useRef(Boolean(user[kind])).current;
  const [step, setStep] = useState(0);
  const [oldProof, setOldProof] = useState<CodeDelivery>();
  const [newProof, setNewProof] = useState<CodeDelivery>();
  const [destination, setDestination] = useState("");
  const [code, setCode] = useState("");
  const [retryAt, setRetryAt] = useState(0);
  const [now, setNow] = useState(Date.now());
  const sendKey = useRef<string>();
  const originalKind: Kind = user[kind]
    ? kind
    : kind === "email"
      ? "phone"
      : "email";
  const original = user[originalKind];
  const title = kind === "email" ? "电子邮箱" : "手机号码";
  const proof = step === 0 ? oldProof : newProof;
  const seconds = Math.max(0, Math.ceil((retryAt - now) / 1000));
  const binding = {
    kind,
    locale: "zh-CN",
    ...(step === 1
      ? {
          oldVerification: oldProof?.verificationNo,
          destination: destination.trim(),
        }
      : {}),
  };
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  function send() {
    void action.run(async () => {
      if (step === 1 && !destination.trim())
        throw new Error(`请填写新的${title}`);
      sendKey.current ||= crypto.randomUUID();
      const result = await requestClient
        .post<CodeDelivery>(codesPath, {
          ...binding,
          idempotencyKey: sendKey.current,
        })
        .catch((error: unknown) => {
          if (
            error instanceof ApiError &&
            (error.response.data as { errorCode?: string } | undefined)
              ?.errorCode === "IDC_NOTIFY_CHANNEL_UNAVAILABLE"
          )
            throw new Error("验证码服务暂未开通，请联系网站客服");
          throw error;
        });
      sendKey.current = undefined;
      setRetryAt(result.retryAfterAt);
      setNow(Date.now());
      if (
        !["accepted", "delivered", "result_unknown"].includes(
          result.deliveryState,
        )
      )
        throw new Error("验证码未发送成功，请稍后重新获取");
      if (step === 0) setOldProof(result);
      else setNewProof(result);
      setCode("");
    });
  }
  function verify() {
    void action.run(async () => {
      if (!proof) throw new Error("请先获取验证码");
      if (!/^[0-9]{6}$/.test(code)) throw new Error("请输入收到的 6 位验证码");
      await requestClient.post(
        `${codesPath}/${encodeURIComponent(proof.verificationNo)}/verification`,
        { ...binding, code },
      );
      if (step === 0) {
        setStep(1);
        setCode("");
        setRetryAt(0);
        sendKey.current = undefined;
        return;
      }
      await requestClient.put(
        "/user/profile",
        { [kind]: destination.trim() },
        {
          headers: {
            "X-Contact-Verification": `${oldProof!.verificationNo}.${proof.verificationNo}`,
          },
        },
      );
      setStep(2);
      setCode("");
      onSaved();
    });
  }
  return (
    <Dialog
      visible
      header={`${initiallyBound ? "更换" : "绑定"}${title}`}
      footer={false}
      onClose={() => {
        if (!action.busy) onClose();
      }}
      closeBtn={!action.busy}
    >
      <Steps current={step} readonly className="contact-steps">
        <Steps.StepItem title="验证身份" />
        <Steps.StepItem title={`设置新${kind === "email" ? "邮箱" : "手机"}`} />
        <Steps.StepItem title="完成" />
      </Steps>
      {step === 2 ? (
        <div className="contact-change-success">
          <CheckCircleFilledIcon size={48} />
          <h3>{title}已更新</h3>
          <p>{maskContact(destination, kind)}</p>
          <p className="muted">以后请使用新的联系方式进行账户验证。</p>
          <Button tag="button" onClick={onClose}>
            完成
          </Button>
        </div>
      ) : !original ? (
        <>
          <Alert
            theme="info"
            title="当前没有可验证的联系方式"
            message="请提交工单，由客服核验身份后协助绑定。"
          />
          <div className="dialog-footer">
            <Button tag="button" variant="outline" onClick={onClose}>
              关闭
            </Button>
            <Link to="/account/support/new">
              <Button tag="button">提交工单</Button>
            </Link>
          </div>
        </>
      ) : (
        <>
          <p className="contact-flow-note">
            {step === 0
              ? `为确认是您本人操作，请验证${user[kind] ? "原" : "已绑定的"}${originalKind === "email" ? "邮箱" : "手机号码"}。`
              : "身份验证已通过，请填写并验证新的联系方式。"}
          </p>
          <Form
            labelAlign="top"
            onSubmit={({ e }) => {
              e?.preventDefault();
              verify();
            }}
          >
            {step === 0 ? (
              <div className="contact-current-value">
                <span>
                  {originalKind === "email" ? "原邮箱" : "原手机号码"}
                </span>
                <strong>
                  {oldProof?.maskedDestination ||
                    maskContact(original, originalKind)}
                </strong>
              </div>
            ) : (
              <Form.FormItem label={`新${title}`}>
                <Input
                  aria-label={`新${title}`}
                  value={destination}
                  disabled={action.busy}
                  placeholder={
                    kind === "email"
                      ? "请输入新邮箱"
                      : "请输入手机号，国际号码需带国家区号"
                  }
                  onChange={(value) => {
                    setDestination(value);
                    setNewProof(undefined);
                    setCode("");
                    sendKey.current = undefined;
                  }}
                />
              </Form.FormItem>
            )}
            <Form.FormItem label="验证码">
              <div className="contact-code-row">
                <Input
                  aria-label="验证码"
                  value={code}
                  maxlength={6}
                  autocomplete="one-time-code"
                  disabled={action.busy}
                  placeholder="6 位验证码"
                  onChange={setCode}
                />
                <Button
                  tag="button"
                  variant="outline"
                  disabled={action.busy || seconds > 0}
                  onClick={send}
                >
                  {seconds > 0
                    ? `${seconds} 秒后重发`
                    : proof
                      ? "重新获取"
                      : "获取验证码"}
                </Button>
              </div>
            </Form.FormItem>
            {proof && (
              <p className="muted" role="status">
                {proof.deliveryState === "result_unknown"
                  ? "发送结果暂未确认，请检查"
                  : "验证码已发送至"}{" "}
                {proof.maskedDestination}，10 分钟内有效。
              </p>
            )}
            <ErrorNote error={action.error} />
            <div className="dialog-footer">
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
                type="submit"
                loading={action.busy}
                disabled={!proof}
              >
                {step === 0 ? "验证并继续" : "确认更换"}
              </Button>
            </div>
          </Form>
          <p className="contact-support-link">
            原联系方式无法接收验证码？
            <Link to="/account/support/new">提交工单</Link>
          </p>
        </>
      )}
    </Dialog>
  );
}
