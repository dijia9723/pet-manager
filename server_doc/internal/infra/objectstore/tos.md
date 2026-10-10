# internal/infra/objectstore/tos.go

封装火山引擎官方 Go SDK v2.9.8（https://github.com/volcengine/ve-tos-golang-sdk），以 Store 接口提供 Put/Delete/Sign，业务模块不接触访问凭证。New 读取 TOSConfig；停用返回 nil，启用校验连接参数、1 秒到 7 天签名期限、1 字节到 1 GiB 上传上限，无协议 endpoint 补 https，单次 SDK 请求超时两分钟。

Put 写私有对象并携带下载文件名；Sign 强制 attachment / application/octet-stream。public_base_url 保留配置兼容，私有消息附件始终走临时签名。URL 仅在业务鉴权后下发，撤回后已签发 URL 在期限内仍可能有效。
