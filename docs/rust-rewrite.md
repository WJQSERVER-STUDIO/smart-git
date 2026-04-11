# Rust 重写说明

## 范围

这个仓库当前做的事情很明确：

- 把 GitHub 仓库缓存成可复用的本地 bare mirror
- 对外提供 Git Smart HTTP 服务
- 暴露缓存元数据与统计信息

`smart-git-rs/` 目录下的 Rust 实现，保持了与现有产品边界一致的目标：

- 容器部署优先
- GitHub 作为默认上游
- 使用 crates.io `gitserver-http` / `gitserver-core` 处理 Smart HTTP 协议
- 使用 `gix` 进行 clone / fetch / repo 状态检查
- 使用 SQLite 存储缓存元数据和统计信息

## 已加入的核心内容

- `smart-git-rs/Cargo.toml`
  - 使用 crates.io `gitserver-http` / `gitserver-core`
  - `gix` 负责 mirror 同步
  - `rusqlite`（`bundled`）负责 SQLite
  - `axum` + `tokio` 提供 HTTP 服务层
- `smart-git-rs/src/config.rs`
  - 默认路径对齐容器部署
  - 本地路径统一使用 `PathBuf`
  - 支持 WANF/TOML 双格式配置，WANF 优先
- `smart-git-rs/config.wanf`
  - 推荐配置示例
- `smart-git-rs/src/repo_id.rs`
  - 校验 repo id，避免路径穿越和 Windows 保留名
- `smart-git-rs/src/git/mirror.rs`
  - 基于 `gix` 的 bare mirror clone/fetch
- `smart-git-rs/src/db.rs`
  - SQLite schema、生命周期写入与统计更新
- `smart-git-rs/src/http/admin.rs`
  - `healthz`
  - 元数据/统计接口
  - 手动 sync 接口
  - 管理 API 当前统一输出 **WANF**
- `smart-git-rs/src/http/git_http.rs`
  - `info/refs`
  - `git-upload-pack`
  - 在协议处理前执行 TTL / 生命周期同步
- `smart-git-rs/src/lifecycle.rs`
  - 仓库生命周期规则集中管理
  - 每仓库串行化
  - SQLite 更新
  - registry 注册
  - 缺记录修复
  - 启动自愈

## 架构方向

使用 `gitserver-http` / `gitserver-core` 与 `gix` 的组合，可以保持 Rust 侧协议实现和 Git 数据访问都在同一套 Rust 生态里，不必混用多套 Git 实现。

### `gix` 在这里负责的事情

- clone 公共 GitHub 仓库
- 对现有 bare mirror 执行 fetch 更新
- 检查 refs 和本地 repo 状态
- 保持 repo 数据完全在服务内维护

### `gitserver-http` 在这里负责的事情

- `GET /:user/:repo/info/refs?service=git-upload-pack`
- `POST /:user/:repo/git-upload-pack`
- packet-line / stateless RPC 行为

一个关键点是：协议层假设仓库已经在本地 registry 中可发现，而本项目是动态缓存服务，不是静态 repo 根目录服务。因此 Rust 实现当前采用的是：

1. 请求进入
2. 生命周期层执行 request-time sync
3. 若本地 repo 可修复则自愈
4. 将 repo 注册进本地 registry
5. 再把请求交给 `gitserver-http`

## 当前阶段

1. 把缓存、元数据、配置和管理 API 迁到 Rust
2. 把 mirror 管理从旧形态切到 `gix`
3. 用 registry-backed 方式承接动态 repo 解析
4. 用 TTL + 启动/请求触发自愈来管理生命周期
5. 在真正替换 Go 服务前，持续用真实 Git 客户端验证兼容性

## 为什么这种形状更适合后续演进

- 容器部署仍然是第一优先级
- 避免在 Rust 服务里混用多套 Git 实现
- bundled SQLite 避免系统 SQLite 版本不一致
- repo id 在进入文件系统前已经校验
- 全部路径使用 `PathBuf`

## 后续建议

1. 增加真实 `git clone` 集成测试
2. 继续验证 `info/refs` / `git-upload-pack` 与 Go 的兼容性
3. 把 Rust 独有的管理接口与 Go 的管理面逐步对齐
4. 决定后台 refresh loop 是否继续保留，还是缩减成 janitor/self-heal 角色

## 配置优先级

Rust 服务支持 WANF 和 TOML 两种配置格式：

- 默认查找顺序：`/data/smart-git/config/config.wanf`，然后 `/data/smart-git/config/config.toml`
- 显式传 `-c /path/to/file.wanf`：直接按 WANF 加载
- 显式传 `-c /path/to/file.toml`：直接按 TOML 加载
- 显式传 `-c /path/to/config`：先尝试 `config.wanf`，再尝试 `config.toml`
- 显式路径找不到时会直接启动失败，不会静默回退
