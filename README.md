# DDBOT Nexus

[![CI](https://github.com/Yar1991-Translation/DDBOT-Nexus/actions/workflows/ci.yml/badge.svg)](https://github.com/Yar1991-Translation/DDBOT-Nexus/actions/workflows/ci.yml)
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/Yar1991-Translation/DDBOT-Nexus)

`DDBOT Nexus` 是一个面向 QQ 群的订阅推送 Bot。

它的定位很明确：

- 负责订阅与推送
- 不做闲聊型机器人
- 强调可维护、可迁移、可观测

说明：

- 项目对外名称已调整为 `DDBOT Nexus`
- Go module 仍沿用旧路径以保证兼容（后续可迁移）
- 可执行文件名仍兼容旧习惯（默认 `DDBOT.exe` / `DDBOT`）

## 目录

1. [功能概览](#功能概览)
2. [快速开始](#快速开始)
3. [配置体系](#配置体系)
4. [命令用法](#命令用法)
5. [Roblox 监视](#roblox-监视)
6. [群聊降噪 Group UX](#群聊降噪-group-ux)
7. [构建与发布](#构建与发布)
8. [常见问题排查](#常见问题排查)
9. [文档索引](#文档索引)

## 功能概览

## 支持的订阅源

- Bilibili（直播、动态）
- Douyin（直播）
- Douyu（直播）
- Huya（直播）
- AcFun（直播）
- Weibo
- Twitter（Nitter）
- YouTube
- Roblox（入服、离服）

## 已实现的关键能力

- 订阅管理：`watch / unwatch / list`
- 群内权限管理：按角色、按命令控制
- 配置兼容：`application.yaml` + `application.v2.yaml` 双文件回退
- 配置迁移：`migrate-config` 命令自动生成 v2 配置
- 群聊降噪：去重、聚合、@全体冷却、安静时段摘要
- 配置热更新：监听配置变更并自动重载

## 运行框架

- LLOneBot
- NapCat
- Lagrange

默认常见 ws 地址：

- `ws://127.0.0.1:15630/ws`

## 快速开始

## Windows

1. 从 Release 下载 `windows-amd64` 压缩包并解压。
2. 运行 `DDBOT.exe`。
3. 首次登录后确认配置文件已生成。
4. 在 OneBot 侧把 ws 插件连接到 DDBOT Nexus。
5. 私聊 Bot 设置管理员：

```text
/whosyourdaddy
```

## Linux

```bash
chmod +x ./DDBOT
./DDBOT
```

## 配置体系

## 双文件兼容与优先级

支持两个配置文件：

- 旧：`application.yaml`
- 新：`application.v2.yaml`

加载顺序：

1. 默认值
2. `application.yaml`
3. `application.v2.yaml`

冲突规则：

- 同一逻辑字段以 v2 为准

## 迁移命令

```bash
DDBOT.exe migrate-config --in application.yaml --out application.v2.yaml
```

命令输出：

- 新配置文件
- 映射报告（字段映射、未知旧字段、注入默认值）

## 启动告警

若命中旧字段，会打印迁移提示（不阻断启动）：

```text
[DEPRECATED_CONFIG] old_key -> new_key (value_source=file:line)
```

## 最小可用 v2 示例

```yaml
bot:
  commandPrefix: /
  onJoinGroup:
    rename: "【bot】"

transport:
  websocket:
    mode: ws-server
    wsServer: 0.0.0.0:15630
    wsReverse: ws://localhost:3001

logging:
  level: info

notify:
  concern:
    emitInterval: 5s
  dispatch:
    largeNotifyLimit: 50
  parallel: 1

runtime:
  reloadDelay:
    enable: true
    time: 3s
```

## 命令用法

## 订阅管理

```text
/watch -s <site> -t <type> <id>
/unwatch -s <site> -t <type> <id>
/list
```

私聊代群操作：

```text
/watch -g <群号> -s <site> -t <type> <id>
/unwatch -g <群号> -s <site> -t <type> <id>
/list -g <群号>
```

## 常用管理命令

```text
/help
/config ...
/enable <command>
/disable <command>
/grant ...
/silence
```

完整命令说明：`EXAMPLE.md`

## Roblox 监视

## 订阅命令

```text
/watch -s roblox -t join 4248353366
/watch -s roblox -t join builderman
/unwatch -s roblox -t join 4248353366
/list -s roblox
```

`id` 支持：

- `UserId`（纯数字）
- 用户名（自动解析）

## 配置（仅 v2）

`providers.roblox.*` 只在 `application.v2.yaml` 生效。

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

字段说明：

- `enabled`：是否启用 Roblox 监视
- `interval`：轮询间隔（建议 `10s`~`30s`）
- `timeout`：单次请求超时
- `batchSize`：单批查询用户数（上限 100）
- `joinApiEnabled`：是否启用 gamejoin API
- `roblosecurity`：`.ROBLOSECURITY` 值（为空会自动降级 deep link）
- `emitLeave`：是否推送离服事件
- `usernameCacheTTL`：用户名解析缓存时长

推送内容（入服）：

- 用户名/显示名
- 游戏名称
- 游戏缩略图
- 服务器实例 ID
- 可加入链接

文档：

- `docs/zh-CN/roblox-monitor.md`
- `docs/en/roblox-monitor.md`

## 群聊降噪 Group UX

这部分用来控制“某个群太吵”的问题，按群独立生效。

## 命令列表

```text
/config chat show
/config chat aggregate on|off
/config chat dedupe_ttl <duration>
/config chat aggregate_window <duration>
/config chat at_all_cooldown <duration>
/config chat quiet_hours on|off --start HH:MM --end HH:MM --summary on|off
/config chat reset
```

私聊代群操作：

```text
/config -g <群号> chat show
```

## 推荐降噪参数

中等强度（大多数群）：

```text
/config chat dedupe_ttl 2m
/config chat aggregate on
/config chat aggregate_window 45s
/config chat at_all_cooldown 30m
```

强降噪（高频群）：

```text
/config chat dedupe_ttl 5m
/config chat aggregate on
/config chat aggregate_window 90s
/config chat at_all_cooldown 2h
```

夜间安静时段：

```text
/config chat quiet_hours on --start 23:00 --end 07:00 --summary on
```

## 构建与发布

## 本地构建

要求：Go `1.24.5`（见 `go.mod`）

```bash
make build
```

没有 `make` 时：

```bash
go build -o DDBOT ./cmd
```

Windows 推荐输出名：

- `DDBOT.exe`

## 版本信息

```bash
DDBOT.exe -v
```

会输出：

- `Tags`
- `COMMIT_ID`
- `BUILD_TIME`

## 常见问题排查

## 1) 订阅了但没有推送

先看这几项：

1. `watch` 是否成功
2. `list` 是否有记录
3. 站点配置是否启用
4. 轮询日志是否有报错（超时、限流、认证）
5. 群是否触发了静默/权限限制

## 2) Roblox 有入服状态但没有可加入链接

常见原因：

- `roblosecurity` 为空
- `joinApiEnabled=false`
- gamejoin 接口临时失败（会自动回退 deep link）

## 3) 配置改了不生效

检查：

1. 是否写到 `application.v2.yaml`（特别是 `providers.roblox.*`）
2. 键名缩进是否正确
3. 启动日志是否出现 `DEPRECATED_CONFIG` / `UNKNOWN_V2_CONFIG`
4. 是否被另一个同名键在 v2 覆盖

## 4) 群里回复太频繁

优先用 Group UX：

```text
/config chat dedupe_ttl 5m
/config chat aggregate on
/config chat aggregate_window 90s
```

## 文档索引

- 部署：`INSTALL.md`
- 命令示例：`EXAMPLE.md`
- 模板系统：`TEMPLATE.md`
- 更新日志：`UPDATE.md`
- 配置兼容：`docs/zh-CN/config-compat.md`
- Roblox 监视：`docs/zh-CN/roblox-monitor.md`

## 反馈

- Issues：<https://github.com/Yar1991-Translation/DDBOT-Nexus/issues>

## 许可证

AGPL-3.0，见 `LICENSE`。
