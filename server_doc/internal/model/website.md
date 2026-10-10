# website.go

平台表 sys_website 保存网站分配关系、网站/CMS 地址、目录与 CMS 部署状态、分配端口和版本。TenantID/ServerID 建立后不可变。SFTPKey、CMSSecret 仅加密保存，不序列化。

跨租户查询必须按认证上下文 TenantID 限制。各网站使用独立系统 SFTP 账号、chroot 和 Directus 项目。server_id + cms_port 的部分唯一索引防止已分配端口冲突；0 不参与唯一性约束。

状态仅控制应用内维护入口，公网 CMS 生命周期另行管理。租户销毁不自动回收远程资源，台账保留供平台运维处理。
