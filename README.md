# Smart-Git

Smart-Git 是一个高性能的 Git Smart HTTP 转发与缓存服务，旨在加速对 GitHub 等上游仓库的访问，并提供本地镜像缓存。

本项目提供两种语言实现，它们在功能和 API 上保持高度一致，可以根据部署环境灵活选择。

## 项目特点

### Go 版本
- **高性能**: 基于 [Touka](https://github.com/infinite-iroha/touka) 框架构建，具备优秀的吞吐能力与扩展性。
- **纯 Go 实现**: 使用 [Go-Git](https://github.com/go-git/go-git) 处理 Git 协议，无 CGO 依赖。
- **轻量存储**: 使用 [BoltDB](https://go.etcd.io/bbolt) 管理元数据，单文件数据库，部署简便。

### Rust 版本 (`smart-git-rs`)
- **现代异步**: 基于 [Axum](https://github.com/tokio-rs/axum) 和 [Tokio](https://github.com/tokio-rs/tokio) 栈，资源占用低且并发性强。
- **稳健的 Git 引擎**: 采用 [Gix](https://github.com/Byron/gitoxide) (Gitoxide) 引擎，提供更快的克隆与同步速度。
- **自动刷新**: 引入后台扫描任务，根据 TTL 自动更新过期仓库。
- **标准存储**: 使用 SQLite 管理元数据，方便进行数据维护。

## 部署

### Docker Compose 部署

你可以直接使用 [docker-compose.yml](https://github.com/WJQSERVER-STUDIO/smart-git/blob/main/docker/compose/docker-compose.yml) 进行快速部署。

## 配置文件

Smart-Git 推荐使用 **WANF** 配置格式，同时也兼容 **TOML**。程序启动时会优先寻找 `.wanf` 配置文件。

### Go 版本配置示例 (`config.wanf`)
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

### Rust 版本配置示例 (`config.wanf`)
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

## API 兼容性

两套实现在管理接口上保持互换性，均支持 **JSON** 和 **WANF** 响应格式。

- `GET /healthz`: 服务健康检查。
- `GET /api/db/data`: 返回当前所有缓存仓库的详细记录。
- `GET /api/db/sum`: 返回仓库的拉取统计信息（克隆次数、请求次数）。
- `POST /api/cache/{owner}/{repo}/sync`: (仅 Rust 版) 手动触发指定仓库的同步。

## 许可

本项目使用 **WJQserver Studio 开源许可证 v2.0**。

## 参考与致谢

- [Touka](https://github.com/infinite-iroha/touka)
- [Go-Git](https://github.com/go-git/go-git)
- [Gix (Gitoxide)](https://github.com/Byron/gitoxide)
- [WANF](https://github.com/WJQSERVER/wanf)
- [erred/gitreposerver](https://github.com/erred/gitreposerver) (Smart HTTP 实现参考)
