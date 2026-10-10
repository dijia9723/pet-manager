# handler.go

统一 response.Body 响应，ID 必须为正整数。JSON 请求限制 2 MiB；文本内容限制 1 MiB。multipart 限制总请求大小，流读取最大 10 MiB，清理临时表单文件。

下载使用鉴权请求返回 application/octet-stream、attachment 和 nosniff。文件操作由网站 ID 映射到受限账号，不接受 tenant_id、服务器账号或绝对宿主机根目录作为文件操作授权依据。

Swag 注解覆盖平台和租户接口，生成物位于 server/docs。
