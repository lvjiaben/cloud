import { strict as assert } from "node:assert";
import { test } from "node:test";

import { nativeTableLines } from "./audit-business-tables.mjs";

test("detects templates, Ant tables, HTML strings and JavaScript DOM factories", () => {
  assert.deepEqual(nativeTableLines([
    "<table class='rows'>",
    "<a-table :data-source='rows'>",
    "const markup = `<table><tbody></tbody></table>`;",
    "const node = document.createElement('table');",
    "return el(\"table\", {}, children);",
    "return h('table', rows);",
    "<Table :columns='columns'>",
    "<vxe-table />",
    "<vxe-grid />",
    "<el-table :data='rows' />",
    "<n-data-table :data='rows' />",
    "<VxeGrid />",
  ].join("\n")), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]);
});

test("does not mistake an actual Vben Grid or unrelated table names for a native table", () => {
  assert.deepEqual(nativeTableLines([
    "const [Grid, api] = useVbenVxeGrid({ gridOptions: {} });",
    "<Grid :table-title='title' />",
    "<VbenDataGrid />",
    "const tableName = 'orders';",
    "const node = document.createElement('button');",
  ].join("\n")), []);
});
