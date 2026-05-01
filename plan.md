# xq-monitor 项目计划

记录项目的开发计划、待跟进项与开发过程中踩过的坑。功能细节见 [README.md](README.md)。

## 项目目标

监控雪球组合调仓，将新增/移除/仓位变动通过 PushPlus 推送到微信，附带按总投资金额计算的具体股数建议。常驻进程，配置热加载。

## 已完成里程碑

| 时间 | 内容 |
|------|------|
| 2026-04-17 | 初版：组合持仓抓取、快照对比、PushPlus 通知 |
| 2026-04-18 | 默认轮询间隔改为 30s |
| 2026-04-19 | 启动时推送持仓概览（验证推送通道） |
| 2026-04-19 | 日志按天切割、gzip 压缩、保留 7 天 |
| 2026-05-01 | 每日 8:00 心跳播报、Cookie 失效汇总告警 |
| 2026-05-01 | 修复审查问题：Go 版本对齐 1.24、logger 错误处理与单测、补 plan.md |

## 待跟进（按优先级）

### P1 - 值得做

- **`snapshot.Save` 改为原子写入**（临时文件 + Rename，与 `gzipFile` 的实现一致）。当前进程被 SIGKILL 或掉电时，快照文件可能写入一半，下次启动 `json.Unmarshal` 失败，导致首次运行逻辑被跳过。
- **强制使用 `Asia/Shanghai` 时区**：在 `main` 启动时 `time.LoadLocation("Asia/Shanghai")` 并替换 `time.Now()` 的所有调用，避免裸机 systemd 部署在 UTC 时区的服务器上心跳与日志切割时间错位。Docker 部署已通过 `ENV TZ=Asia/Shanghai` 规避。

### P2 - 可选改进

- **抽象 `xueqiu.HoldingsFetcher` 接口**（`GetHoldings` + `GetStockPrices`），让 `monitorPortfolio` / `sendDailyHeartbeat` 接受接口而非具体 `*Client`，便于注入 stub 覆盖空持仓防护、首次运行等主流程分支的单元测试。
- **`notify.SendStartupSummary` / `SendDailySummary` 补 token 为空分支的单元测试**，与 `SendFetchErrors_EmptyToken` 风格一致。
- **`config.Load` 校验 `TotalAmount < 0`**，按 `WeightChangeDelta`、`Interval` 同样的方式重置为 0，避免负数总金额导致通知里出现"目标金额 -xxxxx 元"。
- **`notify.buildHoldingsHTML` / `SendFetchErrors` 中将 `err.Error()` 用 `html.EscapeString` 转义后再插入**，防止雪球错误响应体中含 `<` 等 HTML 特殊字符破坏 PushPlus 渲染。

### P3 - 仅需关注

- 如果将来需要并发抓取多个组合：`xueqiu.Client.resolveCookie` 锁内同步执行 HTTP 请求，会阻塞所有调用。当前是串行轮询，没问题。
- 如果将来需要支持多通知渠道（Telegram/邮件）：再考虑抽 `Notifier` 接口；当前单一 PushPlus 不必。

## 踩过的坑

| 现象 | 原因 | 解决 |
|------|------|------|
| 抓到空持仓但历史非空时误清快照 | Cookie 失效返回空数组，被当作"清仓" | `monitorPortfolio` 在 `len(current)==0 && len(previous)>0` 时跳过本轮，不更新快照 |
| 仓位 50.00→50.01 也触发通知 | 浮点累积误差被当作真实变动 | `snapshot.Diff` 用 `math.Round((delta)*100)/100` 截断到两位小数后再与阈值比较 |
| 通知发出后程序崩溃，再次启动重复发 | 通知成功后才更新快照，但中间崩溃会导致下次比对仍命中 | 当前接受这一权衡：宁可重复推送也不漏推；如需进一步降低重复率，可在通知前持久化 pending 状态 |
| `http.Request` 重试时连接池异常 | Go 标准库不允许同一 `*http.Request` 被复用 | `xueqiu.doWithRetry` 每次重试用 `req.Clone(ctx)` 生成新请求 |
| 日志压缩中途崩溃留下损坏 .gz | 直接 gzip 写到目标路径 | `gzipFile` 先写 `*.gz.tmp` 再 `os.Rename`，保证原子性 |
| 启动通知 + 当天 8 点心跳重复推送 | 两者内容几乎一致 | 启动时若已过 8 点，记下 `lastHeartbeatDate=今天`，跳过当天心跳 |

## 部署

参考 [README.md](README.md) 的"部署"章节。云服务器（Asia/Shanghai 时区）使用 Docker 或 systemd 任一方式即可；裸机 systemd 部署需确认服务器时区是 `Asia/Shanghai`，否则参考"P1 强制时区"。
