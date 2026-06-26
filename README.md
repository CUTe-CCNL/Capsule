# Host Agent Incus Plugin

這是一個給 `CUTe-CCNL/host-agent` 使用的 Go plugin，透過 host-agent 的 stdio JSON-RPC plugin IPC 操作本機 Incus Unix socket。

支援範圍:

- 建立 system container 與 VM
- 列出、查詢、啟動、停止、重啟、刪除 instance
- 建立、列出、還原、刪除 snapshot
- 匯出 instance backup 到本機目錄
- 從本機 backup 匯入 instance
- 建立 instance 時 attach 既有 profile/network/device 設定

目前不提供 Incus network CRUD。網路設定採用「建立 instance 時指定既有 network 或 devices」的方式。

## Build

```bash
make test
make build
make package
```

`make build` 產物會輸出到 `bin/incus-plugin`。`make package` 會產生 release bundle 到 `dist/`，例如:

```bash
dist/incus-plugin-v0.1.0-linux-amd64.tar.gz
```

其他 Make targets:

- `make run`: 建置後直接啟動 plugin binary。這會連線 Incus Unix socket，並等待 host-agent 的 stdio JSON-RPC request；一般部署時由 host-agent 啟動。
- `make clean`: 清除 `bin/` 建置產物。
- `make install`: 建置並安裝 plugin 到 host-agent plugin 目錄。
- `make uninstall`: 從 host-agent plugin 目錄移除 plugin binary 與 manifest。

## Install

1. 確認 plugin binary 可執行。
2. 將 `deploy/incus.yaml` 放到 host-agent 的 `plugins.directory`。
3. 確認執行 host-agent 的使用者可以讀寫 Incus Unix socket，通常需要 root 或 `incus-admin` 群組權限。
4. 確認 `INCUS_BACKUP_DIR` 可由 plugin 寫入。
5. 建立 VM、匯出 backup 可能超過 host-agent 預設 timeout，建議把 host-agent `plugins.request_timeout` 設為 `10m` 或更長。

安裝到 host-agent plugin 目錄範例:

```bash
make build
install -Dm755 bin/incus-plugin /opt/host-agent/plugins/incus/incus-plugin
```

也可以使用自動安裝腳本:

```bash
sudo make install
```

預設會安裝到:

- binary: `/opt/host-agent/plugins/incus/incus-plugin`
- manifest: `/opt/host-agent/plugins/incus.yaml`

若 host-agent 使用不同 plugin 目錄，可用環境變數覆寫:

```bash
sudo PLUGIN_DIR=/path/to/plugins make install
sudo PLUGIN_DIR=/path/to/plugins make uninstall
```

安裝腳本會依目標路徑更新 manifest 裡的 `command` 與 `working_dir`。刪除腳本只移除 plugin binary、manifest 與空的 plugin 目錄，不會刪除 Incus instance、snapshot 或 backup。

從 release bundle 安裝:

```bash
tar -xzf incus-plugin-v0.1.0-linux-amd64.tar.gz
sudo ./install.sh
```

## Release

推送 `v*` tag 會由 GitHub Actions 自動建立 GitHub Release，並附上 Linux `amd64` 與 `arm64` tarball:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Configuration

manifest env 可設定:

- `INCUS_SOCKET_PATH`: Incus Unix socket path。空值時使用 Incus SDK 預設，例如 `/run/incus/unix.socket` 或 `/var/lib/incus/unix.socket`。
- `INCUS_PROJECT`: Incus project，預設 `default`。
- `INCUS_BACKUP_DIR`: 匯出檔案目錄，預設 `/var/lib/host-agent/incus-backups`。
- `INCUS_OPERATION_TIMEOUT`: 每個 Incus operation 等待時間，預設 `10m`。
- `INCUS_IMAGE_SERVER`: 建立 instance 時使用的 image server，預設 `https://images.linuxcontainers.org`。
- `INCUS_IMAGE_PROTOCOL`: image protocol，預設 `simplestreams`。

## API

host-agent 會對外提供 `/plugin-api/incus/...`。

```bash
curl http://localhost:9100/plugin-api/incus/status
curl http://localhost:9100/plugin-api/incus/instances
curl http://localhost:9100/plugin-api/incus/instances/web-1
```

建立 container:

```bash
curl -X POST http://localhost:9100/plugin-api/incus/instances \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "web-1",
    "type": "container",
    "image": "ubuntu/24.04",
    "profiles": ["default"],
    "network": "incusbr0",
    "cpu": 2,
    "memory": "2GiB",
    "disk": "20GiB",
    "start": true
  }'
```

建立 VM:

```bash
curl -X POST http://localhost:9100/plugin-api/incus/instances \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "vm-1",
    "type": "virtual-machine",
    "image": "ubuntu/24.04",
    "profiles": ["default"],
    "network": "incusbr0",
    "config": {
      "limits.cpu": "4",
      "limits.memory": "8GiB",
      "security.secureboot": "false"
    },
    "devices": {
      "root": {"type": "disk", "path": "/", "pool": "default", "size": "80GiB"}
    },
    "start": true
  }'
```

Instance lifecycle:

```bash
curl -X POST http://localhost:9100/plugin-api/incus/instances/web-1/start
curl -X POST http://localhost:9100/plugin-api/incus/instances/web-1/stop -d '{"force": true}'
curl -X POST http://localhost:9100/plugin-api/incus/instances/web-1/restart
curl -X DELETE 'http://localhost:9100/plugin-api/incus/instances/web-1?force=true'
```

Snapshots:

```bash
curl http://localhost:9100/plugin-api/incus/instances/web-1/snapshots
curl -X POST http://localhost:9100/plugin-api/incus/instances/web-1/snapshots -d '{"name": "before-upgrade"}'
curl -X POST http://localhost:9100/plugin-api/incus/instances/web-1/snapshots/before-upgrade/restore
curl -X DELETE http://localhost:9100/plugin-api/incus/instances/web-1/snapshots/before-upgrade
```

Exports:

```bash
curl -X POST http://localhost:9100/plugin-api/incus/instances/web-1/exports \
  -H 'Content-Type: application/json' \
  -d '{"file_name": "web-1.tar.gz", "instance_only": false, "optimized": false, "compression": "gzip"}'

curl http://localhost:9100/plugin-api/incus/exports
curl -X POST http://localhost:9100/plugin-api/incus/exports/web-1.tar.gz/import -d '{"name": "web-1-restored"}'
curl -X DELETE http://localhost:9100/plugin-api/incus/exports/web-1.tar.gz
```
