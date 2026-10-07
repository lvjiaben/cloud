import type { Page, Route } from "@host-tests/support/playwright";

const providerURL =
  "**/x/idc-compliance/api/v1/admin/compliance/providers/aliyun-face";

type ConfigurationStatus = "configured" | "draft" | "updating";

interface ProviderConfigurationProjection {
  credentialConfigured: boolean;
  credentialGeneration: number;
  credentialMaskedTail: string;
  providerCode: "aliyun_face";
  sceneId: string;
  status: ConfigurationStatus;
  updatedAt: number;
  validatedAt: null | number;
  validationStatus: "reachable" | "untested";
  version: number;
}

export interface ProviderConfigurationMockOptions {
  initialStatus?: ConfigurationStatus;
  staleLogin?: boolean;
}

export interface ProviderConfigurationMockState {
  configurationRequests: Array<Record<string, unknown>>;
  recoveryRequests: Array<Record<string, unknown>>;
  validationRequests: number;
}

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function configuration(
  status: ConfigurationStatus,
): ProviderConfigurationProjection {
  const configured = status === "configured";
  return {
    credentialConfigured: configured,
    credentialGeneration: configured ? 4 : status === "updating" ? 3 : 0,
    credentialMaskedTail: configured ? "K9M2" : "",
    providerCode: "aliyun_face",
    sceneId: status === "draft" ? "" : "100000000001",
    status,
    updatedAt: 1_786_161_660_000,
    validatedAt: null,
    validationStatus: "untested",
    version: status === "draft" ? 0 : 7,
  };
}

async function fulfillRecentLoginRequired(route: Route) {
  await route.fulfill({
    json: {
      code: 1,
      errorCode: "IDC_COMPLIANCE_PROVIDER_RECENT_AUTH_REQUIRED",
      message: "请重新登录后再修改或验证合规服务商凭据",
    },
  });
}

export async function installProviderConfigurationMock(
  page: Page,
  options: ProviderConfigurationMockOptions = {},
): Promise<ProviderConfigurationMockState> {
  const state: ProviderConfigurationMockState = {
    configurationRequests: [],
    recoveryRequests: [],
    validationRequests: 0,
  };
  let current = configuration(options.initialStatus ?? "draft");

  await page.route(`${providerURL}/validations`, async (route: Route) => {
    state.validationRequests += 1;
    if (options.staleLogin) {
      await fulfillRecentLoginRequired(route);
      return;
    }
    current = {
      ...current,
      validatedAt: 1_786_161_720_000,
      validationStatus: "reachable",
    };
    await route.fulfill({ json: apiEnvelope({ item: current }) });
  });

  await page.route(`${providerURL}/recoveries`, async (route: Route) => {
    state.recoveryRequests.push(
      route.request().postDataJSON() as Record<string, unknown>,
    );
    if (options.staleLogin) {
      await fulfillRecentLoginRequired(route);
      return;
    }
    current = {
      ...current,
      credentialConfigured: true,
      credentialGeneration: 4,
      credentialMaskedTail: "N7P4",
      status: "configured",
      version: 8,
    };
    await route.fulfill({
      json: apiEnvelope({ item: current, reusedExistingMutation: true }),
    });
  });

  await page.route(providerURL, async (route: Route) => {
    const method = route.request().method();
    if (method === "GET") {
      await route.fulfill({ json: apiEnvelope({ item: current }) });
      return;
    }
    if (method === "PUT") {
      const input = route.request().postDataJSON() as Record<string, unknown>;
      state.configurationRequests.push(input);
      if (options.staleLogin) {
        await fulfillRecentLoginRequired(route);
        return;
      }
      const accessKeyID = String(input.accessKeyId ?? "");
      current = {
        ...current,
        credentialConfigured: true,
        credentialGeneration: current.credentialGeneration + 1,
        credentialMaskedTail: accessKeyID.slice(-4),
        sceneId: String(input.sceneId ?? ""),
        status: "configured",
        version: current.version + 1,
      };
      await route.fulfill({ json: apiEnvelope({ item: current }) });
      return;
    }
    await route.fallback();
  });

  return state;
}
