# 宝塔字号与公共页脚修复

## 修复范围

本轮按局部反馈直接修复，源码仅修改官网插件，不提交、不推送、不创建`PR`，不重启`6680`。

宝塔页独立图片过滤错误地清空本方后台外域`Logo`；公共样式默认隐藏非首页页脚，依赖各页面补丁。现移除宝塔独立过滤，由公共页脚统一读取品牌。备案区原为空图标节点，统一显示本地公安图标和普通文件图标；后者不代表官方认证。没有备案编号时仍隐藏对应项。

宝塔五锚点居中，桌面`16px`、手机`14px`，首屏说明`18px`、按钮`14px`，正文使用系统字体。公共首屏按钮移除文字变换、滤镜与三维合成提示，避免悬停缩放造成模糊。公共页脚服务图标统一尺寸和深色背景颜色。

## 文件与验证

前端文件位于`frontend/public/lina/`：`bt.js`、`bt.css`、`site.js`、`site.css`、`footer.js`、`filing-document.svg`。测试涉及`TC132-bt.ts`、`TC135-shared-footer.ts`、`SharedFooterPage.ts`及`source-site.ts`。七条命令、六十九条正文及参数`59d9cde77`未改动，没有执行安装或挂载命令。

- 正文与品牌缓存单测：六项通过。
- 源码预览：`TC132`五项、公共页脚两项通过。
- 实际`HTTP`：设置`BT_LIVE=1`运行同组，七项通过。静态文件由`6680`真实提供，品牌异常与受控成功场景仍通过显式接口拦截构造，未冒充真实后台资料。
- 新公共测试最终编号为`TC135`；最初成功证据目录保留当时的`TC134`名称，避让并行任务后已重命名测试文件。
- 游戏`TC125`两项及`TC135`两项回归通过。安全`TC127`三项失败：正文提取器将并行商品功能的“正在加载商品…”或“暂无关联商品。”计入附件原文。该独立问题没有修改，不能宣称完整安全回归通过。
- 无拦截真实浏览器检查：首页、宝塔、游戏、安全、政企、金融、关于共七页均返回`200`，本方页脚`Logo`、公安及`ICP`备案图标全部解码成功，无页面横向溢出。中途品牌接口超时，失败页面在接口恢复后复核通过。
- 宝塔真实手机页脚无横向溢出，桌面与手机截图已视审，`Logo`及编号图标可见。

真实证据：`temp/20261025/1792900143044-live-footer/report.json`（政企、金融、关于成功），`temp/20261025/1792900315417-live-footer-retry/report.json`（宝塔、首页、游戏、安全成功）。实际宝塔页脚截图：`temp/20261025/1792899954896-live-footer/bt-real-footer.png`及`temp/20261025/1792900315417-live-footer-retry/bt-real-footer-mobile.png`。源码首屏证据位于`temp/20261025/1792899773541-website-test-results/`。

测试命令在插件根执行：

```powershell
node --test hack/tests/bt-content.test.mjs hack/tests/brand-identity.test.mjs
$env:PLAYWRIGHT_EXECUTABLE_PATH='C:/Program Files/Google/Chrome/Application/chrome.exe'
npx --no-install playwright test --config hack/tests/playwright.config.ts TC135-shared-footer.ts TC132-bt.ts
$env:BT_LIVE='1'
npx --no-install playwright test --config hack/tests/playwright.config.ts TC135-shared-footer.ts TC132-bt.ts
```

## 发布与审查

目标：[宝塔面板](http://127.0.0.1:6680/bt.html)。运行目录`temp/output/websites/website-default/public/`。六项文件备份及发布清单位于`temp/20261025/1792899830654-shared-footer-before/receipt.json`。运行`site.js`仅定点移除三处页面品牌映射，保留并行商品改动，未以源码整文件覆盖运行副本。其余文件差量同步，`HTTP`内容核对一致。

按`lina-review`审查本轮实现、差量发布和测试：未发现本次修复阻塞问题。已触发`E2E`质量审查：原始`Logo`过滤、页脚隐藏、备案图标、桌面手机居中字号与悬停文字变换有回归断言，品牌失败重试由现有宝塔测试覆盖；测试无业务写入、无跨用例状态依赖。没有修改正文或翻译资源、接口契约、缓存协议、数据权限或运行期依赖。安全旧原文测试的独立失败已保留，不计入通过；未逐个浏览官网全部页面，不声称全站逐页验收。
