# cmd/api/main.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [bootstrap/app.md](../../internal/bootstrap/app.md) · [config/config.md](../../internal/config/config.md)

## 职责

整个后端的**唯一可执行入口**（`package main`）。只做三件事：

1. 解析 `-c` 命令行参数（配置文件路径，缺省按 `./configs/config.yaml` 查找）；
2. 调 `config.Load` 读配置、`bootstrap.New` 完成全部装配；
3. 阻塞运行 HTTP 服务，收到 `SIGINT/SIGTERM` 后在 15s 超时内**优雅退出**。

## 启动方式

```bash
# 在 server/ 目录下
go run ./cmd/api -c ./configs/config.yaml
# 或 make run
```

所有配置项都支持 `PM_` 前缀的环境变量覆盖，例如 `PM_DATABASE_HOST`、`PM_SEED_PLATFORM_ADMIN_PASSWORD`。

## Swagger 通用注解

文件头的 `@title/@host/@BasePath/@securityDefinitions` 等注解由 [swag](https://github.com/swaggo/swag) 解析，生成物在 [server/docs/](../../../server/docs/)（已提交）。改注解后执行 `make swag` 重新生成。

- 文档地址（仅 dev/test）：`http://localhost:8088/swagger/index.html`
- 生产环境（`app.env=prod`）不注册 Swagger 路由，见 [bootstrap/app.md](../../internal/bootstrap/app.md)。

## 约定

- 入口保持薄：业务装配逻辑全部在 `bootstrap`，这里不允许出现初始化细节；
- 启动失败直接打印到 stderr 并 `os.Exit(1)`，多租户系统不做带病降级。
