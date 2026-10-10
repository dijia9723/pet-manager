# 服务器、网站与 Directus CMS

## 已接入的能力

平台菜单新增「服务器管理」与「网站管理」，租户新增「网站管理」。

| 能力 | 平台 | 租户 |
| --- | --- | --- |
| 服务器 SSH 配置、连接测试、xterm.js 交互终端 | 使用远程账号的系统权限；平台脚本支持 root 或免密 sudo | 无接口 |
| 网站分配、归属、目录初始化、CMS 部署 | 支持 | 不允许修改归属或服务器 |
| 网站文件浏览、下载 | 所有已分配站点 | 本租户站点 |
| 上传、文本编辑、新建目录、重命名、删除文件/空目录 | 支持 | 需要 tenant.website.write |
| Directus 内容管理 | 独立 Directus 后台 | 使用本网站 CMS 账号登录 |

文件上传/下载上限 10 MiB，在线编辑仅支持 1 MiB 内 UTF-8 文本；不提供递归删除。网站「文件维护状态」仅控制本系统文件接口，不关闭公网网站或 Directus。

## 复用的库与服务

- [Go 官方 SSH](https://pkg.go.dev/golang.org/x/crypto/ssh)：身份认证、主机 SHA256 指纹校验、PTY 和 Shell。
- [xterm.js](https://xtermjs.org/docs/) 与 [FitAddon](https://xtermjs.org/docs/guides/using-addons/)：浏览器终端与自动缩放；[coder/websocket](https://pkg.go.dev/github.com/coder/websocket)：终端的双向数据通道。
- [pkg/sftp](https://github.com/pkg/sftp)：文件读写、目录操作和原子替换，版本 1.13.11。
- [OpenSSH chroot / internal-sftp](https://man.openbsd.org/sshd_config)：系统级目录限制、禁用 shell、TTY 和转发。
- [Directus](https://directus.com/docs/self-hosting/deploying)：复用内容编辑器、媒体库、内容模型、账号角色、REST/GraphQL；当前部署固定 12.5.0，数据库使用 PostgreSQL 17.11。
- [Directus SDK](https://directus.com/docs/guides/connect/sdk)：独立网站前端可直接使用 npm 包 @directus/sdk。

Directus 当前许可证对向第三方提供托管服务有专门条款。本平台面向租户的网站托管是否属于允许范围，需要按经营模式核对 [当前许可文本](https://directus.com/license)。支持在部署表单中传入 LICENSE_KEY；填密钥不等于自动具备全部商业托管授权。

## 应用配置

设置稳定的 `PM_HOSTING_ENCRYPTION_KEY`，值为 32 个随机字节的 Base64 编码。通过部署环境注入，勿提交到源码仓库；备份时同时保管密钥，换密钥前必须做凭证迁移，否则已有凭证无法解密。

Windows PowerShell 生成示例：

```powershell
$hostingKeyBytes = New-Object byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Fill($hostingKeyBytes)
$env:PM_HOSTING_ENCRYPTION_KEY = [Convert]::ToBase64String($hostingKeyBytes)
```

Linux 示例：`export PM_HOSTING_ENCRYPTION_KEY="$(openssl rand -base64 32)"`。上述生成只应做一次，然后把结果保存到部署环境的密钥配置；每次重启随机生成会丢失既有凭证访问能力。

未配置密钥时原系统仍可启动，服务器创建和网站分配会明确提示配置缺失。非空但格式不合法的密钥使启动失败。

后端重启自动迁移两张平台表、CMS 端口唯一索引和菜单权限。重新登录刷新菜单；租户主账号具备新权限，子账号通过「角色授权」授予 tenant.website.list、tenant.website.files、tenant.website.write。

## 远程服务器要求与操作

当前自动初始化脚本针对 Linux + systemd + OpenSSH，建议 Debian/Ubuntu。需要 root 账号或可以免密执行 `sudo -n -u root true` 和 `sudo -n -u root sh -s` 的 SSH 账号，以及 useradd/usermod/getent、flock、base64 和 sshd_config 中的 `Include /etc/ssh/sshd_config.d/*.conf`。平台初始化和 CMS 部署自动提权；租户文件操作仍使用专用受限账号。

1. 在「服务器管理」填写主机、端口、SSH 账号、密码或无口令私钥及主机指纹。通过可信渠道读取服务器公钥指纹，例如 `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub`。支持纯 Base64 指纹值、`SHA256:` 前缀或该命令的完整单行输出；保存前验证 32 字节 SHA256 摘要并统一为 `SHA256:` 加无填充 Base64 格式，连接时严格核对该指纹。

   服务器同时提供多个主机密钥时，平台会尝试安全算法对应的密钥，找到与该指纹完全一致的密钥后才发送认证凭证。连接测试会区分指纹不匹配、密码/私钥认证失败、算法协商失败及网络超时；密码认证失败需核对账号密码以及该账号是否允许密码登录。
2. 测试连接。普通账号可执行其拥有权限的命令；自动目录初始化与 Docker 部署要求 root 账号或可通过 `sudo -n -u root` 免密提权的账号。
3. 在「网站管理」为租户分配网站。归属和服务器建立后不可修改，避免把旧目录或 CMS 交给另一租户。
4. 点击「初始化目录」。系统生成专用 Ed25519 密钥和 pm_site_<网站ID> 账号；创建隔离目录、配置并验证 sshd，重载服务，实际连接 SFTP 检查目录后才标记就绪。
5. 已初始化站点可使用文件管理。租户只看到自己的网站，无法选择其他租户的目录，也无法获取 SSH 密钥。

在服务器列表点击「进入终端」打开 xterm.js 交互会话，支持持续 Shell、目录切换、Tab、方向键、Ctrl+C 与终端缩放。关闭终端会释放远程会话，重新连接建立新 Shell；终端使用服务器配置的 SSH 账号，普通账号可在交互会话自行使用 sudo。

终端通过 GET `/api/v1/platform/servers/<ID>/terminal` 升级 WebSocket，仍需平台登录和 `platform.server.exec` 权限；JWT 使用独立的 `bearer.<token>` 子协议传递，URL 不携带凭证。开发环境 Vite `/api` 代理已开启 `ws: true`，修改后需重启 Vite。跨域部署需将前端来源加入 `cors.allow_origins`，反向代理需传递 WebSocket Upgrade/Connection 头并设置足够的空闲超时。

SFTP 初始化失败会提示具体阶段。若提示无法通过 `sudo -n` 提权，可在服务器用该 SSH 账号执行 `sudo -n -u root true` 核对权限；若提示 Include 未生效，可用 `sudo sshd -T -C user=pm_site_<ID>,host=localhost,addr=127.0.0.1` 核对 Match 配置。配置验证失败会尝试恢复原 SSH 配置。

目录约定：

| 路径 | 用途 / 权限 |
| --- | --- |
| /srv/pet-manager、/srv/pet-manager/sites | root:root、0711，受限账号不能列出其他站点 |
| /srv/pet-manager/sites/<网站ID> | chroot 根，root:root、0755，可列出自身目录，租户不能修改 |
| /srv/pet-manager/sites/<网站ID>/www | 独立网站账号拥有，0750；应用界面中的 / |
| /etc/ssh/pet-manager/pm_site_<网站ID> | root 管理的 authorized key |
| /etc/ssh/sshd_config.d/00-pet-manager-pm_site_<网站ID>.conf | 独立 Match User 配置 |
| /etc/pet-manager/sites/<网站ID>/compose.json | root 专用、0600；父目录 0700 |

租户文件请求始终使用专用 SFTP 账号，平台 root 凭证不会用于租户文件操作。路径拒绝 ..、反斜杠、控制字符和符号链接组件；根目录不能修改。系统隔离由 chroot 保证，即使发生并发文件替换也不能访问宿主机外部目录。

初始化使用 flock 串行管理 SSH 配置。配置语法或有效配置验证失败时恢复原配置；中途失败可能留下账号和目录，重试复用相同密钥与站点 ID。请求取消或超时会断开 SSH。

旧版本曾将站点 chroot 根目录设为 0711，导致初始化后的根目录列表验证被拒绝并误报 `20005 无权限执行该操作`。更新后重试「初始化目录」会修复为 0755，租户仍只能修改自己的 www。根目录读取失败会返回具体的 SFTP 验证提示。

## Directus 部署与内容

远程服务器安装 Docker Engine 与支持 `up --wait` 的 Docker Compose 插件。选择 Directus，填写网站根地址与 CMS 独立根地址，然后「部署 CMS」填写独立本机端口、初始管理员邮箱/密码及可选许可证密钥。

部署项目名为 `pm-site-<网站ID>`，每站点独立容器网络、数据库、uploads/extensions 数据卷；数据库不映射公网端口，Directus 仅监听 `127.0.0.1:<分配端口>`。不挂载宿主机目录、Docker socket 或租户编辑的 Compose 文件。

生成不同站点的数据库密码和 SECRET，并加密保存，失败重试保留数据和密钥。健康检查通过后，在该实例内使用官方 API 初始化：

| 集合 | 功能 |
| --- | --- |
| site_articles | 标题、唯一 slug、摘要、富文本正文、发布时间、草稿/发布/归档 |
| site_pages | 页面标题、唯一 slug、富文本正文、草稿/发布/归档 |
| site_navigation | 导航名称、链接、排序、草稿/发布/归档 |
| site_settings | 网站名称、描述、邮箱、SEO，单条设置 |

媒体库使用 Directus 内置文件管理。初始化只创建缺失集合，不修改已有模型和内容。初始管理员密码仅用于首次建号；重试不会重置已存在账号，初始化尚未成功前重试需使用当前有效密码。

平台应用账号和 Directus 账号分别管理，当前未实现单点登录或账号自动同步。部署完成后在 CMS 给租户交付账号，按需要建立编辑者角色；CMS 管理员仅管理本实例，无法访问其他站点。

Directus Public 权限默认关闭。若网站需要公开文章，只授予公开读取的必要字段，并在 CMS 权限规则中加 `status = published` 过滤；只在前端查询中添加过滤不能保护草稿。

## DNS、HTTPS 与前端发布

本期不自动改写 Nginx、DNS 或申请证书。CMS 域名反向代理到分配的本机端口（含 WebSocket），网站前端静态目录对应 www；DNS 指向服务器并配置 HTTPS 后才能从公网访问。

配置 CMS 前端允许来源时使用网站 origin（协议 + 域名 + 端口），不包含子路径。如部署后修改网站/CMS 地址，需要平台同步容器配置并重启，界面改地址不会自动改 PUBLIC_URL/CORS。

前端接入：

```ts
import { createDirectus, rest, readItems } from '@directus/sdk'

const cms = createDirectus('https://cms.example.com').with(rest())
const articles = await cms.request(readItems('site_articles', {
  fields: ['id', 'title', 'slug', 'summary', 'published_at'],
  filter: { status: { _eq: 'published' } },
  sort: ['-published_at'],
}))
```

网页端不要放 CMS 管理员 token。HTML 正文应经过可信渲染/清洗后使用。

## 运维与验收范围

平台可通过交互终端查看 `docker compose -p pm-site-<ID> -f /etc/pet-manager/sites/<ID>/compose.json ps`，检查日志、备份和启停 CMS，普通账号需自行使用 sudo。文件维护停用、服务器停用和租户停用会拦截本平台后续维护请求；独立 Directus 不受平台会话控制，需单独禁用账号/停止实例。

网站台账和远程文件、容器、数据卷不随租户数据库销毁自动删除，保留供平台处理和备份。销毁租户前应先通过 SSH 回收或停用 CMS，移除对应 SFTP 公钥，再处理持久化数据。当前未实现远程资源销毁/自动备份和 CMS 升级按钮。

运行中断的部署状态会在后续启动/列表请求中回收超过 10 分钟的旧任务，以失败状态重试。系统不会删除持久化卷。

验证命令：

```text
cd server
go test ./...
go vet ./...
node --test internal/modules/hosting/directus-bootstrap.test.cjs
cd ../admin
npm run lint
npm run build
```

本地已验证加密/凭证隐藏、租户查询限制、穿越与链接拒绝、SSH 指纹、Compose 隔离/凭证字面值和 CMS 初始化幂等。数据库集成测试默认跳过；真实 Linux chroot 和 Docker 部署仍需在测试服务器验收。
