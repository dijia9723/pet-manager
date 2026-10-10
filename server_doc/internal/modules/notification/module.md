# notification/module.go — 路由登记

Service 实现 router.Module，在 bootstrap 构造并挂载。

平台通知治理 `/platform/notifications`：list（GET 列表/回执）、create（POST 草稿、PUT 草稿、GET 接收对象）、publish（POST 发布）、revoke（POST 撤回），分别要求 platform.notification 下的按钮权限。

租户发布治理 `/tenant/notifications` 提供相同七种接口，分别要求 tenant.notification.list/create/publish/revoke 权限，挂在租户身份校验、门禁与请求事务后。发布对象仅本租户全部或指定启用账号；所有通知治理操作按发送主体租户隔离。

个人收件箱 `/platform/messages`、`/tenant/messages`：GET 列表、GET unread-count、POST {id}/read、POST {id}/process。个人收件箱无需业务角色权限，租户路由仍应用身份校验、租户门禁和请求事务。仅显示当前账号收到的已发布通知。

前端统一页面 `/messages` 与顶栏未读按钮；平台通知治理页面 `/platform/notifications`。租户消息中心为「收到的消息」与「发布消息」页签，后者要求 tenant.notification.list。主账号自动具备新权限，子账号需通过角色授权开放发布权限。

## 统一存储与附件

新增 /notifications/:id/attachments/:resource_id/download（notification.list）和 /messages/:id/attachments/:resource_id/download（本人接收快照）。平台和租户各 2 条，租户仍走 TenantScope。
