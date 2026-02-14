# 配置兼容说明（application.yaml / application.v2.yaml）

## 加载顺序

DDBOT 现在支持双文件回退加载：

1. 内置默认值
2. `application.yaml`（旧配置）
3. `application.v2.yaml`（新配置，优先级最高）

冲突时始终以 v2 为准。

## 兼容行为

- 仅有 `application.yaml`：保持现有行为，兼容启动。
- 仅有 `application.v2.yaml`：按新结构运行。
- 两者同时存在：旧配置作为缺省补全，新配置覆盖冲突字段。

## 兼容告警

检测到旧字段时，启动会输出迁移告警（不阻断启动）：

`[DEPRECATED_CONFIG] old_key -> new_key (value_source=file:line)`

同一旧字段在同一次进程启动中只会告警一次，并输出汇总总数。

## 热更新

配置热更新会同时监听：

- `application.yaml`
- `application.v2.yaml`

无论修改哪个文件，都会按同一优先级规则重新合并。

## v2-only 键说明

- `providers.roblox.*` 仅支持写在 `application.v2.yaml`。
- 这些键会被兼容加载器直通加载，不会触发 `UNKNOWN_V2_CONFIG`。
- `application.yaml` 中的 `roblox.*` 不作为兼容路径。

## 迁移命令

```bash
ddbot migrate-config --in application.yaml --out application.v2.yaml
```

命令输出：

- 新配置文件
- 迁移报告（字段映射、未识别旧字段、默认值注入）

当输出文件已存在时，不会覆盖该文件，而是写入新的 `.migrated` 文件。
