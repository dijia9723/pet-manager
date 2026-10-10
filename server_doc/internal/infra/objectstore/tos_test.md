# internal/infra/objectstore/tos_test.go

覆盖启用配置校验、停用兼容、签名期限边界和下载参数。PM_TEST_TOS=1 / PM_TEST_CONFIG 启用实际 TOS 测试：仅上传随机 integration-tests 对象，通过签名下载验证内容并 cleanup 删除自己的测试对象，不扫描或操作已有文件。
