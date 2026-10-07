import type { Locator, Page } from "@host-tests/support/playwright";

import { MainLayout } from "@host-tests/pages/MainLayout";

/** Presentation-only probes: never modify application data or CSS. */
export class TableSurfacePage {
  constructor(readonly page: Page) {}

  async gotoAdmin(path: string) {
    await this.page.goto(`http://127.0.0.1:6031/admin${path}`, {
      waitUntil: "domcontentloaded",
    });
    await this.grids.first().waitFor({ state: "visible" });
  }

  get grids(): Locator {
    return this.page.locator(".vxe-grid:visible");
  }

  async theme(mode: "dark" | "light") {
    await new MainLayout(this.page).prepareThemeModeWithoutAnimation(mode);
    await this.grids.first().waitFor({ state: "visible" });
  }

  async movePointerAway() {
    await this.page.mouse.move(0, 0);
  }

  async inspect() {
    return this.grids.evaluateAll((grids) => {
      function surface(element: Element | null): string {
        while (element) {
          const color = getComputedStyle(element).backgroundColor;
          if (color !== "transparent" && color !== "rgba(0, 0, 0, 0)") {
            return color;
          }
          element = element.parentElement;
        }
        return "transparent";
      }
      const selectors = [
        ".vxe-table--header-wrapper",
        ".vxe-table--body-wrapper",
        ".vxe-grid--form-wrapper",
        ".vxe-grid--toolbar-wrapper",
        ".vxe-grid--pager-wrapper",
        ".vxe-table--empty-block",
        ".vxe-header--column",
        ".vxe-body--row:not(.row--hover):not(.row--current) .vxe-body--column",
      ];
      return grids.map((grid) => {
        const style = getComputedStyle(grid);
        return {
          background: surface(grid),
          areas: selectors.flatMap((selector) =>
            [...grid.querySelectorAll(selector)]
              .filter((element) => element.getBoundingClientRect().height > 0)
              .map((element) => ({ selector, color: surface(element) })),
          ),
          header: style.getPropertyValue("--vxe-ui-table-header-background-color").trim(),
          headerSurface: surface(grid.querySelector(".vxe-table--header-wrapper")),
          emptyColor: grid.querySelector(".vxe-table--empty-content")
            ? getComputedStyle(grid.querySelector(".vxe-table--empty-content")!).color
            : "",
          hover: style.getPropertyValue("--vxe-ui-table-row-hover-background-color").trim(),
          selected: style.getPropertyValue("--vxe-ui-table-row-current-background-color").trim(),
          layout: style.getPropertyValue("--vxe-ui-layout-background-color").trim(),
          outer: getComputedStyle(document.documentElement).getPropertyValue("--background-deep").trim(),
          overflow: document.documentElement.scrollWidth - window.innerWidth,
        };
      });
    });
  }
}
