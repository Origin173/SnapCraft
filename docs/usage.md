# SnapCraft 使用说明

项目仓库：[https://github.com/Origin173/SnapCraft](https://github.com/Origin173/SnapCraft)

SnapCraft 是一款 Minecraft 服务器联动备份工具，通过 RCON 或控制台安全暂停存档保存，并使用 [rclone](https://rclone.org/) 将备份上传到各类云盘。

## 环境要求

- [rclone](https://rclone.org/install/) 已安装并完成云盘 remote 配置
- Minecraft Java 版服务器开启 RCON，或提供控制台管道访问（基岩版/console 模式）
- 从源码构建时需要 Go 1.22+

## 安装

### 从源码构建

```bash
mkdir -p build/release
go build -o build/release/snapcraft ./cmd/snapcraft
```

Windows：

```powershell
New-Item -ItemType Directory -Force -Path build\release
go build -o build/release/snapcraft.exe ./cmd/snapcraft
```

### 直接安装

```bash
go install github.com/Origin173/SnapCraft/cmd/snapcraft@latest
```

## 快速开始

1. 复制示例配置到工作目录：

```bash
cp config.example.yaml config.yaml
```

2. 编辑 `config.yaml`，填写服务器路径、RCON 信息和 rclone remote。

3. 校验配置：

```bash
snapcraft config validate --config config.yaml --check-paths
```

4. 执行备份：

```bash
snapcraft backup run --config config.yaml
```

5. 查看历史快照：

```bash
snapcraft snapshots list --config config.yaml
```

6. 恢复指定快照（默认从本地仓库，建议先停止服务器）：

```bash
snapcraft restore 2026-05-28T08-30-00Z-a1b2c3 --config config.yaml --force-online
snapcraft restore 2026-05-28T08-30-00Z-a1b2c3 --config config.yaml --remote
```

## 命令参考

| 命令 | 说明 |
|------|------|
| `snapcraft config validate` | 校验配置文件 |
| `snapcraft backup run` | 立即执行一次备份 |
| `snapcraft snapshots list` | 列出本地仓库历史快照 |
| `snapcraft restore <snapshot-id>` | 从本地仓库恢复（默认） |
| `snapcraft restore <snapshot-id> --remote` | 从远端下载后恢复 |
| `snapcraft repo init` | 初始化本地备份仓库 |
| `snapcraft repo verify [snapshot-id]` | 校验本地仓库/快照完整性 |
| `snapcraft schedule run` | 启动定时备份守护进程 |
| `snapcraft prune` | 预览保留策略将删除的快照 |
| `snapcraft prune --apply` | 执行保留策略清理 |

全局参数：

- `-c, --config`：配置文件路径，默认 `config.yaml`

## 本地优先备份

SnapCraft 默认采用**本地优先**策略：

1. 先在本地仓库 `./snapcraft-repo` 完成快照并校验
2. 若 `upload.enabled: true`，再上传到 WebDAV/rclone remote
3. 上传并校验成功后，可按配置清理本地大对象

```yaml
repository:
  local_path: ./snapcraft-repo
  verify_after_backup: true
  verify_after_upload: true
  cleanup_after_verified_upload: false

upload:
  enabled: true
```

**防 TPS 影响**：`save-on` 在本地 staging 完成后立即执行，压缩/CDC/上传都在 save-on 之后进行。

详见 [增量存储设计](incremental-storage.md)。

## 单人存档备份

无需 RCON，直接备份 world 文件夹：

```yaml
server:
  name: MyWorld
  world_path: C:\Users\You\AppData\Roaming\.minecraft\saves\MyWorld
  control:
    type: none

upload:
  enabled: false  # 仅本地备份时可关闭
```

或使用 CLI 参数：

```bash
snapcraft backup run --world "C:\Users\You\AppData\Roaming\.minecraft\saves\MyWorld" --name MyWorld
```

## 增量备份模式

```yaml
backup:
  mode: incremental
  hash_method: blake3
  cdc:
    enabled: true
    min_file_size: 8388608
```

- 小文件：整文件 hash 去重存入 `objects/`
- 大文件：CDC 分块存入 `chunks/`
- SQLite 记录快照树，类似 Git 的增量回档体验

## 配置说明

完整配置示例见项目根目录 [`config.example.yaml`](../config.example.yaml)。

### 服务器联动

```yaml
server:
  name: my-minecraft-server
  world_path: /path/to/server/world
  control:
    type: rcon
    rcon:
      host: 127.0.0.1
      port: 25575
      password: your-rcon-password
```

RCON 需在 `server.properties` 中开启：

```properties
enable-rcon=true
rcon.port=25575
rcon.password=your-secure-password
```

### 备份模式

**归档模式（默认，`archive`）**

每次备份生成 tar 压缩包并上传到 `archives/`，恢复简单可靠，适合大多数场景。

**目录模式（`directory`）**

使用 rclone `sync` 配合 `--backup-dir` 维护远端镜像和变更历史，适合大存档和低带宽环境。

```yaml
backup:
  mode: directory
  compression: zstd  # archive 模式下有效：zstd | gzip | none
```

### rclone 与加密

SnapCraft 不直接对接云盘 API，所有上传/下载均通过 rclone 执行。客户端加密请配置 rclone crypt remote：

```bash
rclone config
# 创建 crypt remote，包装底层 remote
```

```yaml
rclone:
  remote: myremote:crypt
  remote_path: snapcraft/my-minecraft-server
  bwlimit: 10M
```

远端目录结构：

```text
myremote:snapcraft/my-server/
  manifests/<snapshot-id>.json
  archives/<snapshot-id>.tar.zst
  directories/current/
  directories/history/<snapshot-id>/
  logs/<snapshot-id>.log
```

### 定时备份

```yaml
schedule:
  enabled: true
  cron: "0 4 * * *"
```

启动调度器：

```bash
snapcraft schedule run --config config.yaml
```

### 保留策略

```yaml
retention:
  daily: 7    # 保留最近 7 天的每日备份
  weekly: 4   # 保留最近 4 周的每周备份
  monthly: 0  # 可选：保留最近 N 个月的每月备份
```

```bash
snapcraft prune --config config.yaml          # 预览
snapcraft prune --config config.yaml --apply  # 执行
```

### 通知

Webhook 通知（推荐）：

```yaml
notify:
  webhook:
    enabled: true
    url: https://hooks.example.com/backup
```

也可通过环境变量启用：`SNAPCRAFT_WEBHOOK_URL`。

## 环境变量

| 变量 | 覆盖配置项 |
|------|-----------|
| `SNAPCRAFT_SERVER_NAME` | `server.name` |
| `SNAPCRAFT_WORLD_PATH` | `server.world_path` |
| `SNAPCRAFT_RCON_HOST` | `server.control.rcon.host` |
| `SNAPCRAFT_RCON_PORT` | `server.control.rcon.port` |
| `SNAPCRAFT_RCON_PASSWORD` | `server.control.rcon.password` |
| `SNAPCRAFT_RCLONE_REMOTE` | `rclone.remote` |
| `SNAPCRAFT_RCLONE_PATH` | `rclone.remote_path` |
| `SNAPCRAFT_WEBHOOK_URL` | `notify.webhook.url`（同时启用 webhook） |

## 备份流程

1. `save-off` — 暂停自动保存
2. `save-all flush` — 强制刷盘
3. 复制 world 到本地 staging 目录
4. `save-on` — 恢复自动保存（尽量缩短服务器暂停时间）
5. 压缩/打包并上传到 rclone remote
6. 写入 manifest 元数据

若 `save-off` 之后的任一步骤失败，SnapCraft 会自动尝试执行 `save-on`，避免服务器长期处于不可保存状态。

## 恢复流程

1. 从远端下载快照（归档或目录）
2. 使用 `--force-online` 时，先自动创建 safety backup
3. 将当前 world 目录移走备份
4. 原子替换为恢复内容

**建议**：恢复前停止 Minecraft 服务器，仅在明确风险时使用 `--force-online`。

## 开发与测试

```bash
# 运行测试，覆盖率报告输出到 build/test/
mkdir -p build/test
go test ./... -coverprofile=build/test/coverage.out

# 构建 release 二进制到 build/release/
mkdir -p build/release
go build -o build/release/snapcraft ./cmd/snapcraft
```

## 安全说明

- 备份期间通过 lock 文件防止并发执行
- 恢复前可创建 safety backup，降低误回档风险
- 加密依赖 rclone crypt，不在应用层重复实现
- 敏感信息（RCON 密码）建议通过环境变量注入，避免提交到版本库
