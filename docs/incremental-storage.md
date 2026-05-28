# SnapCraft 增量存储设计

SnapCraft 的增量模式参考 [PrimeBackup](https://github.com/TISUnion/PrimeBackup) 思路：SQLite 管理快照元数据，文件内容进入 hash-based 对象池，大文件可选 CDC 分块去重。

## 本地仓库结构

```text
snapcraft-repo/
  snapcraft.db          # SQLite 元数据
  objects/              # 小文件/整文件对象（hash 去重）
  chunks/               # CDC 分块对象
  archives/             # archive 模式 tar 包
  manifests/            # 导出/远端兼容 manifest
  staging/              # 临时 staging
```

## SQLite 表

| 表 | 作用 |
|---|---|
| `snapshots` | 快照 ID、模式、状态、本地/远端同步状态 |
| `entries` | 每个快照的目录树条目 |
| `objects` | 整文件对象池（hash、路径、引用计数） |
| `chunks` | CDC 分块对象池 |
| `file_chunks` | 大文件到 chunk 的顺序映射 |
| `remote_sync` | 远端上传/校验状态 |

## 备份模式

### archive

1. staging 复制 world
2. 生成 tar/zst 写入 `archives/`
3. 登记 DB 并本地校验
4. 可选上传 WebDAV/rclone remote

### incremental

1. staging 复制 world
2. 扫描目录树写入 `entries`
3. 小文件：整文件 hash → `objects/`
4. 大文件（>= `cdc.min_file_size`）：CDC 分块 → `chunks/`
5. 相同 hash 只存一份（去重）
6. 本地校验后可上传缺失对象

## 备份流程（防 TPS 影响）

```text
save-off → save-all → 本地 staging → save-on → 本地 repo 写入/校验 → 可选远端上传
```

关键：**save-on 在本地 staging 完成后立即执行**，压缩、CDC、上传、远端校验都在 save-on 之后进行，WebDAV 慢或断线不会影响 Minecraft 保存状态。

## 恢复来源

```bash
snapcraft restore <id>              # 默认本地 repo
snapcraft restore <id> --remote     # 从远端拉取缺失对象后恢复
snapcraft restore <id> --source remote
```

## 上传与清理

```yaml
upload:
  enabled: true

repository:
  verify_after_upload: true
  cleanup_after_verified_upload: false  # true = 上传且校验成功后删除本地大对象
  keep_local_manifests: true
```

- 上传失败：本地快照仍可用（`completed_local`）
- 校验失败：不删除本地 payload
- 清理后：DB/manifest 保留，恢复缺对象时需 `--remote`

## 单人存档

```yaml
server:
  control:
    type: none
```

或使用 CLI 参数：

```bash
snapcraft backup run --world "C:\Users\You\AppData\Roaming\.minecraft\saves\MyWorld" --name MyWorld
```

## WebDAV 校验说明

优先使用内置 rclone 的 `check` 对比本地与远端。若 remote 不支持 checksum，退化为 size + manifest hash 校验（可靠性略低，见 `docs/usage.md`）。
