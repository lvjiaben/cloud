import type { Page } from "@host-tests/support/playwright";
import { NativeServerPage } from "./NativeServerPage";

export class AccountConsolePage {
  constructor(
    readonly page: Page,
    readonly origin: string,
  ) {}
  get root() {
    return this.page.getByTestId("customer-console");
  }
  get servers() {
    return this.page.getByTestId("native-server-list");
  }
  get orders() {
    return this.page.getByTestId("console-recent-orders");
  }
  get services() {
    return this.page.getByTestId("console-services");
  }
  get serviceTotal() {
    return this.page.getByTestId("console-service-total");
  }
  retryServices() {
    return this.page.getByTestId("console-services-retry").click();
  }
  openAllServices() {
    return this.serviceTotal.click();
  }
  retryBalance() {
    return this.root.getByRole("button", { name: /重\s*试/ }).click();
  }
  open() {
    return this.page.goto(this.origin + "/account");
  }
  manageFirst() {
    return this.servers
      .getByRole("button", { name: "管理", exact: true })
      .first()
      .click();
  }
  openServers() {
    return this.page.goto(this.origin + "/account/servers");
  }
  screenshot(step: string) {
    return new NativeServerPage(this.page, this.origin, "").screenshot(
      "console-" + step,
    );
  }
}
