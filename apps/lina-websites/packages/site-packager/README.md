# @lina/site-packager

本地可执行的官网包与品牌配置门禁。它只接受已经构建好的`.zip`和结构化品牌 JSON，不执行上传内容里的脚本，也不接受 HTML/JavaScript 片段。

命令：`lina-site install <site.zip>`、`preview`、`publish <packageId> <version>`、`rollback`、`uninstall <packageId>`；品牌命令为`brand save <brand.json>`、`brand preview <revision>`、`brand publish <revision>`、`brand rollback`。

每个版本写入不可变目录，发布只原子更新指针，失败时旧版本继续可用。品牌配置按修订保存，发布和回滚同样保留上一版本。`LINA_SITE_STORAGE`可指定存储目录。
