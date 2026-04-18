# xq-monitor

雪球组合持仓监控工具。自动检测目标组合的调仓变动（新增、移除、仓位调整），并通过微信推送通知，附带具体买卖股数建议。

## 功能

- 监控多个雪球组合的持仓变动
- 检测新增持仓、移除持仓、仓位调整（可配置阈值）
- 根据总投资金额计算具体买卖股数（向下 100 股取整）
- 通过 PushPlus 推送微信通知
- 支持常驻轮询模式和单次运行模式
- 配置文件热加载（修改 config.json 无需重启）

## 快速开始

### 1. 编译

```bash
make build
```

### 2. 配置

复制配置模板并填写：

```bash
cp configs/config.example.json config.json
```

编辑 `config.json`：

```json
{
  "portfolios": [
    {"id": "ZH1004909", "name": "剑客2017"}
  ],
  "xueqiu_cookie": "浏览器登录雪球后复制 Cookie",
  "pushplus_token": "去 pushplus.plus 获取",
  "weight_change_delta": 0.5,
  "total_amount": 500000,
  "interval": 300
}
```

| 字段 | 说明 |
|------|------|
| `portfolios` | 要监控的雪球组合列表 |
| `xueqiu_cookie` | 雪球登录 Cookie（必填） |
| `pushplus_token` | PushPlus 推送令牌（不填则只在终端输出） |
| `weight_change_delta` | 仓位变动阈值（%），低于此值不触发通知 |
| `total_amount` | 总投资金额（元），用于计算买卖股数，0 则不计算 |
| `interval` | 轮询间隔（秒），0 表示单次运行 |

敏感字段也可通过环境变量注入：`XUEQIU_COOKIE`、`PUSHPLUS_TOKEN`。

### 3. 运行

```bash
# 常驻模式（按 interval 定时轮询）
make run

# 单次运行
make run-once
```

## 获取雪球 Cookie

1. 浏览器登录 [雪球](https://xueqiu.com)
2. 按 F12 打开开发者工具 → Network
3. 刷新页面，找到任意请求，复制 Request Headers 中的 `Cookie` 值

## 项目结构

```
├── cmd/xq-monitor/        入口
├── internal/
│   ├── config/             配置加载、热加载
│   ├── model/              数据结构
│   ├── xueqiu/             雪球 API 客户端
│   ├── snapshot/            快照存储与持仓对比
│   ├── trade/              买卖建议计算
│   └── notify/             PushPlus 微信通知
├── configs/                配置模板
├── deploy/                 Dockerfile、systemd 服务文件
└── Makefile
```

## 常用命令

```bash
make build      # 编译
make run        # 常驻模式运行
make run-once   # 单次运行
make test       # 运行测试
make cover      # 测试覆盖率
make lint       # 静态检查
make docker     # 构建 Docker 镜像
make clean      # 清理
```

## 部署

### Docker

```bash
make docker
docker run -v $(pwd)/config.json:/app/config.json \
           -v $(pwd)/snapshots:/app/snapshots \
           xq-monitor:dev
```

### systemd

```bash
sudo cp deploy/xq-monitor.service /etc/systemd/system/
sudo systemctl enable --now xq-monitor
```

## License

MIT
