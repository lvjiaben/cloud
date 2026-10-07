import type { Page, Route } from "@host-tests/support/playwright";
import { numberedMockPage } from "./numbered-page-mock";

const base =
  "**/x/idc-compliance/api/v1/admin/compliance/enterprise-verifications";
const listURL =
  /\/x\/idc-compliance\/api\/v1\/admin\/compliance\/enterprise-verifications(?:\?.*)?$/u;
const caseNo = "ent_20260813_0000000000000001";
const materialNo = "mat_20260813_0000000000000001";

export interface EnterpriseVerificationMockState {
  accessRequests: number;
  contentRequests: Array<Record<string, unknown>>;
  decisions: Array<{ action: string; body: Record<string, unknown> }>;
  status: string;
  version: number;
}

export interface EnterpriseCustomerSubmissionMockState {
  draftRequests: Array<Record<string, unknown>>;
  reservationRequests: Array<Record<string, unknown>>;
  submissionRequests: Array<Record<string, unknown>>;
  uploadedByteSizes: number[];
}

function envelope(data: unknown) {
  return { code: 0, data, message: "OK" };
}

export async function installEnterpriseVerificationMock(page: Page, rowCount = 1) {
  const state: EnterpriseVerificationMockState = {
    accessRequests: 0,
    contentRequests: [],
    decisions: [],
    status: "pending",
    version: 2,
  };
  const item = () => ({
    applicantRoleCode: "legal_representative",
    caseNo,
    customerNo: "CUS_20260813_0001",
    decisionReasonCode:
      state.status === "verified" ? "manual_review_passed" : "",
    maskedLegalName: "测******公司",
    maskedRegistrationId: "91************0Y46",
    reverification: true,
    statusCode: state.status,
    submittedAt: 1_786_480_800_000,
    updatedAt: 1_786_480_860_000,
    version: state.version,
  });

  await page.route(
    `${base}/${caseNo}/materials/${materialNo}/content`,
    async (route: Route) => {
      state.contentRequests.push(
        (route.request().postDataJSON() ?? {}) as Record<string, unknown>,
      );
      await route.fulfill({
        body: Buffer.from("%PDF-1.4 enterprise fixture"),
        contentType: "application/pdf",
        headers: {
          "Content-Disposition": "attachment; filename=enterprise-material.pdf",
          "Cache-Control": "no-store",
        },
        status: 200,
      });
    },
  );

  await page.route(
    `${base}/${caseNo}/materials/${materialNo}/access-contexts`,
    async (route: Route) => {
      state.accessRequests += 1;
      await route.fulfill({
        json: envelope({
          accessContext: "ra1.single-use.fixture",
          expiresAt: 1_786_480_920_000,
        }),
      });
    },
  );

  for (const action of ["review-starts", "approvals", "rejections"]) {
    await page.route(`${base}/${caseNo}/${action}`, async (route: Route) => {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      state.decisions.push({ action, body });
      state.version += 1;
      state.status =
        action === "review-starts"
          ? "verifying"
          : action === "approvals"
            ? "verified"
            : "rejected";
      await route.fulfill({
        json: envelope({
          item: {
            caseNo,
            decisionReasonCode:
              action === "approvals"
                ? "manual_review_passed"
                : action === "rejections"
                  ? String(body.reasonCode ?? "")
                  : "",
            reused: false,
            statusCode: state.status,
            version: state.version,
          },
        }),
      });
    });
  }

  await page.route(`${base}/${caseNo}`, async (route: Route) => {
    await route.fulfill({
      json: envelope({
        item: {
          ...item(),
          legalProfile: {
            agentIdentityNumber: "",
            agentName: "",
            agentPresent: false,
            applicantRoleCode: "legal_representative",
            industryCode: "IDC",
            legalName: "测******公司",
            legalRepresentativeId: "**************0015",
            legalRepresentativeName: "测*人",
            registrationCountryCode: "CN",
            registrationIdentifier: "91************0Y46",
          },
          materials: [
            {
              byteSize: 4096,
              legalHold: false,
              materialNo,
              materialTypeCode: "business_license",
              mediaType: "application/pdf",
              retentionUntil: null,
              sequenceNo: 1,
              statusCode: "available",
            },
          ],
        },
      }),
    });
  });

  await page.route(listURL, async (route: Route) => {
    const query = new URL(route.request().url()).searchParams;
    const rows = Array.from({ length: rowCount }, (_, index) => ({ ...item(), caseNo: index ? `${caseNo}_${index}` : caseNo }));
    const filtered = rows.filter((row) => (!query.get("customerNo") || row.customerNo === query.get("customerNo")) && (!query.get("statusCode") || row.statusCode === query.get("statusCode")));
    await route.fulfill({
      json: envelope(numberedMockPage(filtered, query)),
    });
  });
  return state;
}

export async function installEnterpriseCustomerSubmissionMock(page: Page) {
  const accountBase =
    "/x/idc-compliance/api/v1/account/compliance/enterprise-verifications";
  const state: EnterpriseCustomerSubmissionMockState = {
    draftRequests: [],
    reservationRequests: [],
    submissionRequests: [],
    uploadedByteSizes: [],
  };

  await page.route(`**${accountBase}`, async (route: Route) => {
    const body = route.request().postDataJSON() as Record<string, unknown>;
    state.draftRequests.push(body);
    await route.fulfill({
      json: envelope({
        item: {
          caseNo,
          legalChangeRequestNo: body.legalChangeRequestNo ?? "",
          reverification: Boolean(body.legalChangeRequestNo),
          statusCode: "draft",
          version: 1,
        },
      }),
    });
  });
  await page.route(`**${accountBase}/${caseNo}/materials`, async (route: Route) => {
    state.reservationRequests.push(
      route.request().postDataJSON() as Record<string, unknown>,
    );
    await route.fulfill({
      json: envelope({
        item: {
          materialNo,
          materialTypeCode: "business_license",
          statusCode: "reserved",
        },
      }),
    });
  });
  await page.route(
    `**${accountBase}/${caseNo}/materials/${materialNo}/content`,
    async (route: Route) => {
      state.uploadedByteSizes.push(route.request().postDataBuffer()?.byteLength ?? 0);
      await route.fulfill({
        json: envelope({
          item: {
            byteSize: route.request().postDataBuffer()?.byteLength ?? 0,
            materialNo,
            mediaType: "application/pdf",
            statusCode: "available",
          },
        }),
      });
    },
  );
  await page.route(
    `**${accountBase}/${caseNo}/submission`,
    async (route: Route) => {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      state.submissionRequests.push(body);
      await route.fulfill({
        json: envelope({
          item: {
            caseNo,
            maskedLegalName: "测******公司",
            maskedRegistrationId: "91************0Y46",
            statusCode: "pending",
            version: 2,
          },
        }),
      });
    },
  );
  return state;
}

export { caseNo as enterpriseCaseNo, materialNo as enterpriseMaterialNo };
