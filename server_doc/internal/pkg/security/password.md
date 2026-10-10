# internal/pkg/security/password.go

> 脚手架导航：[回到开发者入口](../../../../README.md)

## 职责

口令散列与随机串生成，全站密码相关逻辑只允许用这里。

| 函数 | 说明 |
| --- | --- |
| `HashPassword(plain)` | bcrypt（DefaultCost）散列，失败包装错误上抛 |
| `VerifyPassword(hash, plain)` | 常量时间比对，返回 bool |
| `RandomToken(n)` | `crypto/rand` 生成 URL-safe base64，用于 request_id / sid |

## 约定

- 密码字段 JSON 序列化必须 `json:"-"`（见各账号模型），任何日志/审计不得出现明文；
- 不用 md5/sha 自研方案，不加密码"盐"参数（bcrypt 内置）；
- `RandomToken` 读取系统随机源失败直接 panic（环境级故障，无降级意义）。
