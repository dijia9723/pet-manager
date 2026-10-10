# directus-bootstrap.js

嵌入部署脚本，在对应 Directus 容器内通过本机 REST API 用初始管理员环境变量登录。只创建缺失的 site_articles/site_pages/site_navigation/site_settings，复用 Directus 富文本和字段界面。

内容默认 draft，允许 published/archived；不自动打开 Public 权限，不修改现有模型。错误日志为固定提示，不包含密码或 token。认证失败则停止初始化并让部署失败。
