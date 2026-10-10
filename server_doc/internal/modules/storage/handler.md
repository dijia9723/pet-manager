# internal/modules/storage/handler.go

存储 HTTP 使用 response.OK/Fail 和 errcode，具名 ListQuery/AllocationInput，Swagger 覆盖全部路由。idParam 校验正整数。

上传限制 multipart 请求体为单文件上限+1 MiB，8 MiB 后使用临时文件，结束 RemoveAll。内容类型由前 512 字节探测，大小来自解析文件，拒绝空文件、异常名称和超限文件。返回临时下载地址，不向浏览器提供密钥。删除暂时失败返回 503 和待清理提示，独立持久化的台账交由后台重试，前端刷新显示待清理。
