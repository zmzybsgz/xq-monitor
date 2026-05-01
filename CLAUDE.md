# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目本质

雪球组合持仓监控的 Go CLI 守护进程：定期轮询雪球 API → 与本地快照对比 → 通过 PushPlus 推送微信。无外部依赖（go.mod 全标准库），单二进制部署。

## 常用命令

```bash
make build         # 编译 → bin/xq-monitor
make run           # build + 常驻运行
make test          # go test ./... -v -count=1
make cover         # 覆盖率报告（func 级 + HTML 提示）
make lint          # go vet ./...
make tidy          # go mod tidy
make docker        # docker build -f deploy/Dockerfile

# 跑单个包/单个测试
go test ./internal/snapshot/ -v -run TestDiff_Mixed
go test ./internal/logger/ -run TestGzipFile -v
```

入口：`cmd/xq-monitor/main.go`（带版本注入：`-X main.version=$(git describe)`）。

## 架构关键点

### 启动 → 第一次 poll → 永久循环（main.go）

启动期做了三件事，顺序很重要：
1. 抓取所有组合的持仓 → 缓存到 `startupHoldings`（按 portfolio ID）和 `summaryHoldings`（按 display name）
2. 推送启动概览（`SendStartupSummary`）
3. **第一次 `poll(...)` 把 `startupHoldings` 作为 `preloaded` 传入** —— `monitorPortfolio` 检测到对应 ID 已有数据，跳过 API 抓取，避免短时间内重复请求雪球

之后进入 `for { time.Sleep; TryReload; sendDailyHeartbeat?; poll(nil) }` 永久循环。`preloaded == nil` 是后续轮的标记。

### 配置热加载（config.TryReload）

基于 `os.Stat().ModTime()` 与上次加载时间比较，**变更时整个 `*xueqiu.Client` 会被重建**（main.go 第 220-222 行）。这是为什么 `Client` 持有的匿名 Cookie 缓存（`resolveCookie`）不会跨配置生效——重建 = 缓存自然清空。

### 快照对比的两层短路

1. `snapshot.Fingerprint`（MD5）：完全一致 → 不进 Diff
2. `snapshot.Diff` 在 `math.Round((delta)*100)/100` 截断后用 `>= threshold` 判断；细微浮点抖动（如 50.001→50.002）不会触发

### "通知成功后才更新快照"的语义

`monitorPortfolio` 在 `notify.SendWechat` 返回错误时**不写快照**（直接 return），下一轮会再发现同一变化重试推送。这是有意的"宁可重复推、不要漏推"权衡。如果你修改这段顺序，要清楚自己在改什么。

### Cookie 失效保护

`monitorPortfolio` 中：`len(current) == 0 && len(previous) > 0` → 跳过本轮且不更新快照。雪球 Cookie 失效时返回空持仓，没有这个保护会被当作"清仓"误推一条全部移除的通知。

### 抓取失败的限流告警

`poll` 收集本轮所有 `GetHoldings` 失败的组合到 `fetchErrors`，**全局每小时最多发一条**（`lastErrNotify` 在 `main` 中持久化）。多组合同时失败合并为一条 `SendFetchErrors`，避免 Cookie 过期时雪崩式推送。

### 每日心跳

`main` 启动时若已过 8 点，把 `lastHeartbeatDate` 设为今天 → 跳过当天心跳（启动通知已覆盖）。否则当天 8 点首次循环触发 `sendDailyHeartbeat`。

### HTTP 重试（xueqiu.doWithRetry）

最多 3 次，指数退避（1s/2s/4s）。**每次重试 `req.Clone(ctx)`**——Go 标准库不允许复用同一 `*http.Request`（连接池会异常）。仅对 5xx 和 429 重试，4xx 直接失败。

### 日志切割（logger.go）

`Setup` 后台 goroutine 每天 0:00:01 调 `openToday`，关掉旧文件、打开新文件、重设 `log.SetOutput(MultiWriter(stdout, file))`。压缩用临时文件 + `Rename` 保证原子（避免崩溃留下损坏 `.gz`）。`compressOldFiles` / `cleanOldFiles` 是无状态纯函数，便于单测注入临时目录。

## 模块约定

- **无循环依赖**：所有 `internal/*` 包只反向依赖 `internal/model`；`main` 是唯一串接点。
- **`xueqiu.Client` 是具体类型**，`monitorPortfolio` 直接接受 `*xueqiu.Client`（无接口）。如果要给主流程加单测，需要先抽 `HoldingsFetcher` 接口（见 plan.md P2）。
- **测试不发真实 HTTP**：`xueqiu` 包导出小写的 `parseCurrentResponse` / `parseQuoteResponse` 仅给同包测试用。
- **`internal/model` 没有测试文件**是有意的：纯 struct + 一个 `HasChanges()`，没有逻辑要测。

## plan.md

项目根的 [plan.md](plan.md) 维护：里程碑、按 P1/P2/P3 分级的待跟进项、踩坑记录。**修了 bug 或踩了新坑要顺手更新这里**——是项目硬约定。

## 时区相关

代码里 `time.Now()` 未显式 `LoadLocation`。Docker 部署通过 `ENV TZ=Asia/Shanghai`（deploy/Dockerfile）规避；裸机 systemd 部署需服务器自己是 `Asia/Shanghai`。如果要修，统一在 `main` 启动时 `time.Local = time.LoadLocation("Asia/Shanghai")`，别在每个 `time.Now()` 处加 `.In(...)`。

## 已忽略的目录

`.gitignore` 里：`config.json`（含 cookie/token）、`snapshots/`（运行时数据）、`logs/`、`bin/`、`coverage.out`、`.env`、`.claude/`。新增运行时产物记得也加进去。
