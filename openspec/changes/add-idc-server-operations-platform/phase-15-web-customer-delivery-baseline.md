# 第十五阶段旧 Nuxt 基线说明

> 本文件记录一次已经被否决的历史实施，不能再作为当前设计或后续开发依据。当前唯一有效的前端边界见[第十五阶段前端边界纠偏](phase-15-frontend-boundary-correction.md)、[设计](design.md)、[门户规范](specs/idc-merchant-portal/spec.md)和[任务清单](tasks.md)。

## 为什么作废

此前把公开官网和客户后台合并到一个 Nuxt `web-customer`中，虽然验证了服务端渲染、双语和`SEO`等技术能力，但违反了以下产品要求：

- 客户后台必须是`apps/lina-vben/apps/web-customer/`下的独立 Vben/Vite 应用，并一比一跟随`web-antd`的结构和写法。
- 官网必须独立于`lina-vben`，让第三方开发者无需进入 LinaPro 管理前端仓库也能开发、构建和交付官网。
- 官网必须能以预编译压缩包在商家后台本地安装、预览、切换和回滚，不能每次修改都重新编译 LinaPro。

因此，本文原先冻结的“Nuxt 客户后台”“官网与客户中心单一源码”“客户后台携带 Nuxt 服务进程”等结论全部作废。

## 历史代码如何处理

旧 Nuxt 原型没有直接删除，而是整体移到`apps/lina-websites/sites/default/`作为默认官网迁移基线。后续只保留公开商品、内容和`SEO`能力；购物车、订单、财务、实名、服务器和账户页面迁入 Vben `web-customer`。迁移门禁全部通过后，再原子删除旧 Portal 和重复源码。

历史测试通过记录只能证明旧原型当时可运行，不能证明新的 Vben 客户后台、独立官网包运行时或第十五阶段已经完成。
