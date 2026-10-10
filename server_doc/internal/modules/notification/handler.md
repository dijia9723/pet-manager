# notification/handler.go — HTTP 协议

负责绑定具名 Input、ProcessInput、ListQuery、TargetQuery，使用统一 response/errcode 出口。所有路径 ID 必须为正整数，枚举和长度在绑定时校验，标题、正文和反馈不能仅含空格。

治理接口支持标题/状态筛选、分页接收回执、分页接收对象搜索。收件箱支持 unread/read/pending/processed 筛选，pending 仅包含需要处理且未处理的通知。

发布动作与草稿保存分开；客户端可先检查草稿再发布。接收对象在发布时重新验证，失效的指定目标会阻止整个发布事务。Swagger 注解与生成物由 swag init 同步。

平台与租户发布路由共用具名 DTO 和 handler。租户接口接收范围仅 own_tenant/tenant_users；接收对象 type 仅 tenant_users。拒绝越界发送返回 403 + 20005；跨租户通知 ID 返回 404。发送方类型、发送方租户和发送账号由服务从登录上下文确定，JSON 中同名伪造字段不会写入。

## 统一存储与附件

新增平台/租户管理与收件箱附件下载 4 个 Swagger 接口，解析通知 ID、resource_id，按管理模式或本人收件模式鉴权。统一返回 {url} 临时地址，已删除或无权限均返回 NotFound。
