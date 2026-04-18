APP     := xq-monitor
CMD     := ./cmd/xq-monitor
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run run-once test cover lint clean docker help

## build: 编译二进制
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) $(CMD)

## run: 常驻模式运行
run: build
	./bin/$(APP)

## run-once: 单次运行
run-once: build
	./bin/$(APP) --once

## test: 运行全部单元测试
test:
	go test ./... -v -count=1

## cover: 运行测试并生成覆盖率报告
cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo "---"
	@echo "HTML 报告: go tool cover -html=coverage.out"

## lint: 静态检查
lint:
	go vet ./...

## docker: 构建 Docker 镜像
docker:
	docker build -f deploy/Dockerfile -t $(APP):$(VERSION) .

## clean: 清理构建产物
clean:
	rm -rf bin/ coverage.out

## help: 显示帮助
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
