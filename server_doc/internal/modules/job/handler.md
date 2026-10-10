# internal/modules/job/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [service.md](service.md)

任务管理的 HTTP 层，接口均为平台端 + BearerAuth + 对应权限点（见 [module.md](module.md)），Swagger 分组「平台-任务管理」。

## 接口要点

- `GET /platform/jobs`：返回全量 `[]Item`（系统任务数量少，不分页）；
- `GET /platform/jobs/methods`：返回已注册方法及服务器 sh 可用性；
- `POST /platform/jobs`：新建 system/http/shell 任务，配置示例见 [service.md](service.md)；编码冲突返回 **409**，配置错误返回 **400**；
- `DELETE /platform/jobs/{id}`：仅允许已停用且未执行中的自定义任务，软删除保留日志；预置任务返回 **403**；
- `PUT /platform/jobs/{id}`：必填 spec，可修改 remark、name、kind、config、timeout_seconds；预置任务只允许修改调度、超时、备注；cron 解析失败返回 **400**；
- `POST .../enable|disable`：空 data 成功响应；
- `POST .../run`：先抢锁和写入 running 日志，再异步执行；任务或同一系统方法正在执行返回 **409**；配置失效记录失败日志；手动执行不修改原调度时间；
- `GET .../logs`：分页 `response.Page[LogItem]`，query 支持 page/page_size/status(running·success·fail)/trigger(schedule·manual)。

## 约定

- 与其它模块一致：参数错误走 `errcode.ErrInvalidParam`，不存在走 `errcode.ErrNotFound`，统一 `response.OK/Fail`；
- `jobPathID` 是本模块的路径参数解析（与 tenant 模块的 pathID 同构，因包不同各自保留一份）。
