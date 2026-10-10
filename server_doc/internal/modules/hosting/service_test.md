# service_test.go

主机指纹回归测试覆盖纯摘要、标准前缀、完整 ssh-keygen 输出、首尾空白与 Base64 填充；拒绝 MD5、错误长度/编码和多条指纹，验证 SaveServer 在凭证与数据库访问前完成指纹校验。

无外部依赖的安全边界测试：AES-GCM 随机性/篡改拒绝与 JSON 秘密隐藏，GORM DryRun 检查认证 TenantID 不能被请求查询覆盖；危险路径和超大上传在 IO 前拒绝。

真实协议测试使用内存 SFTP 服务拒绝文件和父目录符号链接；本机 SSH 测试服务验证主机 pin 与取消处理。Compose 测试检查 loopback、数据库无公网端口、命名数据卷、控制目录隔离及 $ 密码正确转义。

多主机密钥回归测试复现默认 SSH 库优先 ECDSA 导致 ED25519 pin 不匹配；验证 ED25519/ECDSA/RSA 三类 pin 均可连接，错误 pin 零次密码认证，密码错误仅认证一次并明确提示。另覆盖旧纯摘要存储及连接错误分类。

不能替代真实 Linux chroot 与 Directus 容器验收。

路由验收验证租户无 SSH/终端/分配/部署接口；签发真实租户 JWT 请求平台命令、WebSocket 终端和部署路由必须返回 403 / 20005，匿名请求返回 401。终端测试使用浏览器 bearer.<token> 子协议传递真实签名 JWT。
