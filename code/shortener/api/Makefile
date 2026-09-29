# api 目录的常用命令入口。
#
# 用法：在本目录执行 make 目标名，例如 make build。
# 直接执行 make 会打印全部可用目标。
#
# 关于缩进：Makefile 的配方行（也就是命令本身那一行）必须以制表符开头，不能使用空格。
# 修改本文件时请保持制表符缩进，否则 make 会报 missing separator 并拒绝执行。
#
# 关于变量覆盖：下面定义的变量都可以在命令行覆盖，
# 例如 make run HTTP_ADDR=:9090 会用 9090 端口启动服务。

# 默认目标。执行不带参数的 make 时进入这一条。
.DEFAULT_GOAL := help

# 变量定义。?= 的含义是「环境里没有设置时才使用这里的取值」。
GO ?= go
# 本机无法访问 proxy.golang.org，因此模块代理必须指向国内镜像。
GOPROXY ?= https://goproxy.cn,direct
HTTP_ADDR ?= :8080
TEST_DATABASE_URL ?= postgres://shortener:shortener_dev_password@localhost:5432/shortener?sslmode=disable
TEST_REDIS_ADDR ?= localhost:6379

# 把 GOPROXY 导出给每一条配方使用，这样不需要在每个终端里手工 export。
export GOPROXY

# 声明这些名字不是文件，因此即使目录下存在同名文件也仍然执行对应的配方。
.PHONY: help build vet fmt fmt-check test test-unit test-store test-cache test-all run tidy clean compose-ps compose-up compose-down compose-logs verify

help: ## 打印全部可用目标
	@echo "可用目标："
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk -F ':.*?## ' '{printf "  %-16s %s\n", $$1, $$2}'
	@echo ""
	@echo "示例：先执行 make build，再执行 make test-unit。"

build: ## 编译全部包，检查语法与类型，不产生输出文件
	$(GO) build ./...

vet: ## 静态检查，可以发现格式化字符串参数个数不匹配这类问题
	$(GO) vet ./...

fmt: ## 按 Go 官方格式重排全部源文件
	gofmt -w .

fmt-check: ## 检查格式，列出需要重排的文件，全部合规时不输出文件名
	@files="$$(gofmt -l .)"; if [ -n "$$files" ]; then echo "以下文件需要执行 make fmt："; echo "$$files"; exit 1; fi; echo "格式检查通过"

test: ## 运行全部包的测试，需要依赖的两组在缺少环境变量时会被跳过
	$(GO) test ./...

test-unit: ## 只运行不需要外部依赖的两组测试，也就是短码生成与路由
	$(GO) test ./internal/shortcode/... ./internal/httpapi/... -v

test-store: ## 运行需要真实 PostgreSQL 的存储层测试
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' $(GO) test ./internal/store/... -v

test-cache: ## 运行需要真实 Redis 的缓存层测试
	TEST_REDIS_ADDR='$(TEST_REDIS_ADDR)' $(GO) test ./internal/cache/... -v

test-all: ## 运行全部测试，包含需要真实 PostgreSQL 与 Redis 的两组
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' TEST_REDIS_ADDR='$(TEST_REDIS_ADDR)' $(GO) test ./... -v

run: ## 启动后端服务
	HTTP_ADDR='$(HTTP_ADDR)' $(GO) run ./cmd/api

tidy: ## 整理依赖，根据源码里的导入维护 go.mod 与 go.sum
	$(GO) mod tidy

clean: ## 删除编译产物，并且清空测试缓存，使下一次测试真正重新执行
	rm -f api
	$(GO) clean -testcache
	@echo "已清理"

compose-ps: ## 查看 PostgreSQL 与 Redis 两个依赖服务的状态
	cd .. && docker compose ps

compose-up: ## 启动 PostgreSQL 与 Redis 两个依赖服务
	cd .. && docker compose up -d

compose-down: ## 停止并删除 PostgreSQL 与 Redis 两个容器
	cd .. && docker compose down

compose-logs: ## 同时查看两个依赖服务的日志
	cd .. && docker compose logs -f

verify: build vet fmt-check test-unit ## 一次跑完编译、静态检查、格式检查与不需要依赖的测试
	@echo "verify 全部通过"
