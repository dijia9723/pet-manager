# deploy.go

normalizeFingerprint 支持纯 Base64 SHA256 摘要、带 SHA256: 前缀的指纹和 ssh-keygen -lf 单行输出。严格验证 Base64 编码及 32 字节长度，拒绝多行或多条指纹，返回统一的 SHA256: 无填充格式。

目录初始化使用固定系统路径/生成的账号，要求 Linux/systemd/OpenSSH。平台脚本支持 root 直接执行或通过 sudo -n -u root 免密提权；不支持提权时返回明确提示，租户文件访问仍使用受限账号。flock 串行 SSH 配置变更，写 Match User 的 chroot/internal-sftp、公钥认证、禁用转发/TTY；sshd -t 与 -T 验证失败恢复原配置。实际 SFTP 连接验证 /www、根目录与不可见 /etc/passwd，成功才 ready。

初始化脚本输出固定阶段标记，失败时仅将已知阶段映射为中文诊断（工具缺失、Include、配置校验、重载等），不向 API 回显任意远程输出。补齐 /usr/sbin 与 /sbin 的 PATH。

站点 chroot 根目录使用 root:root、0755，允许受限账号列出自身根目录完成隔离验证；公共父目录仍为 0711，www 为站点账号所有、0750。重试初始化会修复历史 0711 站点根目录。SFTP 根目录读取失败返回 10503 和具体诊断，避免被文件操作的通用权限错误映射为平台无权限 20005；发现宿主文件或其他根目录项仍拒绝验证。

Directus Compose 配置在 root 0700 控制目录，文件 0600，租户不可编辑；每站点独立项目、数据库、网络和数据卷，服务端口只绑定 loopback。环境变量中的 $ 需转成 $$，避免 Compose 插值改变密码。

用带 updated_at 条件的状态更新争抢部署，避免并发部署或编辑；CMS 端口由数据库唯一索引预留。失败仍保留 SECRET/数据库密码与持久化卷，重试初始化不覆盖内容。

CMS 远程脚本还使用按站点 flock，防止连接中断后远程旧进程仍运行时重试并发覆盖 Compose。SSH 配置锁与 CMS 站点锁等待最多 5 秒。

超过 10 分钟的中断状态在恢复时转 failed。完成状态使用独立短上下文，避免请求取消导致永久卡在 deploying。
