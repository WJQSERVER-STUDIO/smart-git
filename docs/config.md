# 配置指南

Smart-Git 推荐使用 **WANF** 配置格式，同时也兼容 **TOML**。系统启动时会优先查找 `.wanf` 后缀的配置文件。

---

## Go 版本配置

### WANF 格式 (`config.wanf`)
```wanf
Server {
  host = "0.0.0.0"
  port = 8080
  baseDir = "/data/smart-git/repos"
  memLimit = 0
}

Log {
  logfilepath = "/data/smart-git/log/smart-git.log"
  maxlogsize = 5
  level = "info"
}

Database {
  path = "/data/smart-git/db/smart-git.db"
}

Cache {
  expire = 1h
  expireEx = 10m
}
```

### TOML 格式 (`config.toml`)
```toml
[server]
host = "0.0.0.0"
port = 8080
baseDir = "/data/smart-git/repos"
memLimit = 0

[log]
logfilepath = "/data/smart-git/log/smart-git.log"
maxlogsize = 5
level = "info"

[database]
path = "/data/smart-git/db/smart-git.db"

[cache]
expire = "1h"
expireEx = "10m"
```

---

## Rust 版本配置 (`smart-git-rs`)

### WANF 格式 (`config.wanf`)
```wanf
server {
  host = "0.0.0.0"
  port = 8080
}

database {
  path = "/data/smart-git/db/smart-git.db"
}

cache {
  repo_dir = "/data/smart-git/repos"
  refresh_ttl_secs = 300
  refresh_scan_secs = 60
}

upstream {
  github_base = "https://github.com"
}
```

### TOML 格式 (`config.toml`)
```toml
[server]
host = "0.0.0.0"
port = 8080

[database]
path = "/data/smart-git/db/smart-git.db"

[cache]
repo_dir = "/data/smart-git/repos"
refresh_ttl_secs = 300
refresh_scan_secs = 60

[upstream]
github_base = "https://github.com"
```

---

## 配置项详解

### Server / server (服务器配置)
- **host**: 服务器监听的 IP 地址。默认为 `0.0.0.0`（监听所有网卡）。
- **port**: 服务器监听的 TCP 端口。默认为 `8080`。
- **baseDir / repo_dir**: 本地 Git 仓库缓存的根目录。程序会在此目录下按 `user/repo.git` 的结构存储 bare 仓库。
- **memLimit (仅 Go)**: 设置 Go 运行时的内存限制（单位：MB）。若大于 0，则会调用 `debug.SetMemoryLimit`。

### Log / log (日志配置 - 仅 Go 支持详细配置)
- **logfilepath**: 日志文件的存储路径。
- **maxlogsize**: 单个日志文件的最大大小（单位：MB）。
- **level**: 日志输出级别。支持 `dump`, `debug`, `info`, `warn`, `error`, `none`。

### Database / database (数据库配置)
- **path**: 数据库文件路径。Go 版本使用 BoltDB (单文件 KV)，Rust 版本使用 SQLite。

### Cache / cache (缓存策略配置)
- **expire (Go)**: 仓库缓存的有效期（如 `1h`, `30m`）。过期后的请求将触发与上游同步。
- **expireEx (Go)**: 延展时间。当检查发现上游未更新（Hash 未变）时，为缓存增加的额外有效期。
- **refresh_ttl_secs (Rust)**: 缓存有效期（单位：秒）。
- **refresh_scan_secs (Rust)**: 后台同步任务的扫描频率（单位：秒）。程序会定期扫描并刷新已过期的仓库。

### Upstream / upstream (上游配置 - 仅 Rust)
- **github_base**: 上游 Git 托管平台的基准 URL。默认为 `https://github.com`。
