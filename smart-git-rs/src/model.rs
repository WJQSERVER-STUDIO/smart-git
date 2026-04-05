use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub struct RepoCacheRecord {
    pub owner: String,
    pub name: String,
    pub upstream_url: String,
    pub local_path: String,
    pub head_oid: Option<String>,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Serialize)]
pub struct RepoStatsRecord {
    pub owner: String,
    pub name: String,
    pub clone_count: i64,
    pub request_count: i64,
}

#[derive(Debug, Serialize)]
pub struct SyncResponse {
    pub owner: String,
    pub name: String,
    pub upstream_url: String,
    pub local_path: String,
    pub head_oid: Option<String>,
    pub fresh_clone: bool,
    pub refreshed: bool,
}

#[derive(Debug, Serialize)]
pub struct HealthResponse {
    pub status: &'static str,
    pub repo_dir: String,
    pub database_path: String,
    pub github_base: String,
}
