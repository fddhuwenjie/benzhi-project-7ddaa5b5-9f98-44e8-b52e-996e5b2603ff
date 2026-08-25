# BENZHI_README

## 项目说明
- 项目：benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff
- 项目用途：完整实现古树迁移方案联合审查台，以 Go 单进程 HTTP 服务提供浏览器工作台、确定性技术规则、双专业会审、整改复核、批准归档、本地 JSON 快照、审计、乐观并发和持久化幂等能力。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：古树迁移方案联合审查台
- 项目概述：面向城市绿化技术人员的古树迁移方案审查应用，以迁移个案为聚合根，将基础资料、规则校验、专业意见、整改版本和批准档案串成一个可追溯闭环。项目采用 Go 单进程 HTTP 服务和原生浏览器资源，项目根目录包含简体中文 README.md，说明用途、标准构建、运行和测试方式。
- 核心工作流：迁移方案从草稿录入开始，经规则校验后提交联合会审，会审形成修改意见，编制人员提交整改版本，审查人员完成复核并批准归档；状态依次为 draft、validated、under_review、revision_required、resubmitted、approved。
- 对外接口：由 Go 服务提供原生 HTML、CSS 和 JavaScript 的浏览器审查工作台，覆盖个案编辑、规则结果、会审意见、版本对比和批准档案页面；监听地址支持 -addr=127.0.0.1:<port>，默认使用 127.0.0.1:19081，并允许 PORT 为端口号时绑定 127.0.0.1:<PORT>，不得默认绑定 0.0.0.0、8080、80 或 3000。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selftest -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff-arm64 linux/arm64
docker run -it benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selftest -addr=127.0.0.1:19081`
