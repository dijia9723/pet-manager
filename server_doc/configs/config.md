# configs/config.yaml

> 脚手架导航：[回到开发者入口](../../README.md) · [config 包](../internal/config/config.md)

## 职责

本地开发的默认配置文件。生产环境推荐**不挂载本文件**，全部改用 `PM_` 环境变量注入（viper 允许"只有环境变量"的无文件启动）。

## 配置段说明

| 段 | 关键字段 | 说明 |
| --- | --- | --- |
| `app` | `name` / `env` / `addr` | `env` 取 `dev/test/prod`；prod 会切换 gin ReleaseMode 并关闭 Swagger |
| `log` | `level` / `format` / `output` | console 带颜色，json 供容器采集 |
| `database` | `host..platform_schema` | 单连接池；DSN 的 `search_path` 固定平台 schema，租户切换一律走事务 |
| `redis` | `addr` / `password` / `db` / `key_prefix` | 会话、权限缓存、租户快照、限流都在这里 |
| `jwt` | `secret` / `access_ttl` / `refresh_ttl` | 生产必须覆盖 secret（默认 2h / 168h） |
| `cors` | `allow_origins` | 白名单精确匹配，未命中来源不放行 |
| `seed` | `platform_admin_username/password` | 首启种子账号，生产用 `PM_SEED_PLATFORM_ADMIN_PASSWORD` 覆盖 |

## 环境变量映射规则

viper key 中的 `.` 替换为 `_`，统一加 `PM_` 前缀：

```
database.password      -> PM_DATABASE_PASSWORD
redis.password         -> PM_REDIS_PASSWORD
seed.platform_admin_password -> PM_SEED_PLATFORM_ADMIN_PASSWORD
```

## 注意

- `redis.password` 必须在代码里显式 `SetDefault` 注册，否则 `PM_REDIS_PASSWORD` 会被 viper 静默忽略（项目既有教训，见 [config.go Load/defaults](../internal/config/config.md)）；
- 当前文件含开发库连接信息，**不要把生产凭据提交进来**；
- 修改默认值去 [config.go](../internal/config/config.md) 的 `defaults()`，两边语义保持一致。

## 统一存储与附件

tos 段提供平台统一对象存储配置；PM_TOS_* 可逐项覆盖。presign_ttl 默认 10m、有效 1 秒至 7 天，max_upload_bytes 默认 104857600（100 MiB）。public_base_url 保留，但私有消息附件走 TOS 临时签名。密钥不写业务字典、不下发前端。

## 服务器凭证配置

hosting.encryption_key 默认空，通过 PM_HOSTING_ENCRYPTION_KEY 注入。生成及备份流程见 doc/hosting.md。非空非法密钥拒绝启动，空密钥仅关闭新增服务器/网站的凭证写入。
