# internal/pkg/jwtx/jwt.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [middleware/auth.md](../../middleware/auth.md)

## 职责

登录凭证的签发与校验（HS256），包名刻意与上游 `github.com/golang-jwt/jwt` 区分。

## Claims 载荷

```
uid / uname / utype(platform|tenant)
tid / tcode / tschema   // 租户身份才有；tschema 随凭证下发免回查，但仍会被 Guard 再校验
typ(access|refresh)     // 拒绝拿 refresh token 调业务接口
sid                     // 会话 ID，Redis 注销/强制下线的依据
+ RegisteredClaims(iss/iat/nbf/exp)
```

## API

| 方法 | 说明 |
| --- | --- |
| `New(JWTConfig)` | secret/issuer/双 TTL |
| `Sign(claims, typ)` | 签发，返回 token 与有效期秒数 |
| `Parse(raw)` | 强制 issuer + 仅 HS256；过期 → `20003`，其余无效 → `20002` |
| `AccessTTL()/RefreshTTL()` | 会话存储 TTL 取 refresh 有效期 |

## 折中说明

JWT 本身无状态，但每个 token 带 `sid` 且 Auth 中间件必查 Redis 会话，因此登出/踢下线能立即生效，代价是每请求一次 Redis 读。
