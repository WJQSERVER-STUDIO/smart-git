# 安装指南

Smart-Git 提供多种部署方式，推荐使用 Docker 部署。

---

## 1. 使用 Docker Compose 部署 (推荐)

这是最便捷的部署方式。

### Go 版本
1. 下载 `docker/compose/docker-compose.yml`：
   ```bash
   wget https://raw.githubusercontent.com/WJQSERVER-STUDIO/smart-git/main/docker/compose/docker-compose.yml
   ```

2. 启动服务：
   ```bash
   docker compose up -d
   ```

### Rust 版本 (`smart-git-rs`)
1. 下载 `docker/compose/docker-compose-rs.yml`：
   ```bash
   wget https://raw.githubusercontent.com/WJQSERVER-STUDIO/smart-git/main/docker/compose/docker-compose-rs.yml
   ```

2. 启动服务：
   ```bash
   docker compose -f docker-compose-rs.yml up -d
   ```

---

## 2. 源码编译安装

### Go 版本
环境要求：Go 1.26+

1. 克隆并进入项目：
   ```bash
   git clone https://github.com/WJQSERVER-STUDIO/smart-git.git
   cd smart-git
   ```

2. 编译可执行文件：
   ```bash
   go build -o smart-git .
   ```

3. 使用指定配置运行：
   ```bash
   ./smart-git -c config/config.wanf
   ```

### Rust 版本 (`smart-git-rs`)
环境要求：Rust 1.94+

1. 进入 Rust 目录：
   ```bash
   cd smart-git-rs
   ```

2. 编译 release 版本：
   ```bash
   cargo build --release
   ```

3. 使用指定配置运行：
   ```bash
   ./target/release/smart-git-rs -c config.wanf
   ```

---

## 3. 运行环境说明

- **磁盘空间**: 取决于缓存仓库的总量。
- **网络访问**: 需确保服务器能正常连接 GitHub (或配置的上游地址)。
- **持久化**: 建议将仓库根目录及数据库目录挂载至持久化卷。

---

## 4. 注意事项

- **数据库路径**: 运行前请确保配置文件中的数据库父目录存在，或者具有创建目录的权限。
- **WANF 优先**: 两个版本均支持 `-c` 参数指定配置文件路径。若未指定，系统将尝试加载默认路径下的 `.wanf` 或 `.toml` 文件。
