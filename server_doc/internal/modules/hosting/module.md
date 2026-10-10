# module.go

hosting 模块注册平台 /servers、/websites，以及租户 /websites。平台 SSH 命令、WebSocket 交互终端和部署接口不在租户路由注册。终端 GET /servers/:id/terminal 使用 platform.server.exec 权限点。

文件浏览/下载使用 *.website.files，编辑、上传、mkdir/rename/delete 使用 *.website.write。租户身份与状态门禁仍由统一路由施加；使用 TransactionInService 避免持有跨网络的 schema 事务，网站分配台账是平台表，服务按认证 TenantID 显式过滤。
