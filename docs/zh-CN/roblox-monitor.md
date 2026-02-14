# Roblox 入服监视

## 概览

- 订阅源：`site=roblox`
- 订阅类型：`type=join`
- 事件：入服（`enter`）与离服（`leave`）
- 入服链接策略：`gamejoin API` 优先，失败自动回退 deep link

## 命令示例

```text
/watch -s roblox -t join 123456
/watch -s roblox -t join builderman
/unwatch -s roblox -t join 123456
/list -s roblox
```

说明：

- `id` 支持 Roblox `UserId`（纯数字）或用户名。
- 用户不存在时，`watch` 会返回可读错误并拒绝订阅。

## 配置（仅 application.v2.yaml）

Roblox 仅支持 v2 配置键。`application.yaml` 中的 `roblox.*` 不会生效。

```yaml
providers:
  roblox:
    enabled: true
    interval: 10s
    timeout: 8s
    batchSize: 100
    userAgent: "Mozilla/5.0 ..."
    joinApiEnabled: true
    roblosecurity: ""
    emitLeave: true
    usernameCacheTTL: 24h
```

## 字段说明

- `enabled`: 是否启用 Roblox 轮询。
- `interval`: 轮询间隔。
- `timeout`: 单次 API 请求超时。
- `batchSize`: 每批 Presence 查询数量，最大 `100`。
- `joinApiEnabled`: 是否启用 `gamejoin` 直连接口。
- `roblosecurity`: Roblox Cookie（`.ROBLOSECURITY`），为空时自动降级 deep link。
- `emitLeave`: 是否推送离服事件。
- `usernameCacheTTL`: 用户名到 UserId 的缓存时长。

## 安全说明

- 不要在日志或截图中泄露 `.ROBLOSECURITY`。
- DDBOT 不会打印 Cookie 明文，只会输出是否配置和长度掩码。

## 兼容说明

- `providers.roblox.*` 为 v2-only 键，已在兼容加载器白名单放行。
- 不会触发 `UNKNOWN_V2_CONFIG` 告警。
