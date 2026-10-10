// Command api 是宠物商户 SaaS 的后端入口。
//
// 启动：go run ./cmd/api -c ./configs/config.yaml
// 所有配置项都能用 PM_ 前缀的环境变量覆盖，例如 PM_DATABASE_HOST。
//
// Swagger 文档（仅 dev/test 环境开放）：服务启动后访问 http://localhost:8088/swagger/index.html
// 注解变更后需重新生成：make swag（或 swag init -d cmd/api,internal -g main.go -o docs）。
//
//	@title						宠物商户 SaaS 后端 API
//	@version					1.0
//	@description				宠物商户 SaaS 多租户底座脚手架接口文档。全站统一响应体 {"code","message","data"}，code=0 表示成功，业务错误码见 pkg/errcode。
//	@contact.name				pet-manager
//	@license.name				私有项目，未授权请勿使用
//	@host						localhost:8088
//	@BasePath					/api/v1
//	@schemes					http
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				登录/刷新接口返回的 access_token，填写格式：Bearer {access_token}
package main

import (
	"context"   // 优雅退出时构造带超时的上下文
	"flag"      // 解析 -c 启动参数
	"fmt"       // 格式化输出启动/退出信息
	"os"        // 进程退出码、标准错误、信号通道
	"os/signal" // 监听操作系统退出信号
	"syscall"   // SIGINT/SIGTERM 信号常量
	"time"      // 优雅退出的超时时长

	"pet-manager/server/internal/bootstrap" // 应用装配：把配置组装为可运行的 App
	"pet-manager/server/internal/config"    // 配置加载（配置文件 + PM_ 环境变量覆盖）
)

// shutdownTimeout 是优雅退出的最长等待时间，超时后强制结束，避免进程无法退出。
const shutdownTimeout = 15 * time.Second

// main 是进程入口，只负责解析参数与处理致命错误；
// 真正的启动逻辑放在 run 中，便于单独调用与测试。
func main() {
	// -c 指定配置文件路径；留空时由 config.Load 按约定路径查找
	configPath := flag.String("c", "", "配置文件路径，缺省按 ./configs/config.yaml 查找")
	flag.Parse()

	// run 返回错误即视为启动失败，打印到标准错误并以非 0 退出码结束进程
	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}

// run 完成应用的完整生命周期：
// 加载配置 -> 装配应用 -> 启动服务 -> 等待退出信号 -> 优雅关闭。
func run(configPath string) error {
	// 第 1 步：加载配置（以配置文件为主，PM_ 前缀的环境变量可覆盖）
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// 第 2 步：根据配置装配应用（数据库、Redis、路由、中间件、各业务模块等）
	app, err := bootstrap.New(cfg)
	if err != nil {
		return err
	}

	// 第 3 步：在独立 goroutine 中启动 HTTP 服务，主 goroutine 继续往下等待退出信号
	errCh := make(chan error, 1)
	go func() { errCh <- app.Run() }()

	// 监听退出信号：Ctrl+C 对应 SIGINT，容器/系统终止对应 SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞等待「服务自身出错退出」或「收到系统退出信号」两者之一
	select {
	case err := <-errCh:
		// 服务运行中自行返回错误，直接向上抛出
		return err
	case sig := <-stop:
		// 收到退出信号：为优雅关闭设置超时上限，关闭应用释放资源
		fmt.Printf("收到信号 %s，开始优雅退出\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return app.Shutdown(ctx)
	}
}
