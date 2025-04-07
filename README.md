# Smart-Git

基于HertZ和Go-Git实现的git clone (smart http) 转发

## 特点

- 基于HertZ netpoll 构建, 高性能可扩展
- 基于Go-Git实现git相关功能
- 使用轻量级数据库BoltDB实现相关元数据管理

## 部署

Docker Compose 安装 [docker-compose.yml](https://github.com/WJQSERVER-STUDIO/smart-git/blob/25w02a/docker/compose/docker-compose.yml)

## 配置文件

```toml
[server]
host = "0.0.0.0" # 监听地址
port = 8080  # 监听端口
baseDir = "/data/smart-git/repos" # 缓存文件夹
memLimit = 0 #MB 内存使用限制

[log]
logfilepath = "/data/smart-git/log/smart-git.log"  # 日志存储位置
maxlogsize = 5 # MB
level = "info" # dump, debug, info, warn, error, none

[Database]
path = "/data/smart-git/db/smart-git.db" # 数据库存储位置

[cache]
expire = "1h" # 缓存过期时间
expireEx = "10m" # 过期延长时间(当hash检查后发现未过期, 增加的时间)
```

## 许可

本项目使用 WJQserver Studio 开源许可证 v2.0

## 调用

使用以下框架/实现
[HertZ](https://github.com/cloudwego/hertz)
[Go-Git](https://github.com/go-git/go-git)
[BboltDB](https://go.etcd.io/bbolt)
[toml](https://github.com/BurntSushi/toml)
[logger](github.com/WJQSERVER-STUDIO/go-utils/logger)

参考以下仓库实现Git Smart Http
[erred/gitreposerver](https://github.com/erred/gitreposerver)