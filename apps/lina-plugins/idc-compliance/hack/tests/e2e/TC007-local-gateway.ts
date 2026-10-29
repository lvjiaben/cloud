import { execFile } from "node:child_process";
import { promisify } from "node:util";

import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

const runFile = promisify(execFile);
const gatewayRoot = new URL("../../../central-gateway/", import.meta.url);

async function goTest(packages: string[], pattern: string, postgres = false) {
  if (
    postgres &&
    !process.env.LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN?.trim()
  ) {
    throw new Error(
      "TC007 requires LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN for the real local PostgreSQL journey",
    );
  }
  const { stderr, stdout } = await runFile(
    "go",
    ["test", ...packages, "-count=1", "-run", pattern, "-v"],
    {
      cwd: gatewayRoot,
      env: { ...process.env, GOWORK: "off" },
      maxBuffer: 8 * 1024 * 1024,
      windowsHide: true,
    },
  );
  const output = `${stdout}\n${stderr}`;
  expect(output).toContain("PASS");
  expect(output).not.toContain("--- SKIP:");
  return output;
}

test.describe("TC-7 随安装交付的本地合规网关", () => {
  test("TC-7a: 本地安装标识和进程凭据可以生成、轮换与撤销", async () => {
    await goTest(
      [
        "./pkg/processauth",
        "./cmd/idc-compliance-local-gateway-keyring",
      ],
      "Test(GenerateRotateRevokeAndSign|RunGeneratesAndRotatesWithoutOverwrite|RunRejectsRelativeAndRevocationWithoutRotation)$",
    );
  });

  test("TC-7c: 商家自己的套餐或授权不足时停止新的收费调用", async () => {
    await goTest(
      ["./internal/localusage"],
      "Test(AdmitNewChargeStopsInsufficientPlanAndKeepsUnmeteredHistoryAvailable)$",
    );
  });

  test("TC-7d: 供应商直接回调在本地数据库精确去重且不能跨路由重放", async () => {
    await goTest(
      ["./internal/suppliercallback"],
      "TestSupplierCallbackPostgreSQL(ConcurrentDeduplicationAndBounds|ConcurrentCrossRouteEventConflict)$",
      true,
    );
  });

  test("TC-7e: 本地灾备恢复不依赖软件提供方业务服务", async () => {
    await goTest(
      ["./internal/datalifecycle"],
      "TestLocalLifecyclePostgreSQLExecutionAndRestoreDrill$",
      true,
    );
    await goTest(
      ["./internal/app", "./internal/localops"],
      "Test(RuntimeConfigurationHasOnlyInstallationLocalDependencies|MissingClientDoesNotFallbackAndTakeoverRequiresCurrentMerchantEmployee)$",
    );
  });
});
