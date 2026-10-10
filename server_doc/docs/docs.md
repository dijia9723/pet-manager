# docs/ — Swagger 生成物（请勿手改）

> 脚手架导航：[回到开发者入口](../../README.md) · [swag Makefile 目标](../../server/Makefile)

> 本文档镜像源码位置 `server/docs/`（生成物 docs.go / swagger.json / swagger.yaml 仍在代码目录，勿手改）。

本目录由 [swag](https://github.com/swaggo/swag) 根据代码注解自动生成：

- `docs.go` / `swagger.json` / `swagger.yaml`：OpenAPI 2.0 定义；
- 由 [bootstrap](../internal/bootstrap/app.md) 匿名导入 `_ "pet-manager/server/docs"` 注册，缺失会导致 `/swagger/doc.json` 404/500。

## 重新生成

```bash
# 在 server/ 目录，需要 go install github.com/swaggo/swag/cmd/swag@latest
make swag          # 等价 swag init -d cmd/api,internal -g main.go -o docs --parseInternal
make swag-fmt      # 格式化注解
```

生成物**已提交**：未安装 swag CLI 也能正常编译运行。只在修改了 `@Summary/@Param/@Router` 等注解后才需重新生成。

当前已包含单租户立即销毁 `POST /platform/tenants/{id}/destroy`，请求体 `tenant.DestroyInput`，需要 BearerAuth 和平台专属权限。生成后应对照实际注册路由验证操作数一致。
