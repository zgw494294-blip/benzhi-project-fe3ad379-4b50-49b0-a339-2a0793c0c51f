# BENZHI_README

基于 Go 实现的sensor-calibration-release HTTP API 项目，一款后端服务，实现了环境监测传感器部署前校准放行服务，覆盖建批、方案锁定、多点采样判定、返校复验、独立审核、冻结签发、持久化恢复与审计查询。

## 项目说明
- 项目：benzhi-project-fe3ad379-4b50-49b0-a339-2a0793c0c51f
- 项目用途：实现了环境监测传感器部署前校准放行服务，覆盖建批、方案锁定、多点采样判定、返校复验、独立审核、冻结签发、持久化恢复与审计查询。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck -selfcheck-timeout=15s
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-fe3ad379-4b50-49b0-a339-2a0793c0c51f-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-fe3ad379-4b50-49b0-a339-2a0793c0c51f-arm64 linux/arm64
docker run -it benzhi-project-fe3ad379-4b50-49b0-a339-2a0793c0c51f-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck -selfcheck-timeout=15s`
