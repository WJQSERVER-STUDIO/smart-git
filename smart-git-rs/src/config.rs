use std::{
    fs,
    path::{Path, PathBuf},
    time::Duration,
};

use anyhow::Context;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(default)]
pub struct Config {
    pub server: ServerConfig,
    pub database: DatabaseConfig,
    pub cache: CacheConfig,
    pub upstream: UpstreamConfig,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(default)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(default)]
pub struct DatabaseConfig {
    pub path: PathBuf,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(default)]
pub struct CacheConfig {
    pub repo_dir: PathBuf,
    pub refresh_ttl_secs: u64,
    pub refresh_scan_secs: u64,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(default)]
pub struct UpstreamConfig {
    pub github_base: String,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            server: ServerConfig::default(),
            database: DatabaseConfig::default(),
            cache: CacheConfig::default(),
            upstream: UpstreamConfig::default(),
        }
    }
}

impl Default for ServerConfig {
    fn default() -> Self {
        Self {
            host: "0.0.0.0".to_owned(),
            port: 8080,
        }
    }
}

impl Default for DatabaseConfig {
    fn default() -> Self {
        Self {
            path: PathBuf::from("/data/smart-git/db/smart-git.db"),
        }
    }
}

impl Default for CacheConfig {
    fn default() -> Self {
        Self {
            repo_dir: PathBuf::from("/data/smart-git/repos"),
            refresh_ttl_secs: 300,
            refresh_scan_secs: 60,
        }
    }
}

impl Default for UpstreamConfig {
    fn default() -> Self {
        Self {
            github_base: "https://github.com".to_owned(),
        }
    }
}

impl Config {
    pub fn default_path() -> PathBuf {
        PathBuf::from("/data/smart-git/config/config.toml")
    }

    pub fn load(path: Option<&Path>) -> anyhow::Result<Self> {
        let path = path.map(PathBuf::from).unwrap_or_else(Self::default_path);
        if !path.exists() {
            return Ok(Self::default());
        }

        let raw = fs::read_to_string(&path)
            .with_context(|| format!("failed to read config file {}", path.display()))?;
        toml::from_str(&raw)
            .with_context(|| format!("failed to parse config file {}", path.display()))
    }

    pub fn ensure_dirs(&self) -> anyhow::Result<()> {
        fs::create_dir_all(&self.cache.repo_dir).with_context(|| {
            format!(
                "failed to create repository cache root {}",
                self.cache.repo_dir.display()
            )
        })?;

        if let Some(parent) = self.database.path.parent() {
            fs::create_dir_all(parent).with_context(|| {
                format!("failed to create database parent {}", parent.display())
            })?;
        }

        Ok(())
    }
}

impl ServerConfig {
    pub fn listen_addr(&self) -> String {
        format!("{}:{}", self.host, self.port)
    }
}

impl CacheConfig {
    pub fn refresh_ttl(&self) -> Duration {
        Duration::from_secs(self.refresh_ttl_secs)
    }

    pub fn refresh_scan_interval(&self) -> Duration {
        Duration::from_secs(self.refresh_scan_secs)
    }
}
