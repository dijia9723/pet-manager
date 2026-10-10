# terminal_test.go

使用本机 SSH 测试服务与真实 WebSocket 协议验证 PTY 初始化、resize、中文/emoji、Ctrl+C、Tab、方向键、二进制输入与退出码。验证浏览器断开或无效 JSON/尺寸/类型会释放 SSH 连接；不可信 Origin 在 SSH 连接前拒绝，PTY 拒绝与主机 pin 不匹配通过控制事件显示具体原因。

该测试服务模拟 SSH Shell 数据通道，不替代远程 Linux 上 vim、sudo 等交互程序验收。
