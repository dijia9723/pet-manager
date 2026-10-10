# directus-bootstrap.test.cjs

Node 内置测试运行初始化脚本，通过受控 fetch 验证四个集合、草稿默认值、Directus 富文本接口、仅创建缺失模型，以及认证失败不写数据、不输出秘密。

运行：node --test internal/modules/hosting/directus-bootstrap.test.cjs。无需连接真实 CMS。
