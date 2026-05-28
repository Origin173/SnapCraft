# SnapCraft

[![GitHub](https://img.shields.io/badge/GitHub-Origin173%2FSnapCraft-blue)](https://github.com/Origin173/SnapCraft)

Minecraft 服务器联动备份工具：通过 RCON 安全暂停存档，内置 [rclone](https://rclone.org/) 备份到 WebDAV、S3、Google Drive 等云盘，并支持快照列表与回档。

**仓库地址：** https://github.com/Origin173/SnapCraft

## 功能

- 服务器联动：`save-off` → `save-all flush` → 备份 → `save-on`
- 内置 rclone 驱动所有云盘传输，支持 crypt 加密 remote
- 本地优先仓库 + 可选 WebDAV/rclone 上传
- SQLite + hash 对象池 + CDC 增量备份（参考 PrimeBackup）
- 默认本地恢复，`--remote` 从远端拉取
- 单人存档离线模式（`control.type: none`）
- 归档、目录、增量三种备份模式
- 快照列表、指定时间点恢复
- Cron 定时备份、保留策略、Webhook/邮件通知

## 快速开始

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml

go build -o snapcraft ./cmd/snapcraft

./snapcraft config validate --config config.yaml
./snapcraft backup run --config config.yaml
```

## 文档

- [使用说明](docs/usage.md) — 安装、配置、命令、备份/恢复流程
- [增量存储设计](docs/incremental-storage.md) — SQLite、对象池、CDC、校验与清理
- [config.example.yaml](config.example.yaml) — 完整配置示例

## 项目结构

```text
config.example.yaml   # 配置示例（复制为 config.yaml 使用）
docs/
  usage.md            # 使用说明
cmd/snapcraft/        # CLI 入口
internal/             # 核心业务逻辑
```

## License

See [LICENSE](LICENSE).
