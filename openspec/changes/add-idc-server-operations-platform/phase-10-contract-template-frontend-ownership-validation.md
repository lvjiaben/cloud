# 第十阶段合同模板前端归属纠偏验证记录

## 结论

合同模板管理和编辑页面继续由`idc-compliance/frontend/`拥有，并通过现有源码插件页面机制显示在 LinaPro 管理后台。合同编辑器使用的`Tiptap`和`pdfjs-dist`已经迁入插件自己的依赖、锁文件、构建和嵌入范围；`apps/lina-vben/apps/web-antd/`的正式源码、配置、直接依赖及其锁文件导入项均已恢复到当前 Lina 宿主基线。

本项是 IDC 管理后台的插件页面，不属于客户页面。登录后客户可见的“我的合同”、签署和开票页面仍须在所属阶段直接进入与`web-antd`同结构写法的`apps/lina-vben/apps/web-customer/`；公开官网只能进入`apps/lina-websites/`。

## 问题原因与修复

原实现虽然把合同页面放在合规插件中，却把页面使用的新增`Tiptap`表格扩展、文本对齐扩展和`PDF`依赖加入了`web-antd/package.json`。原因是插件页面在宿主前端编译时直接解析这些依赖，错误地把宿主依赖当成了插件交付捷径。

修复后的边界如下：

- `idc-compliance/frontend/package.json`和独立`pnpm-lock.yaml`锁定业务依赖与构建工具。
- `frontend/scripts/build-vendor.mjs`使用跨平台 Node 接口和`esbuild`生成`frontend/vendor/`中的浏览器`ESM`、类型声明、`PDF Worker`与`SHA-256`摘要清单。
- 插件页面只通过相对路径读取`frontend/vendor/`产物，不再从`web-antd`读取`Tiptap`或`PDF`依赖；`Vue`仍作为宿主共享运行时。
- `plugin_embed.go`只嵌入`plugin.yaml`、插件页面、插件预构建产物和`manifest`，不嵌入`node_modules`与构建源码。
- 根`linactl build`先执行所有启用插件的自有构建，再构建宿主前端和后端，避免宿主编译时读取尚未生成的插件产物。

## 验证结果

| 验证项 | 命令或证据 | 结果 |
|---|---|---|
| 插件独立安装 | 在`idc-compliance/frontend/`执行`pnpm install --frozen-lockfile` | 通过 |
| 插件自有构建 | 执行`pnpm run build`和`pnpm run build:check` | 通过，产物与摘要清单一致 |
| 官方定向构建入口 | 执行`go run ./hack/tools/linactl build dir=apps/lina-plugins/idc-compliance plugins=1 verbose=1` | 通过 |
| 构建顺序 | 执行`go test ./hack/tools/linactl -run TestRunBuildRunsPluginConfigCommandsBeforeFrontendAndBackend -count=1` | 通过 |
| 插件嵌入编译 | 使用`temp/go.work.plugins`执行合规插件根包`go test .` | 通过 |
| 宿主消费插件页面类型 | 设置`LINAPRO_SOURCE_PLUGINS=1`后执行`pnpm --filter @lina/web-antd typecheck` | 通过 |
| 生产前端构建 | 设置`LINAPRO_SOURCE_PLUGINS=1`后执行`pnpm run build --filter=@lina/web-antd` | 通过，`11/11`任务成功 |
| 宿主基线 | 比对`web-antd/.env.development`、`package.json`、`vite.config.mts`及总锁文件中的`apps/web-antd`导入项 | 与当前`HEAD`一致 |
| IDC 宿主污染扫描 | 扫描`web-antd/src`、正式配置和依赖中的`linapro-idc`、合同编辑器及业务依赖标识 | 未发现本项 IDC 内容 |
| 中文真实浏览器 | Google Chrome 执行`TC-4b-1` | 通过 |
| 英文暗色真实浏览器 | Google Chrome 执行`TC-4b-2` | 通过 |
| 合同模板整组回归 | Google Chrome 执行精确`TC-4b-`用例组 | `8 passed`，插件目录、双语编辑器、本地正文、复制和启用确认均通过 |
| OpenSpec | `openspec validate add-idc-server-operations-platform --strict` | 通过 |

本项归属纠偏的中文与英文编辑器用例为`2 passed`；合同模板当前整组回归为`8 passed`。截图为`temp/20260814/163414-contract-template-editor-zh-cn.png`和`temp/20260814/163418-contract-template-editor-en-us-dark.png`；已检查中英文、暗色主题、页面布局、表单内容、原始翻译键和明显遮挡，没有发现本项问题。

## 本轮补充收口门禁

- Windows 的`CloseHandle`返回值现已检查；`.\make.cmd lint.go dir=hack/tools/linactl plugins=0`重新执行通过，`golangci-lint`为`0 issues`且`staticcheck U1000`通过。
- `.\make.cmd i18n.check`重新执行通过，运行时硬编码、宿主与启用插件双语资源覆盖均无阻断。
- 共享工作区中此前重复的宿主控制器声明已经不存在；在`apps/lina-core/`执行`GOWORK=off go test ./internal/cmd -count=1`通过。检查期间还发现资源插件启动声明用`panic`处理可预期契约错误，已改为由`Declare`和`Freeze`返回初始化错误；资源 SPI 测试与资源插件 lint 均通过。
- 在`hack/tests/`执行全量 TypeScript 编译和`pnpm test:validate`均通过；Windows 标准执行入口可以发现合规插件`4`个文件、`23`个测试。
- `git diff --name-only -- apps/lina-vben/apps/web-antd`为空，IDC 页面、接口、路由、状态、语言资源和业务依赖扫描没有发现宿主污染。

按照本轮`lina-review`结果，原有阻断已经修复且没有发现新的严重或一般问题，当前允许精确勾选`10.8.c.5`和`FB-14`。这只关闭合同编辑器宿主依赖越界反馈，不代表`web-customer`、官网迁移或第十阶段其他业务任务完成。

## 影响判断

- `i18n`：生产文案和现有双语资源没有变化；新增测试分别验证中文与英文实际文案，未显示原始键。全仓检查的既有阻断已单独记录。
- 数据权限、接口、数据库和缓存：本项只调整前端依赖归属、静态产物和构建顺序，没有改变业务接口、权限效果、数据模型、数据库访问或缓存行为。
- 运行期依赖：没有新增后端运行期服务或依赖注入；前端业务依赖 owner 从宿主纠正为合规插件，由插件构建时生成静态产物并随插件嵌入。
- 跨平台：插件构建使用 Node 标准文件接口、`node:path`和`esbuild`，构建顺序使用现有 Go 工具；Windows 已实际通过，Linux 与 macOS 本轮未执行。
