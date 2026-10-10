# internal/config/config.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [config.yaml 说明](../../configs/config.md)

## 职责

定义全站配置树 `Config` 并用 viper 加载：YAML 文件 + `PM_` 环境变量覆盖 + 代码内默认值，三层合并。

## 结构

```
Config
├── AppConfig      name / env(dev|test|prod) / addr；IsProd() 控制日志模式与 Swagger
├── LogConfig      level / format(console|json) / output(stdout|stderr)
├── DatabaseConfig 连接池参数 + platform_schema；DSN() 固定平台 search_path
├── RedisConfig    addr / password / db / pool_size / key_prefix
├── JWTConfig      secret / issuer / access_ttl / refresh_ttl
├── CORSConfig     allow_origins 白名单
├── SeedConfig     platform_admin_username / password
├── JobsConfig     shell_path
└── TOSConfig      平台对象存储连接参数、签名期限与上传上限
```

## Load 行为

- `path` 为空：按文件名 `config` 搜索 `./configs`、`.`；
- `SetEnvPrefix("PM")` + `.` → `_` replacer + `AutomaticEnv()`；
- 配置文件不存在且未显式指定路径时允许启动（纯环境变量部署）；
- 所有默认值集中在 `defaults(v)`。

## 硬性约定

- 新增配置项**必须**在 `defaults()` 里 `SetDefault`：viper 的 `AutomaticEnv` 只能覆盖已注册的 key，漏注册会导致环境变量被静默忽略（`PM_REDIS_PASSWORD` 的既有教训）；
- 敏感项（密码、JWT secret）默认值仅用于本地，生产必须用环境变量覆盖。

## 统一存储与附件

TOSConfig 接纳 enabled、endpoint、region、access_key、secret_key、bucket、key_prefix、public_base_url、presign_ttl、max_upload_bytes，全部 defaults 注册。默认停用，签名 10 分钟，单文件 100 MiB。访问密钥仅服务端配置；套餐等级和容量写平台 storage.plan 字典。

## hosting

HostingConfig.EncryptionKey 支持 PM_HOSTING_ENCRYPTION_KEY；defaults 注册空字符串保证 Viper 环境变量覆盖生效。必须是稳定的 32 字节随机密钥的 Base64 值，不与 JWT 密钥复用。
