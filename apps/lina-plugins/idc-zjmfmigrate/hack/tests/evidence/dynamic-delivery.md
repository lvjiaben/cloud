# 动态页面交付验证

日期：2026-10-09。发布清单：`v0.2.2`。

## 根因与交付

原页面依赖宿主Vue路径且没有自有构建入口，广义`go:embed frontend`还会扫描依赖目录。现交付`admin/migration/index.html`、独立锁文件及构建配置，仅嵌入`frontend/dist`；缺失声明资产时构建直接失败。

`migration-checks`页面只经`idc-migration.admin.v1`桥调用已有`admin/migration/*`业务契约，写入使用`body`载荷，宿主与业务拥有方继续校验权限。来源预检成功才能创建批次；正式导入、执行、切换和回滚要求确认。页面提供冲突准确分页、决定表单、对账和报告，不读取宿主存储、Cookie或令牌。

## 已验证

- 插件完整构建和WASM装配通过，前端8项单测通过。
- `TC004-dynamic-page-delivery.ts`在Chromium运行真实编译页面和官方Vben表单抽屉，验证创建门禁、来源填写、实际预检回调，以及POST请求的能力与body字段。
- 浏览器桥响应为fixture，不连接真实来源数据库，不证明正式导入完成。统一重编译和启用后还需验证真实宿主桥装配。
