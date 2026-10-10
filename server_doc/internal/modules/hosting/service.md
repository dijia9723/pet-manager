# service.go

NewService 的可选 allowedOrigins 参数由 bootstrap 传入 CORS 来源列表，供 WebSocket 终端验证 Origin。

服务器保存时将主机指纹输入规范化为 SHA256: 格式，支持纯摘要及 ssh-keygen -lf 完整单行输出；请求字段最多 1024 字符，数据库只保存规范化后的指纹。

服务器/网站平台台账的 CRUD 与访问控制。新增网站检验租户和服务器，并生成独立 SFTP Ed25519 密钥；分配关系不可迁移。事务行锁避免服务器删除与网站分配竞争。

scopeWebsites 对租户查询加认证 TenantID 条件，没有租户 ID 或身份则拒绝。ListQuery 的 tenant_id 仅对平台生效。租户列表隐藏 ServerID/CMSPort，所有凭证字段 JSON 忽略。

模块可在无 hosting 密钥时注册，但创建凭证失败并提示配置；有效密钥为 Base64 的 32 字节随机值。
