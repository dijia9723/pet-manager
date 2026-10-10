# auth_test.go

验证浏览器 WebSocket 从 bearer.<token> 子协议读取凭证；普通请求忽略该子协议，Authorization 优先，原 query 凭证兼容性保持。hosting 路由测试另验证真实租户 JWT 经该子协议仍无法进入平台终端。
