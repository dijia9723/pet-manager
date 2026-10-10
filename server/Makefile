# 后端常用命令。约定：所有命令都在 server/ 目录下执行。
APP        ?= pet-manager-api
CONFIG     ?= ./configs/config.yaml
COMPOSE    ?= docker compose -f ../deploy/docker-compose.yml

.PHONY: help infra-up infra-down infra-logs run build fmt vet test test-int tidy clean swag swag-fmt

help:
	@echo "infra-up   启动 PG/Redis/MinIO"
	@echo "infra-down 停止本地依赖"
	@echo "run        本地启动 API"
	@echo "build      编译到 bin/"
	@echo "fmt        格式化"
	@echo "vet        静态检查"
	@echo "test       跑全部单测（无外部依赖）"
	@echo "test-int   跑集成测试（需要 PG + Redis，见下方说明）"
	@echo "swag       重新生成 Swagger 文档到 docs/"
	@echo "swag-fmt   格式化 Swagger 注解"

infra-up:
	$(COMPOSE) up -d

infra-down:
	$(COMPOSE) down

infra-logs:
	$(COMPOSE) logs -f

run:
	go run ./cmd/api -c $(CONFIG)

build:
	go build -o bin/$(APP) ./cmd/api

fmt:
	gofmt -l -w ./internal ./cmd

vet:
	go vet ./...

# 单元测试不依赖外部服务，CI 与本地都可直接跑
test:
	go test ./internal/... -count=1

# 集成测试会真连数据库与 Redis，验证"租户互不可见""越权返回 403"这类硬约束。
# 默认跳过，需要显式开启：
#   PM_TEST_INTEGRATION=1 go test ./internal/tests/... -v
test-int:
	PM_TEST_INTEGRATION=1 go test ./internal/tests/... -count=1 -v

tidy:
	go mod tidy

# 重新生成 Swagger 文档（新增/修改接口注解后执行）。
# 首次使用需安装 CLI：go install github.com/swaggo/swag/cmd/swag@latest
# Windows 下若 swag 不在 PATH，用 "$(go env GOPATH)\bin\swag.exe" 替换。
# 生成物 docs/ 已提交，没装 swag 也能直接编译运行。
swag:
	swag init -d cmd/api,internal -g main.go -o docs --parseInternal

# 统一格式化注解写法（对齐 @Summary/@Param 等）
swag-fmt:
	swag fmt -d cmd/api,internal

clean:
	rm -rf bin