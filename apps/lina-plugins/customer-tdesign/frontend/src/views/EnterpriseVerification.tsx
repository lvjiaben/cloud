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
import * as api from "#/api/idc/enterprise-verification";
import { actionKey, clearActionKey } from "#/api/request";
import {
  DetailList,
  ErrorNote,
  FetchState,
  Status,
  useAction,
  useLoad,
} from "#/components/business";
const materialLabels: Record<string, string> = {
  business_license: "营业执照",
  legal_representative_id: "法定代表人身份证明",
  agent_id: "经办人身份证明",
  authorization_letter: "授权委托书",
};
export default function EnterpriseVerification() {
  const data = useLoad(api.getEnterpriseVerification, []),
    action = useAction(),
    [form] = Form.useForm();
  const [accepted, setAccepted] = useState(false),
    [role, setRole] = useState<"legal_representative" | "authorized_agent">(
      "legal_representative",
    ),
    [change, setChange] = useState(false),
    [changeFields, setChangeFields] = useState<string[]>([]),
    [reason, setReason] = useState("");
  const state = data.data,
    verified = state?.profile?.verificationStatusCode === "verified",
    draft = state?.case?.statusCode === "draft",
    waiting = ["pending", "review_required", "verifying"].includes(
      state?.case?.statusCode || "",
    );
  const types =
    role === "authorized_agent"
      ? Object.keys(materialLabels)
      : ["business_license", "legal_representative_id"];
  const canStart =
    state && !draft && !waiting && (!verified || !!state.activeLegalChange);
  const ready = types.every((t) =>
    state?.case?.materials.some(
      (m) => m.materialTypeCode === t && m.statusCode === "available",
    ),
  );
  return (
    <Card
      title="企业认证"
      actions={
        <Button tag="button" variant="text" onClick={data.reload}>
          刷新进度
        </Button>
      }
    >
      <FetchState state={data}>
        <ErrorNote error={action.error} />
        {state?.profile && (
          <DetailList
            items={[
              ["当前企业名称", state.profile.legalName],
              ["登记号码", state.profile.registrationIdentifierMasked],
              [
                "当前有效状态",
                <Status value={state.profile.verificationStatusCode} />,
              ],
            ]}
          />
        )}
        <p>
          本次申请：
          <Status value={state?.case?.statusCode || "unverified"} />
        </p>
        {waiting && (
          <Alert title="材料正在审核，结果更新后可在这里查看。当前已认证资料继续有效。" />
        )}
        {verified && !state?.activeLegalChange && !waiting && (
          <Button
            tag="button"
            variant="outline"
            onClick={() => setChange((v) => !v)}
          >
            变更企业资料
          </Button>
        )}
        {change && (
          <div className="form-width">
            <p>选择变更内容。新资料审核通过前，原企业资料保持有效。</p>
            {[
              ["legal_name", "企业名称"],
              ["legal_identifier", "登记号码"],
            ].map(([value, label]) => (
              <Checkbox
                key={value}
                checked={changeFields.includes(value)}
                onChange={(on) =>
                  setChangeFields((s) =>
                    on ? [...s, value] : s.filter((v) => v !== value),
                  )
                }
              >
                {label}
              </Checkbox>
            ))}
            <Select
              value={reason}
              onChange={(v) => setReason(String(v))}
              options={[
                { label: "企业名称变更", value: "legal_name_change" },
                {
                  label: "登记信息变更",
                  value: "enterprise_registration_change",
                },
                { label: "证件号码更正", value: "identifier_correction" },
                { label: "其他", value: "other" },
              ]}
            />
            <Button
              tag="button"
              loading={action.busy}
              disabled={!changeFields.length || !reason}
              onClick={() =>
                void action.run(async () => {
                  await api.createEnterpriseLegalChange({
                    expectedProfileVersion: state!.profile!.profileVersion,
                    fieldCodes: changeFields,
                    reasonCode: reason,
                    idempotencyKey: actionKey("enterprise-change"),
                  });
                  clearActionKey("enterprise-change");
                  setChange(false);
                  setAccepted(false);
                  data.reload();
                })
              }
            >
              建立变更申请
            </Button>
          </div>
        )}
        {canStart && (
          <div className="notice">
            <h3>企业认证信息处理说明</h3>
            <p>
              为确认付款、开票及服务使用主体，需要处理企业名称、统一社会信用代码、法定代表人与经办人身份资料，以及您上传的证明文件。
            </p>
            <p>
              资料只进入本网站的合规服务及获授权的审核人员，材料在服务端加密保存，不发送给软件提供方。页面不在浏览器存储中保存这些内容。
            </p>
            <p>
              您可在提交前停止；被退回后可修正并重提。资料变更审核期间，原认证继续有效。说明版本：
              {state.policyVersion}
            </p>
            <Checkbox checked={accepted} onChange={setAccepted}>
              我已阅读并同意本次企业认证信息处理
            </Checkbox>
            <p>
              <Button
                tag="button"
                disabled={!accepted}
                loading={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    const consent =
                      await api.createEnterpriseVerificationConsent({
                        policyVersion: state.policyVersion,
                        locale: "zh-CN",
                        idempotencyKey: actionKey("enterprise-consent"),
                      });
                    await api.createEnterpriseVerificationDraft({
                      consentNo: consent.consentNo,
                      legalChangeRequestNo:
                        state.activeLegalChange?.requestNo || "",
                      idempotencyKey: actionKey("enterprise-draft"),
                    });
                    clearActionKey("enterprise-consent");
                    clearActionKey("enterprise-draft");
                    setAccepted(false);
                    data.reload();
                  })
                }
              >
                同意并填写企业资料
              </Button>
            </p>
          </div>
        )}
        {draft && (
          <Form
            form={form}
            className="form-width"
            labelAlign="top"
            onSubmit={({ fields, validateResult }) => {
              if (validateResult !== true) return;
              void action.run(async () => {
                const input = {
                  ...fields,
                  agentName:
                    role === "authorized_agent" ? fields.agentName : "",
                  agentIdentityNumber:
                    role === "authorized_agent"
                      ? fields.agentIdentityNumber
                      : "",
                  industryCode: "",
                  registrationCountryCode: "CN",
                  applicantRoleCode: role,
                  materialNos: state!
                    .case!.materials.filter(
                      (m) =>
                        m.statusCode === "available" &&
                        types.includes(m.materialTypeCode),
                    )
                    .map((m) => m.materialNo),
                  idempotencyKey: actionKey("enterprise-submit"),
                } as api.EnterpriseSubmissionInput;
                await api.submitEnterpriseVerification(
                  state!.case!.caseNo,
                  input,
                );
                form.reset();
                clearActionKey("enterprise-submit");
                data.reload();
              });
            }}
          >
            <label className="field">
              申请人身份
              <Select
                value={role}
                onChange={(v) => setRole(v as typeof role)}
                options={[
                  { label: "法定代表人", value: "legal_representative" },
                  { label: "授权经办人", value: "authorized_agent" },
                ]}
              />
            </label>
            {[
              ["legalName", "企业名称"],
              ["registrationIdentifier", "统一社会信用代码"],
              ["legalRepresentativeName", "法定代表人姓名"],
              ["legalRepresentativeIdentityNumber", "法定代表人证件号码"],
              ...(role === "authorized_agent"
                ? [
                    ["agentName", "经办人姓名"],
                    ["agentIdentityNumber", "经办人证件号码"],
                  ]
                : []),
            ].map(([name, label]) => (
              <Form.FormItem
                key={name}
                name={name}
                label={label}
                rules={[{ required: true, message: `请输入${label}` }]}
              >
                <Input autocomplete="off" />
              </Form.FormItem>
            ))}
            <h3>证明材料</h3>
            <p>PDF、JPG 或 PNG，每份不超过 5 MB。</p>
            {types.map((type) => (
              <label className="field" key={type}>
                {materialLabels[type]}
                {state?.case?.materials.some(
                  (m) =>
                    m.materialTypeCode === type && m.statusCode === "available",
                ) && " · 已上传"}
                <input
                  type="file"
                  accept="application/pdf,image/jpeg,image/png"
                  disabled={action.busy}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    e.target.value = "";
                    if (file)
                      void action.run(async () => {
                        if (
                          file.size > 5 * 1024 * 1024 ||
                          ![
                            "application/pdf",
                            "image/jpeg",
                            "image/png",
                          ].includes(file.type)
                        )
                          throw new Error(
                            "请选择不超过 5 MB 的 PDF、JPG 或 PNG",
                          );
                        await api.uploadEnterpriseMaterial(
                          state!.case!.caseNo,
                          type,
                          file,
                          crypto.randomUUID(),
                        );
                        data.reload();
                      });
                  }}
                />
              </label>
            ))}
            <Button
              tag="button"
              type="submit"
              disabled={!ready}
              loading={action.busy}
            >
              提交审核
            </Button>
          </Form>
        )}
      </FetchState>
    </Card>
  );
}
