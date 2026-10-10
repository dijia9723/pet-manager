# managed_server.go

平台表 sys_managed_server：主机、端口、SSH 用户、认证类型、SHA256 主机指纹、启停状态及备注。Credential 为 AES-256-GCM 密文，不参与 JSON 序列化。

模型在 PlatformModels 登记。有关联网站时不能删除，也不能改主机、端口或指纹；更新空凭证保留当前已保存凭证。配置与接入见 [hosting.md](../../../doc/hosting.md)。
